package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// How the machine is doing, at the right of the tabs, when herdr-vitals is
// installed: a small graph of its CPU since the popup opened and a gauge of
// its memory. herdr-vitals does the reading (`herdr-vitals stream`, a line a
// second, on Linux or macOS); this draws it, and a click on it opens
// herdr-vitals' popup, with this one to come back to.

// vitalsPlugin is herdr-vitals.
const vitalsPlugin = "herdr-vitals"

// vitalsReading is a line of herdr-vitals' stream.
type vitalsReading struct {
	CPU   float64 `json:"cpu"`  // 0–100, every core together
	Mem   float64 `json:"mem"`  // % in use
	Load  float64 `json:"load"` // the 1-minute load a core
	Cores int     `json:"cores"`
}

// vitalsKept is how many readings the graph remembers: more than it's ever
// drawn wide.
const vitalsKept = 64

type (
	vitalsStartMsg struct{ ch <-chan vitalsReading }
	vitalsMsg      vitalsReading
)

// vitals runs herdr-vitals' stream, as herdr would run that plugin: from its
// folder, with its environment. Nil if it isn't installed.
func (liveSource) vitals(ctx context.Context) <-chan vitalsReading {
	p := popupRef{PluginID: vitalsPlugin, Entrypoint: "picker"}
	root, command, err := paneCommand(p)
	if err != nil {
		return nil
	}
	path := command[0]
	if strings.Contains(path, "/") && !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	cmd := exec.CommandContext(ctx, path, "stream")
	cmd.Dir, cmd.Env = root, swapEnv(os.Environ(), pluginID(), p, root)
	out, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return nil
	}
	ch := make(chan vitalsReading)
	go func() {
		defer close(ch)
		defer cmd.Wait()
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			var r vitalsReading
			if json.Unmarshal(sc.Bytes(), &r) != nil {
				continue
			}
			select {
			case ch <- r:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

func (m model) startVitals() tea.Cmd {
	src, ctx := m.src, m.ctx
	return func() tea.Msg {
		if ch := src.vitals(ctx); ch != nil {
			return vitalsStartMsg{ch}
		}
		return nil
	}
}

// nextVitals waits for the stream's next reading. When it ends (herdr-vitals
// gone, or the popup closing), the graph stays as it last was.
func nextVitals(ch <-chan vitalsReading) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return nil
		}
		return vitalsMsg(r)
	}
}

// vitalsLevel is the colour of a reading: its own while calm, yellow from
// 80% and red from 90%, as herdr-vitals has them.
func vitalsColor(hue lipgloss.TerminalColor, pct float64) lipgloss.TerminalColor {
	switch {
	case pct >= 90:
		return look.Red
	case pct >= 80:
		return look.Yellow
	}
	return hue
}

var vitalsEighths = []rune("▁▂▃▄▅▆▇█")

// vitalsSpark is a one-line graph of the newest width readings, the newest
// on the right; the baseline where there are none yet.
func vitalsSpark(vals []float64, width int) string {
	if len(vals) > width {
		vals = vals[len(vals)-width:]
	}
	out := lipgloss.NewStyle().Foreground(look.Surface).Render(strings.Repeat("▁", width-len(vals)))
	for _, v := range vals {
		i := min(7, max(0, int(math.Ceil(v/100*8))-1))
		out += lipgloss.NewStyle().Foreground(vitalsColor(look.Blue, v)).Render(string(vitalsEighths[i]))
	}
	return out
}

// vitalsGauge is a bar width cells long, filled to pct.
func vitalsGauge(pct float64, width int) string {
	full := min(width, int(math.Round(pct/100*float64(width))))
	return lipgloss.NewStyle().Foreground(vitalsColor(look.Mauve, pct)).Render(strings.Repeat("━", full)) +
		lipgloss.NewStyle().Foreground(look.Surface).Render(strings.Repeat("━", width-full))
}

// vitalsStrip is the machine's CPU and memory as they fit in room cells:
// a graph and a gauge, smaller ones, the CPU alone, or nothing. From the
// moment the stream starts it's there, empty until the first reading, so
// the header doesn't shift when it comes.
func (m model) vitalsStrip(room int) string {
	if len(m.cpu) == 0 && m.vitalsCh == nil {
		return ""
	}
	pct := func(hue lipgloss.TerminalColor, v float64) string {
		text := strconv.Itoa(int(v+0.5)) + "%"
		if len(m.cpu) == 0 {
			text = "–"
		}
		return lipgloss.NewStyle().Foreground(vitalsColor(hue, v)).Bold(true).
			Render(lipgloss.PlaceHorizontal(4, lipgloss.Right, text))
	}
	cpu := 0.0
	if len(m.cpu) > 0 {
		cpu = m.cpu[len(m.cpu)-1]
	}
	for _, size := range [][2]int{{16, 8}, {10, 6}, {8, 0}} {
		s := styleDim.Render("cpu ") + vitalsSpark(m.cpu, size[0]) + " " + pct(look.Blue, cpu)
		if size[1] > 0 {
			s += styleDim.Render("  mem ") + vitalsGauge(m.mem, size[1]) + " " + pct(look.Mauve, m.mem)
		}
		if lipgloss.Width(s) <= room {
			return s
		}
	}
	return ""
}

// openVitals closes this popup for herdr-vitals', with this one to come
// back to.
func (m model) openVitals() (tea.Model, tea.Cmd) {
	to, ok := m.links[vitalsPlugin]
	if !ok {
		return m, nil
	}
	pane := ""
	if e := m.selected(); e != nil {
		pane = e.agent.PaneID
	}
	back, _ := json.Marshal(here(pane))
	to.Env = map[string]string{returnEnv: string(back)}
	src := m.src
	return m, func() tea.Msg { return openedMsg{src.open(to)} }
}

// vitalsHit is how wide the header's strip is and where it starts, for a
// click on it: 0 wide if it isn't there.
func (m model) vitalsHit() (int, int) {
	recapping := 0
	for _, e := range m.entries {
		if e.recapping {
			recapping++
		}
	}
	line, _ := m.tabBar()
	after := 0
	if recapping > 0 {
		after = lipgloss.Width(m.spin.View() + styleDim.Render(fmt.Sprintf(" recapping %d ", recapping)))
		if m.width-lipgloss.Width(line)-after <= 1 {
			after = 0
		}
	}
	strip, x0 := m.headerVitals(lipgloss.Width(line), after)
	return lipgloss.Width(strip), x0
}
