package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// What the Herdr app's Recap mod (app/) asks of this machine. The app runs
// its agents in herdr-core, not the herdr server, so it names an agent by its
// Claude process rather than a herdr pane, and runs this where the agent runs.

// sessionAsk is one agent the mod shows: its Claude process, its title, and
// the folder it's in now (where it may have moved since it started).
type sessionAsk struct {
	PID   int    `json:"pid"`
	Title string `json:"title,omitempty"`
	Cwd   string `json:"cwd,omitempty"`
}

// sessionInfo is what the mod is told of it: what the popup shows of an
// agent, short of what herdr-github and herdr-linear know. Recap is the
// cached one, written for an earlier point when Fresh is false; Error says
// why there's none.
type sessionInfo struct {
	PID     int       `json:"pid"`
	Session string    `json:"session,omitempty"`
	Recap   string    `json:"recap,omitempty"`
	At      time.Time `json:"at,omitzero"`
	Fresh   bool      `json:"fresh"`
	// Title: a better one, when the agent's own is poor.
	Title string `json:"title,omitempty"`
	// From the transcript.
	Model      string    `json:"model,omitempty"`
	Mode       string    `json:"mode,omitempty"`
	LastPrompt string    `json:"last_prompt,omitempty"`
	PromptAt   time.Time `json:"prompt_at,omitzero"`
	Active     time.Time `json:"active,omitzero"`
	Pending    string    `json:"pending,omitempty"`
	PendingAt  time.Time `json:"pending_at,omitzero"`
	TasksDone  int       `json:"tasks_done,omitempty"`
	TasksTotal int       `json:"tasks_total,omitempty"`
	// From git: the branch, its ticket's key, what's not committed or
	// pushed, and where it pushes to, for herdr-github to find its PR by.
	Branch  string     `json:"branch,omitempty"`
	Ticket  string     `json:"ticket,omitempty"`
	Changes gitChanges `json:"changes"`
	Remote  string     `json:"remote,omitempty"`
	Head    string     `json:"head,omitempty"`
	Error   string     `json:"error,omitempty"`
}

