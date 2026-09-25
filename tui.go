package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ── messages ──────────────────────────────────────────────────────────────────

type agentsMsg struct {
	agents     []agentInfo
	workspaces []workspaceInfo
	err        error
}

// pollMsg re-reads the agents, so statuses stay live while the popup is open.
type pollMsg struct{}

// sessionMsg is a pane's Claude session, found for the status change seq.
type sessionMsg struct {
	pane    string
	seq     int64
	session claudeSession
	cached  *recap
	current bool
	err     error
}

type recapMsg struct {
	pane    string
	session string
	recap   recap
	err     error
}

type focusedMsg struct{ err error }

// changesMsg is git's answer about an agent's worktree, read at `at`.
type changesMsg struct {
	pane, at string
	changes  gitChanges
}

// repliedMsg is how sending a reply went.
type repliedMsg struct {
	pane, title string
	err         error
}

// pollEvery is how often the open popup re-reads herdr's agents.
const pollEvery = time.Second

// maxRecapping bounds how many recaps run at once: each is a claude process.
const maxRecapping = 6

// recapLines is the most lines a recap is given; Claude keeps them under 40
// words, which is two or three lines in a popup.
const recapLines = 3

// ── styles ────────────────────────────────────────────────────────────────────

// The defaults, for a theme that leaves a colour unset; useTheme recolours
// the styles from herdr's theme. The status colours are herdr's sidebar's.
var (
	defaultStyleDim      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "243"})
	defaultStyleHeader   = lipgloss.NewStyle().Bold(true)
	defaultStyleTabOn    = lipgloss.NewStyle().Bold(true).Underline(true)
	defaultStyleSelected = lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "254", Dark: "237"})
	defaultStyleErr      = lipgloss.NewStyle().Foreground(lipgloss.Color("#eb5757"))
	defaultStyleOK       = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	defaultStyleBlocked  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	defaultStyleWorking  = lipgloss.NewStyle().Foreground(lipgloss.Color("#c78a1f"))
	defaultStyleDone     = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	defaultStyleIdle     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	defaultStyleTitle    = lipgloss.NewStyle().Bold(true)
	defaultStyleRecap    = lipgloss.NewStyle()
	defaultStyleBranch   = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	defaultStyleModel    = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	defaultStyleContext  = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	defaultStyleMode     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	defaultStyleToken    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
)

var (
	styleDim      = defaultStyleDim
	styleHeader   = defaultStyleHeader
	styleTabOn    = defaultStyleTabOn
	styleSelected = defaultStyleSelected
	styleErr      = defaultStyleErr
	styleOK       = defaultStyleOK
	styleBlocked  = defaultStyleBlocked
	styleWorking  = defaultStyleWorking
	styleDone     = defaultStyleDone
	styleIdle     = defaultStyleIdle
	styleTitle    = defaultStyleTitle
	styleRecap    = defaultStyleRecap
	styleBranch   = defaultStyleBranch
	styleModel    = defaultStyleModel
	styleContext  = defaultStyleContext
	styleMode     = defaultStyleMode
	styleToken    = defaultStyleToken
)

// statusRank orders the list: what needs you first, then what finished, then
// what's still going, then the rest.
func statusRank(status string) int {
	switch status {
	case "blocked":
		return 0
	case "done":
		return 1
	case "working":
		return 2
	case "idle":
		return 3
	}
	return 4
}

// ── model ─────────────────────────────────────────────────────────────────────

// entry is one agent's row.
type entry struct {
	agent       agentInfo
	session     *claudeSession
	recap       *recap
	current     bool   // the recap describes the conversation as it is now
	resolvedSeq int64  // the status change the session was last looked up at
	resolving   bool   // looking up the session
	recapping   bool   // a recap is being written
	triedRecap  bool   // one has been asked for since the popup opened
	note        string // why there's no recap: not Claude, nothing said yet, an error
	branch      string // the git branch in the agent's folder
	branchAt    string // the folder and status change branch was read at
	changes     gitChanges
}

