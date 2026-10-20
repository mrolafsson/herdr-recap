package main

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// The machine's CPU and memory sit right of the tabs when there's room,
// smaller when there's less, and not at all when there's none: the tabs
// always come first, and nothing goes past the edge.
func TestTheMachineFitsBesideTheTabs(t *testing.T) {
	m, _ := demoModel(t)
	for i := range 40 {
		next, _ := m.Update(vitalsMsg(demoVitals(i)))
		m = next.(model)
	}
	tabsOnly := func(m model) string { l, _ := m.tabBar(); return ansi.Strip(l) }
	for _, w := range []int{200, 140, 110, 100, 90, 80, 70, 60} {
		m.width = w
		head := m.viewHeader()
		if lipgloss.Width(head) > w {
			t.Errorf("%d wide: header is %d", w, lipgloss.Width(head))
		}
		if !strings.HasPrefix(ansi.Strip(head), tabsOnly(m)) {
			t.Errorf("%d wide: the tabs were cut for it", w)
		}
		plain := ansi.Strip(head)
		switch {
		case w >= 140 && !strings.Contains(plain, "mem "):
			t.Errorf("%d wide: no memory: %q", w, plain)
		case w <= 60 && strings.Contains(plain, "cpu "):
			// Only if the tabs left room; at 60 they don't.
			if lipgloss.Width(tabsOnly(m))+20 > w {
				t.Errorf("%d wide: squeezed in: %q", w, plain)
			}
		}
	}
}

func TestNoVitalsNoStrip(t *testing.T) {
	m, _ := demoModel(t)
	m.cpu = nil
	m.width = 200
	if strings.Contains(ansi.Strip(m.viewHeader()), "cpu ") {
		t.Error("a strip with nothing to draw")
	}
}

func TestAClickOnTheMachineOpensVitals(t *testing.T) {
	m, src := linkModel(t, false)
	m.width = 160
	m.links[vitalsPlugin] = popupRef{PluginID: vitalsPlugin, Entrypoint: "picker", Width: "90%", Height: "85%"}
	w, x0 := m.vitalsHit()
	if w == 0 {
		t.Fatal("no strip to click")
	}
	_, cmd := m.Update(tea.MouseMsg{X: x0 + 2, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if cmd == nil {
		t.Fatal("the click did nothing")
	}
	cmd()
	if len(src.opened) != 1 || src.opened[0].PluginID != vitalsPlugin || src.opened[0].Width != "90%" {
		t.Fatalf("opened %+v", src.opened)
	}
	// With this popup to come back to, on the agent it was on.
	var back popupRef
	if err := json.Unmarshal([]byte(src.opened[0].Env[returnEnv]), &back); err != nil || back.PluginID != "herdr-recap" || back.Env[selectEnv] != "w1:p4" {
		t.Errorf("back %+v %v", back, err)
	}
}

// The graph fills as the readings come: each one waits for the next.
func TestTheGraphFillsAsReadingsCome(t *testing.T) {
	m, _ := demoModel(t)
	m.cpu = nil
	ch := make(chan vitalsReading)
	go func() {
		for i := range 5 {
			ch <- demoVitals(i)
		}
		close(ch)
	}()
	m = settle(t, m, func() tea.Msg { return vitalsStartMsg{ch} })
	if len(m.cpu) != 5 {
		t.Fatalf("the graph has %d readings of 5", len(m.cpu))
	}
}

// Before the first reading the strip is there, empty: the header doesn't
// shift when it comes.
func TestTheStripIsThereBeforeTheFirstReading(t *testing.T) {
	m, _ := demoModel(t)
	m.cpu, m.width = nil, 200
	m.vitalsCh = make(chan vitalsReading)
	before := ansi.Strip(m.viewHeader())
	if !strings.Contains(before, "cpu ") || !strings.Contains(before, "–") {
		t.Fatalf("no empty strip: %q", before)
	}
	next, _ := m.Update(vitalsMsg(demoVitals(3)))
	after := ansi.Strip(next.(model).viewHeader())
	if strings.Index(after, "cpu ") != strings.Index(before, "cpu ") {
		t.Errorf("the strip moved:\n%q\n%q", before, after)
	}
}

// What's recapping comes and goes left of the machine, which stays put.
func TestRecappingDoesNotMoveTheMachine(t *testing.T) {
	m, _ := demoModel(t)
	for i := range 20 {
		next, _ := m.Update(vitalsMsg(demoVitals(i)))
		m = next.(model)
	}
	m.width = 200
	for _, e := range m.entries {
		e.recapping = false
	}
	before := ansi.Strip(m.viewHeader())
	for _, e := range m.entries {
		e.recapping = true
		break
	}
	after := ansi.Strip(m.viewHeader())
	if !strings.Contains(after, "recapping 1") {
		t.Fatalf("not recapping: %q", after)
	}
	// Columns on the screen, not bytes: the spinner is several bytes, one cell.
	col := func(s, of string) int { return lipgloss.Width(s[:strings.Index(s, of)]) }
	if col(after, "cpu ") != col(before, "cpu ") || col(after, "recapping") > col(after, "cpu ") {
		t.Errorf("the machine moved, or recapping isn't left of it:\n%q\n%q", before, after)
	}
	_, x0 := m.vitalsHit()
	if x0 != lipgloss.Width(strings.SplitN(m.viewHeader(), "cpu ", 2)[0]) {
		t.Errorf("a click looks for it at %d", x0)
	}
}