// runSessions prints each asked-for agent's session and cached recap as JSON.
// --write: recaps out of date are written too, by a copy of this program that
// carries on after this one returns (the mod's calls are cut off at 30 s, and
// a recap can take longer); the mod finds them on its next ask. --now (what
// that copy runs): write them here and now.
func runSessions(ctx context.Context, cfg config, args []string) error {
	fs := flag.NewFlagSet("sessions", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	write := fs.Bool("write", false, "")
	now := fs.Bool("now", false, "")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("sessions: %w", err)
	}
	if fs.NArg() != 1 {
		return errors.New(`sessions needs one argument: [{"pid": N, "title": "…"}, …]`)
	}
	var asks []sessionAsk
	if err := json.Unmarshal([]byte(fs.Arg(0)), &asks); err != nil {
		return fmt.Errorf("sessions: %w", err)
	}

	if *now {
		for _, a := range asks {
			s, err := sessionForPID(a.PID, "claude")
			var ran bool
			if err == nil {
				_, ran, err = ensureRecap(ctx, cfg, s, a.Title, false)
			}
			// A recap that was fresh asked for no title: ask for it now.
			if err == nil && !ran && wantsTitle(s, loadRecap(s.ID), a.Title, a.Cwd, cfg.Language) {
				_, err = ensureTitle(ctx, cfg, s, a.Title)
			}
			if err != nil && !errors.Is(err, errNothingYet) {
				fmt.Fprintln(os.Stderr, "herdr-recap: recap of", a.PID, "failed:", err)
			}
		}
		return nil
	}

	out := make([]sessionInfo, 0, len(asks))
	var stale []sessionAsk
	for _, a := range asks {
		info := sessionInfo{PID: a.PID}
		s, err := sessionForPID(a.PID, "claude")
		if err != nil {
			info.Error = err.Error()
			out = append(out, info)
			continue
		}
		info.Session = s.ID
		meta := readMeta(s.Transcript)
		info.Model, info.Mode, info.LastPrompt = shortModel(meta.Model), meta.Mode, meta.LastPrompt
		info.PromptAt, info.Active = meta.PromptAt, meta.Active
		info.Pending, info.PendingAt = meta.Pending, meta.PendingAt
		info.TasksDone, info.TasksTotal = meta.TasksDone, meta.TasksTotal
		dir := a.Cwd
		if dir == "" {
			dir = s.Cwd
		}
		info.Branch = gitBranch(dir)
		if info.Branch == "" {
			info.Branch = meta.Branch
		}
		info.Ticket = ticketKey(info.Branch)
		info.Changes = readChanges(dir)
		info.Remote, info.Head = gitPushTarget(dir)
		c := cachedRecap(s.ID, cfg.Language)
		info.Fresh = fresh(c, s, cfg.Language)
		if c != nil {
			info.Recap, info.At = c.Text, c.At
			if !c.Settled {
				info.Title = c.Title
			}
		}
		if s.Transcript != "" && (!info.Fresh || wantsTitle(s, c, a.Title, dir, cfg.Language)) {
			stale = append(stale, a)
		}
		out = append(out, info)
	}
	if *write && len(stale) > 0 {
		ask, _ := json.Marshal(stale)
		if err := spawn("sessions", "--now", string(ask)); err != nil {
			fmt.Fprintln(os.Stderr, "herdr-recap: writing recaps:", err)
		}
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}

// sessionFound is an agent whose conversation has the mod's filter: some of
// what was said there, as the popup shows it in place of the recap.
type sessionFound struct {
	PID  int    `json:"pid"`
	Text string `json:"text"`
}

// runSearch prints the asked-for agents whose conversations say every word
// of the filter, as JSON: the mod's filter, looked for past its rows.
func runSearch(ctx context.Context, cfg config, args []string) error {
	if len(args) != 2 {
		return errors.New(`search needs two arguments: [{"pid": N}, …] and the filter`)
	}
	var asks []sessionAsk
	if err := json.Unmarshal([]byte(args[0]), &asks); err != nil {
		return fmt.Errorf("search: %w", err)
	}
	return json.NewEncoder(os.Stdout).Encode(searchAsked(ctx, cfg, asks, args[1]))
}

// searchAsked searches each asked-for agent's conversation for the filter, a
// few at a time, and keeps the ones that have it, in the order asked. An
// agent with no session to read is passed over.
func searchAsked(ctx context.Context, cfg config, asks []sessionAsk, filter string) []sessionFound {
	found := make([]sessionFound, 0, len(asks))
	words := searchWords(filter)
	if words == nil {
		return found
	}
	texts := make([]string, len(asks))
	var wg sync.WaitGroup
	room := make(chan struct{}, 4)
	for i, a := range asks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			room <- struct{}{}
			defer func() { <-room }()
			if s, err := sessionForPID(a.PID, "claude"); err == nil {
				texts[i] = searchSession(ctx, cfg, s, words)
			}
		}()
	}
	wg.Wait()
	for i, a := range asks {
		if texts[i] != "" {
			found = append(found, sessionFound{PID: a.PID, Text: texts[i]})
		}
	}
	return found
}

// gitPushTarget is where dir's branch pushes: origin's URL, and its
// upstream's branch name ("" when it has none yet).
func gitPushTarget(dir string) (remote, head string) {
	if dir == "" {
		return "", ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	git := func(args ...string) string {
		out, _ := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
		return strings.TrimSpace(string(out))
	}
	remote = git("remote", "get-url", "origin")
	if up := git("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); up != "" {
		_, head, _ = strings.Cut(up, "/")
	}
	return remote, head
}

// wantsTitle: the agent's title is poor and no better one has been
// suggested, or turned down, yet.
func wantsTitle(s claudeSession, c *recap, title, dir, language string) bool {
	if c != nil && (c.Settled || c.Renaming || (c.Title != "" && c.TitleLanguage == language)) {
		return false
	}
	if dir == "" {
		dir = s.Cwd
	}
	return titleable(s) && poorTitle(title, gitBranch(dir))
}

// runTitle asks for a better title for each asked-for agent, whatever its own
// is like: the app's ctrl+r on an agent with none suggested, as the TUI's.
// Like `sessions --write`, a copy of this program does it in the background
// (a title can take longer than the mod waits), and the suggestion comes with
// the next `sessions`. --now: here and now.
func runTitle(ctx context.Context, cfg config, args []string) error {
	now := len(args) > 0 && args[0] == "--now"
	if now {
		args = args[1:]
	}
	if len(args) != 1 {
		return errors.New(`title needs one argument: [{"pid": N, "title": "…"}, …]`)
	}
	var asks []sessionAsk
	if err := json.Unmarshal([]byte(args[0]), &asks); err != nil {
		return fmt.Errorf("title: %w", err)
	}
	if !now {
		if err := spawn("title", "--now", args[0]); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]int{"asked": len(asks)})
	}
	for _, a := range asks {
		s, err := sessionForPID(a.PID, "claude")
		if err == nil {
			_, err = ensureTitle(ctx, cfg, s, a.Title)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "herdr-recap: a title for", a.PID, "failed:", err)
		}
	}
	return nil
}