type model struct {
	ctx        context.Context
	src        source
	tokens     []string // config.json's tokens: which to show, in order
	width      int
	height     int
	entries    map[string]*entry
	order      []string // pane IDs, as listed
	workspaces []workspaceInfo
	cursor     int
	offset     int // first entry shown
	loaded     bool
	spin       spinner.Model
	err        string
	flash      string
	sem        chan struct{} // bounds the recaps running at once
	now        func() time.Time

	mouseX, mouseY int // last pointer position; -1 until the mouse moves

	// A reply being typed to the selected agent (p), sent with agent.prompt.
	replying  bool
	replyTo   string // its pane
	replyText textinput.Model
}

func newModel(ctx context.Context, src source) model {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = lipgloss.NewStyle()
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "yes, go ahead"
	ti.CharLimit = 2000
	return model{
		ctx: ctx, src: src, entries: map[string]*entry{}, spin: sp,
		sem: make(chan struct{}, maxRecapping), now: time.Now, mouseX: -1, mouseY: -1, replyText: ti,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.loadAgents())
}

func (m model) loadAgents() tea.Cmd {
	src := m.src
	return func() tea.Msg {
		agents, workspaces, err := src.agents()
		return agentsMsg{agents, workspaces, err}
	}
}

func (m model) resolve(a agentInfo) tea.Cmd {
	src := m.src
	return func() tea.Msg {
		s, err := src.session(a.PaneID)
		msg := sessionMsg{pane: a.PaneID, seq: a.StateChangeSeq, session: s, err: err}
		if err == nil {
			msg.cached, msg.current = src.cached(s)
		}
		return msg
	}
}

func (m model) recapCmd(pane string, s claudeSession, force bool) tea.Cmd {
	src, ctx, sem := m.src, m.ctx, m.sem
	return func() tea.Msg {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return recapMsg{pane: pane, session: s.ID, err: ctx.Err()}
		}
		defer func() { <-sem }()
		r, err := src.recap(ctx, s, force)
		return recapMsg{pane: pane, session: s.ID, recap: r, err: err}
	}
}

// ── update ────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scrollTo()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case pollMsg:
		return m, m.loadAgents()

	case agentsMsg:
		next := tea.Tick(pollEvery, func(time.Time) tea.Msg { return pollMsg{} })
		if msg.err != nil {
			m.err = "Couldn't list herdr's agents: " + msg.err.Error()
			return m, next
		}
		if strings.HasPrefix(m.err, "Couldn't list herdr's agents") {
			m.err = ""
		}
		return m, tea.Batch(next, m.updateAgents(msg.agents, msg.workspaces))

	case sessionMsg:
		e := m.entries[msg.pane]
		if e == nil {
			return m, nil
		}
		e.resolving = false
		if msg.err != nil {
			e.session, e.note = nil, sessionNote(msg.err)
			if msg.seq != e.agent.StateChangeSeq {
				// It changed while being looked up: look again.
				return m, m.startResolve(e)
			}
			return m, nil
		}
		if e.session == nil || e.session.ID != msg.session.ID {
			e.recap = nil // a different conversation now: the old recap isn't its
		}
		s := msg.session
		e.session, e.note = &s, ""
		if msg.cached != nil {
			e.recap = msg.cached
		}
		e.current = msg.current
		if s.Transcript == "" {
			e.note = "Nothing to recap yet."
			return m, nil
		}
		// An out-of-date recap is rewritten, but only once for an agent
		// that's working: it'd be out of date again straight away.
		if !e.current && !e.recapping && (e.agent.Status != "working" || !e.triedRecap) {
			return m, m.startRecap(e, false)
		}
		return m, nil

	case recapMsg:
		e := m.entries[msg.pane]
		if e == nil {
			return m, nil
		}
		e.recapping = false
		if e.session == nil || e.session.ID != msg.session {
			return m, nil
		}
		switch {
		case errors.Is(msg.err, errNothingYet):
			e.note = "Nothing to recap yet."
		case errors.Is(msg.err, context.Canceled):
		case msg.err != nil:
			e.note = "Couldn't recap: " + msg.err.Error()
		default:
			r := msg.recap
			e.recap, e.current, e.note = &r, true, ""
		}
		return m, nil

	case changesMsg:
		if e := m.entries[msg.pane]; e != nil && e.branchAt == msg.at {
			e.changes = msg.changes
		}
		return m, nil

	case repliedMsg:
		switch {
		case isHerdrCode(msg.err, "agent_blocked"):
			m.err = shorten(msg.title, 40) + " is waiting on a question or approval: go to it (enter) to answer."
		case msg.err != nil:
			m.err = "Couldn't send the reply: " + msg.err.Error()
		default:
			m.flash = "Sent to " + shorten(msg.title, 60) + "."
		}
		return m, nil

	case focusedMsg:
		if errors.Is(msg.err, errDemo) {
			m.flash = "In the demo that's as far as it goes: the agents are made up."
			return m, nil
		}
		if msg.err != nil {
			m.err = "Couldn't go to that agent: " + msg.err.Error()
			return m, nil
		}
		return m, tea.Quit

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return m, nil
}

// updateAgents takes a fresh agent list: new agents get a row, gone ones lose
// theirs, and a Claude agent whose status changed has its session looked up
// again (it may be a new conversation, and its recap out of date).
func (m *model) updateAgents(agents []agentInfo, workspaces []workspaceInfo) tea.Cmd {
	selected := m.selectedPane()
	m.loaded, m.workspaces = true, workspaces
	var cmds []tea.Cmd
	seen := map[string]bool{}
	for _, a := range agents {
		seen[a.PaneID] = true
		e := m.entries[a.PaneID]
		if e == nil {
			e = &entry{resolvedSeq: -1}
			m.entries[a.PaneID] = e
		}
		kindChanged := e.agent.Agent != a.Agent
		e.agent = a
		// The branch can change with the folder or on any turn: read it again
		// then (a few small files), not every second.
		if at := a.Cwd + "\x00" + strconv.FormatInt(a.StateChangeSeq, 10); a.Cwd != "" && e.branchAt != at {
			e.branch, e.branchAt = gitBranch(a.Cwd), at
			src, pane, cwd := m.src, a.PaneID, a.Cwd
			cmds = append(cmds, func() tea.Msg { return changesMsg{pane, at, src.changes(cwd)} })
		}
		if a.Agent != "claude" {
			e.session, e.recap, e.note = nil, nil, "Recaps are for Claude agents."
			continue
		}
		if kindChanged && e.note == "Recaps are for Claude agents." {
			e.note = ""
		}
		if !e.resolving && e.resolvedSeq != a.StateChangeSeq {
			cmds = append(cmds, m.startResolve(e))
		}
	}
	for pane := range m.entries {
		if !seen[pane] {
			delete(m.entries, pane)
		}
	}
	m.sortEntries()
	m.cursor = 0
	for i, p := range m.order {
		if p == selected {
			m.cursor = i
		}
	}
	m.scrollTo()
	return tea.Batch(cmds...)
}

func (m *model) startResolve(e *entry) tea.Cmd {
	e.resolving, e.resolvedSeq = true, e.agent.StateChangeSeq
	return m.resolve(e.agent)
}

func (m *model) startRecap(e *entry, force bool) tea.Cmd {
	e.recapping, e.triedRecap = true, true
	if e.note != "" && strings.HasPrefix(e.note, "Couldn't recap") {
		e.note = ""
	}
	return m.recapCmd(e.agent.PaneID, *e.session, force)
}

// sessionNote says why an agent's session couldn't be found.
func sessionNote(err error) string {
	if errors.Is(err, errNotClaude) {
		return "Claude isn't running in this pane any more."
	}
	return "Couldn't find its conversation: " + err.Error()
}

func (m *model) sortEntries() {
	wsIndex := map[string]int{}
	for i, w := range m.workspaces {
		wsIndex[w.WorkspaceID] = i
	}
	m.order = m.order[:0]
	for p := range m.entries {
		m.order = append(m.order, p)
	}
	sort.Slice(m.order, func(i, j int) bool {
		a, b := m.entries[m.order[i]].agent, m.entries[m.order[j]].agent
		if ra, rb := statusRank(a.Status), statusRank(b.Status); ra != rb {
			return ra < rb
		}
		// Within a status, what has waited longest first.
		sa, sb := m.inStateSince(m.entries[m.order[i]]), m.inStateSince(m.entries[m.order[j]])
		if !sa.Equal(sb) && !sa.IsZero() && !sb.IsZero() {
			return sa.Before(sb)
		}
		if sa.IsZero() != sb.IsZero() {
			return !sa.IsZero()
		}
		wa, oka := wsIndex[a.WorkspaceID]
		wb, okb := wsIndex[b.WorkspaceID]
		if oka != okb {
			return oka
		}
		if wa != wb {
			return wa < wb
		}
		return paneLess(a.PaneID, b.PaneID)
	})
}

// paneLess orders pane IDs by their numbers, so w1:p2 comes before w1:p10.
func paneLess(a, b string) bool {
	na, nb := paneNumber(a), paneNumber(b)
	if na != nb {
		return na < nb
	}
	return a < b
}

func paneNumber(id string) int {
	_, p, _ := strings.Cut(id, ":p")
	n, err := strconv.Atoi(p)
	if err != nil {
		return 1 << 30
	}
	return n
}

func (m model) selectedPane() string {
	if m.cursor >= 0 && m.cursor < len(m.order) {
		return m.order[m.cursor]
	}
	return ""
}

func (m model) selected() *entry {
	return m.entries[m.selectedPane()]
}

func (m model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.replying {
		return m.handleReplyKey(k)
	}
	m.flash = ""
	switch k.String() {
	case "p":
		return m.startReply()
	case "ctrl+c", "esc", "q":
		return m, tea.Quit
	case "up", "k", "ctrl+p":
		m.move(-1)
	case "down", "j", "ctrl+n":
		m.move(1)
	case "pgup":
		m.move(-max(1, m.visibleEntries()))
	case "pgdown":
		m.move(max(1, m.visibleEntries()))
	case "home", "g":
		m.move(-len(m.order))
	case "end", "G":
		m.move(len(m.order))
	case "enter":
		return m.activate()
	case "ctrl+r", "r":
		return m.recapAgain()
	}
	return m, nil
}

// startReply opens the reply line for the selected agent.
func (m model) startReply() (tea.Model, tea.Cmd) {
	e := m.selected()
	if e == nil {
		return m, nil
	}
	m.replying, m.replyTo, m.err = true, e.agent.PaneID, ""
	m.replyText.SetValue("")
	return m, m.replyText.Focus()
}

// handleReplyKey is typing a reply: enter sends it, esc drops it.
func (m model) handleReplyKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.replying = false
		m.replyText.Blur()
		return m, nil
	case "enter":
		text := strings.TrimSpace(m.replyText.Value())
		m.replying = false
		m.replyText.Blur()
		e := m.entries[m.replyTo]
		if text == "" || e == nil {
			return m, nil
		}
		src, pane, name := m.src, e.agent.PaneID, title(e.agent)
		return m, func() tea.Msg { return repliedMsg{pane, name, src.prompt(pane, text)} }
	}
	var cmd tea.Cmd
	m.replyText, cmd = m.replyText.Update(k)
	return m, cmd
}

// activate goes to the selected agent, closing the popup once herdr has.
func (m model) activate() (tea.Model, tea.Cmd) {
	e := m.selected()
	if e == nil {
		return m, nil
	}
	src, pane := m.src, e.agent.PaneID
	return m, func() tea.Msg { return focusedMsg{src.focus(pane)} }
}

// recapAgain writes the selected agent's recap anew, current or not.
func (m model) recapAgain() (tea.Model, tea.Cmd) {
	e := m.selected()
	switch {
	case e == nil:
		return m, nil
	case e.session == nil || e.session.Transcript == "":
		if e.note != "" {
			m.flash = e.note
		}
		return m, nil
	case e.recapping:
		return m, nil
	}
	return m, m.startRecap(e, true)
}

func (m *model) move(delta int) {
	if len(m.order) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), len(m.order)-1)
	m.scrollTo()
}

// ── layout ────────────────────────────────────────────────────────────────────

// listTop is the first list line: under the header and a blank.
const listTop = 2

func (m model) listHeight() int {
	// header + blank above; status line + footer below, the footer on the
	// popup's last line (mouse.go counts on that).
	return max(3, m.height-listTop-2)
}

// recapWidth is the room a recap line has: indented under the title.
func (m model) recapWidth() int { return max(10, m.width-6) }

// body is what goes under an agent's title: its recap wrapped, or why there
// isn't one.
func (m model) body(e *entry) (lines []string, dim bool) {
	w := m.recapWidth()
	switch {
	case e.recap != nil && e.recap.Text != "":
		return wrap(e.recap.Text, w, recapLines), !e.current
	case e.recapping:
		return []string{"Recapping…"}, true
	case e.resolving && e.note == "":
		return []string{"…"}, true
	case e.note != "":
		return wrap(e.note, w, 2), true
	}
	return []string{""}, true
}

// entryHeight is an entry's lines on screen, as viewEntry draws them. The
// selected entry has one more: your last prompt.
func (m model) entryHeight(i int) int {
	return len(m.viewEntry(m.entries[m.order[i]], i == m.cursor))
}

// visibleEntries is how many whole entries fit from the offset.
func (m model) visibleEntries() int {
	n, used := 0, 0
	for i := m.offset; i < len(m.order); i++ {
		used += m.entryHeight(i)
		if used > m.listHeight()+1 { // the last entry's trailing blank may not fit
			break
		}
		n++
	}
	return n
}

func (m *model) scrollTo() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	for m.offset < m.cursor {
		used := 0
		for i := m.offset; i <= m.cursor; i++ {
			used += m.entryHeight(i)
		}
		if used <= m.listHeight()+1 {
			break
		}
		m.offset++
	}
	m.offset = max(0, min(m.offset, len(m.order)-1))
}

// entryAt is the entry drawn on screen line y, if any.
func (m model) entryAt(y int) (int, bool) {
	line := listTop
	for i := m.offset; i < len(m.order); i++ {
		h := m.entryHeight(i)
		if y >= line && y < line+h-1 { // not its trailing blank
			return i, true
		}
		line += h
		if line >= listTop+m.listHeight() {
			break
		}
	}
	return 0, false
}

// wrap breaks text into lines of at most w cells, at most n of them, the
// last ending in … if the text runs on.
func wrap(text string, w, n int) []string {
	words := strings.Fields(text)
	var lines []string
	cur := ""
	for i, word := range words {
		next := word
		if cur != "" {
			next = cur + " " + word
		}
		if ansi.StringWidth(next) <= w || cur == "" {
			cur = next
			continue
		}
		lines = append(lines, cur)
		cur = word
		if len(lines) == n {
			last := lines[n-1] + " " + strings.Join(words[i:], " ")
			lines[n-1] = shorten(last, w)
			return lines
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	for i, l := range lines {
		lines[i] = shorten(l, w)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

func shorten(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= n {
		return s
	}
	return strings.TrimRight(ansi.Truncate(s, n-1, ""), " ") + "…"
}

// ago is how long ago t was, briefly: "just now", "4m", "2h", "3d".
func ago(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// ── view ──────────────────────────────────────────────────────────────────────

// View is what Bubble Tea writes to the terminal. Every frame passes through
// screenSafe, whatever path its text took to get there.
func (m model) View() string {
	return screenSafe(m.view())
}

func (m model) view() string {
	if m.width == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.viewHeader() + "\n\n")
	h := m.listHeight()
	used := 0
	switch {
	case !m.loaded:
		b.WriteString("  " + m.spin.View() + " Loading…\n")
		used++
	case len(m.order) == 0:
		b.WriteString(styleDim.Render("  No agents running.") + "\n")
		used++
	}
	for i := m.offset; i < len(m.order) && used < h; i++ {
		for _, l := range m.viewEntry(m.entries[m.order[i]], i == m.cursor) {
			if used == h {
				break
			}
			b.WriteString(l + "\n")
			used++
		}
	}
	for ; used < h; used++ {
		b.WriteString("\n")
	}
	b.WriteString(m.statusLine())
	b.WriteString(m.renderFooter(m.footer()))
	return b.String()
}

// viewHeader is the title, and how many agents are in each state.
func (m model) viewHeader() string {
	counts := map[string]int{}
	recapping := 0
	for _, e := range m.entries {
		counts[e.agent.Status]++
		if e.recapping {
			recapping++
		}
	}
	var parts []string
	for _, s := range []struct {
		status, label string
		style         lipgloss.Style
	}{
		{"blocked", "need you", styleBlocked}, {"done", "done", styleDone},
		{"working", "working", styleWorking}, {"idle", "idle", styleIdle},
	} {
		if n := counts[s.status]; n > 0 {
			label := s.label
			if s.status == "blocked" && n == 1 {
				label = "needs you"
			}
			parts = append(parts, s.style.Render(strconv.Itoa(n))+styleDim.Render(" "+label))
		}
	}
	line := styleTabOn.Render(" Recap ")
	if len(parts) > 0 {
		line += "  " + strings.Join(parts, styleDim.Render(" · "))
	}
	if recapping > 0 {
		right := m.spin.View() + styleDim.Render(fmt.Sprintf(" recapping %d ", recapping))
		if pad := m.width - lipgloss.Width(line) - lipgloss.Width(right); pad > 1 {
			line += strings.Repeat(" ", pad) + right
		}
	}
	return line
}

// glyph is the status mark herdr's sidebar uses, in its colour.
func (m model) glyph(status string) string {
	switch status {
	case "blocked":
		return styleBlocked.Render("◉")
	case "working":
		return styleWorking.Render(m.spin.View())
	case "done":
		return styleDone.Render("●")
	case "idle":
		return styleIdle.Render("✓")
	}
	return styleDim.Render("○")
}

// title is what the row is called: the agent's own terminal title (Claude's
// title for the conversation), else its name, else its folder.
func title(a agentInfo) string {
	for _, t := range []string{a.Title, a.Name, filepath.Base(a.Cwd)} {
		if t = strings.TrimSpace(t); t != "" && t != "." && t != "/" && !strings.EqualFold(t, "claude code") {
			return t
		}
	}
	return a.Agent
}

func (m model) viewEntry(e *entry, selected bool) []string {
	w := m.width
	var meta *sessionMeta
	if e.session != nil {
		meta = &e.session.Meta
	}
	// Right of the title: its space, and when it last did anything.
	var right []string
	if len(m.workspaces) > 1 {
		for _, ws := range m.workspaces {
			if ws.WorkspaceID == e.agent.WorkspaceID && ws.Label != "" {
				right = append(right, shorten(ws.Label, 24))
			}
		}
	}
	if s := m.inState(e); s != "" {
		right = append(right, s)
	}
	left := " " + m.glyph(e.agent.Status) + " "
	style := styleTitle
	if e.agent.Status == "working" {
		style = styleWorking.Bold(true)
	}
	rightText := styleDim.Render(strings.Join(right, " · "))
	room := max(1, w-lipgloss.Width(left)-lipgloss.Width(rightText)-2)
	lines := []string{fitRow(left, style.Render(shorten(title(e.agent), room)), rightText, w)}

	if info := m.details(e, meta); len(info) > 0 {
		lines = append(lines, "   "+shorten(strings.Join(info, styleDim.Render(" · ")), max(1, w-4)))
	}
	// The selected agent's model, context and mode, with the other details.
	if selected && meta != nil {
		var about []string
		if meta.Model != "" {
			about = append(about, styleModel.Render(shortModel(meta.Model)))
		}
		if meta.Context > 0 {
			about = append(about, styleContext.Render(shortTokens(meta.Context)+" ctx"))
		}
		if meta.Mode != "" && meta.Mode != "default" {
			about = append(about, styleMode.Render(meta.Mode))
		}
		if len(about) > 0 {
			lines = append(lines, "   "+shorten(strings.Join(about, styleDim.Render(" · ")), max(1, w-4)))
		}
	}
	// What a blocked agent is waiting for: its pending question or request.
	if e.agent.Status == "blocked" && meta != nil && meta.Pending != "" {
		lines = append(lines, "   "+styleBlocked.Render(shorten("? "+meta.Pending, max(1, w-4))))
	}
	body, dim := m.body(e)
	for _, l := range body {
		if dim {
			l = styleDim.Render(l)
		} else {
			l = styleRecap.Render(l)
		}
		lines = append(lines, "   "+l)
	}
	if selected && meta != nil {
		if meta.LastPrompt != "" {
			lines = append(lines, "   "+styleDim.Italic(true).Render(shorten("› "+meta.LastPrompt, max(1, w-4))))
		}
	}
	if selected {
		for i, l := range lines {
			lines[i] = highlight(l, w)
		}
	}
	return append(lines, "")
}

// details is the line under a title, each piece in its own colour: branch,
// uncommitted and unpushed work, task progress, the pane's tokens, and how
// current the recap is. Model, context and mode are the selected row's.
func (m model) details(e *entry, meta *sessionMeta) []string {
	var info []string
	branch := e.branch
	if meta != nil && meta.Branch != "" {
		branch = meta.Branch
	}
	if branch != "" {
		info = append(info, styleBranch.Render("⎇ "+shorten(branch, 40)))
	}
	if c := changesText(e.changes); c != "" {
		info = append(info, c)
	}
	if meta != nil && meta.TasksTotal > 0 {
		info = append(info, styleContext.Render(fmt.Sprintf("%d/%d tasks", meta.TasksDone, meta.TasksTotal)))
	}
	if e.agent.Agent != "claude" && e.agent.Agent != "" {
		info = append(info, styleModel.Render(e.agent.Agent))
	}
	for _, v := range tokenValues(e.agent.Tokens, m.tokens) {
		info = append(info, styleToken.Render(v))
	}
	switch {
	case e.recapping && e.recap != nil:
		info = append(info, styleDim.Render("recapping…"))
	case e.recap != nil && !e.current:
		info = append(info, styleDim.Render("recap from "+ago(m.now(), e.recap.At)))
	}
	return info
}

// changesText is a worktree's work in progress: "4 files +120 −30 ↑1 ↓2",
// the counts in the theme's colours; "" when there's none.
func changesText(c gitChanges) string {
	var parts []string
	if c.Files > 0 {
		unit := " files"
		if c.Files == 1 {
			unit = " file"
		}
		parts = append(parts, styleMode.Render(strconv.Itoa(c.Files)+unit))
	}
	if c.Added > 0 {
		parts = append(parts, styleOK.Render("+"+strconv.Itoa(c.Added)))
	}
	if c.Deleted > 0 {
		parts = append(parts, styleErr.Render("−"+strconv.Itoa(c.Deleted)))
	}
	if c.Ahead > 0 {
		parts = append(parts, styleMode.Render("↑"+strconv.Itoa(c.Ahead)))
	}
	if c.Behind > 0 {
		parts = append(parts, styleMode.Render("↓"+strconv.Itoa(c.Behind)))
	}
	return strings.Join(parts, " ")
}

// inStateSince is when the agent entered its status, as far as its
// conversation shows: when it started waiting on you (its pending request),
// when you last prompted it (working), or its last message (done, idle).
func (m model) inStateSince(e *entry) time.Time {
	if e == nil || e.session == nil {
		return time.Time{}
	}
	meta := e.session.Meta
	switch e.agent.Status {
	case "blocked":
		if !meta.PendingAt.IsZero() {
			return meta.PendingAt
		}
	case "working":
		if !meta.PromptAt.IsZero() {
			return meta.PromptAt
		}
	}
	return meta.Active
}

// inState is the status and how long it's lasted: "waiting 3m", "done 25m".
func (m model) inState(e *entry) string {
	since := m.inStateSince(e)
	if since.IsZero() {
		return ""
	}
	label := map[string]string{"blocked": "waiting", "working": "working", "done": "done", "idle": "idle"}[e.agent.Status]
	if label == "" {
		return ""
	}
	return label + " " + duration(m.now().Sub(since))
}

// duration is a span, briefly: "<1m", "4m", "2h", "3d".
func duration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "<1m"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// tokenValues are a pane's tokens to show, as values: the ones named in
// config.json in that order, or else all of them by name, less herdr-github's
// pr_* details when its pr is there (the pr says it).
func tokenValues(tokens map[string]string, only []string) []string {
	var names []string
	if len(only) > 0 {
		names = only
	} else {
		for k := range tokens {
			if strings.HasPrefix(k, "pr_") && tokens["pr"] != "" {
				continue
			}
			names = append(names, k)
		}
		sort.Strings(names)
	}
	var vals []string
	seen := map[string]bool{}
	for _, k := range names {
		if v := strings.TrimSpace(tokens[k]); v != "" && !seen[v] {
			seen[v] = true
			vals = append(vals, shorten(v, 30))
		}
	}
	return vals
}

// fitRow lays out left + title + right-aligned meta in exactly w cells,
// truncating the title when space runs out.
func fitRow(left, title, right string, w int) string {
	lw, rw := lipgloss.Width(left), lipgloss.Width(right)
	room := w - lw - rw - 2
	if room < 8 {
		right, rw = "", 0
		room = w - lw - 1
	}
	title = shorten(title, max(1, room))
	pad := max(1, w-lw-lipgloss.Width(title)-rw-1)
	return left + title + strings.Repeat(" ", pad) + right
}

// highlight gives a whole line the selection background, w cells wide. The
// line's own coloured pieces each end in a style reset, which would also end
// the background, so it's switched back on after every one.
func highlight(line string, w int) string {
	on := styleSelected.Render("x")
	on = on[:strings.Index(on, "x")] // the escape that turns the background on
	if on != "" {
		line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+on)
	}
	return styleSelected.Width(w).Render(line)
}

// statusLine is the one line above the footer: the last error, or a note.
func (m model) statusLine() string {
	if m.replying {
		name := "agent"
		if e := m.entries[m.replyTo]; e != nil {
			name = shorten(title(e.agent), 30)
		}
		return " " + styleTabOn.Render("Reply to "+name+":") + " " + m.replyText.View() + "\n"
	}
	switch {
	case m.err != "":
		return " " + styleErr.Render(shorten(clean(m.err, false), max(10, m.width-2))) + "\n"
	case m.flash != "":
		return " " + styleDim.Render(shorten(clean(m.flash, false), max(10, m.width-2))) + "\n"
	}
	return "\n"
}

func (m model) footer() []hint {
	if m.replying {
		return []hint{{"enter send", "enter"}, {"esc cancel", "esc"}}
	}
	return []hint{{"enter go to agent", "enter"}, {"p reply", "p"}, {"r recap again", "r"}, {"esc close", "esc"}}
}

// program is the running popup.
var program *tea.Program

func runPicker(ctx context.Context, cfg config, demo bool) error {
	// Ask the terminal for its background now: once the program owns stdin,
	// the reply would arrive as stray input.
	if cfg.Theme != "dark" && cfg.Theme != "light" {
		cfg.Theme = "light"
		if lipgloss.HasDarkBackground() {
			cfg.Theme = "dark"
		}
	}
	useTheme(pickerTheme(cfg.Theme == "dark"))
	var src source = liveSource{cfg}
	if demo {
		src = newDemoSource()
	}
	m := newModel(ctx, src)
	m.tokens = cfg.Tokens
	program = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())
	_, err := program.Run()
	if errors.Is(err, tea.ErrProgramKilled) {
		return nil
	}
	return err
}

// debugList prints the rows as the popup would find them, without drawing
// it: for checking what herdr and Claude report.
func debugList(cfg config) error {
	src := liveSource{cfg}
	agents, _, err := src.agents()
	if err != nil {
		return err
	}
	for _, a := range agents {
		fmt.Printf("%s  %-8s %-7s %s\n", a.PaneID, a.Status, a.Agent, title(a))
		if a.Agent != "claude" {
			continue
		}
		s, err := src.session(a.PaneID)
		if err != nil {
			fmt.Println("    session:", err)
			continue
		}
		fmt.Printf("    session %s  config %s\n    transcript %s\n", s.ID, s.ConfigDir, s.Transcript)
		mt := readMeta(s.Transcript)
		c := readChanges(a.Cwd)
		fmt.Printf("    branch %q  model %s  ctx %d  mode %q  tasks %d/%d  changes %+v\n",
			mt.Branch, mt.Model, mt.Context, mt.Mode, mt.TasksDone, mt.TasksTotal, c)
		fmt.Printf("    active %s  prompted %s  pending %q\n    last prompt %q\n",
			mt.Active.Format(time.RFC3339), mt.PromptAt.Format(time.RFC3339), mt.Pending, shorten(mt.LastPrompt, 80))
		if r, current := src.cached(s); r != nil {
			fmt.Printf("    recap (%s, current=%v): %s\n", r.At.Format(time.RFC3339), current, r.Text)
		}
	}
	fmt.Fprintln(os.Stderr, "state:", stateDir())
	return nil
}
