package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// hint is one footer entry. Clicking it presses its key.
type hint struct {
	label string
	key   string // "" = not clickable
}

const hintSep = " · "

var (
	defaultStyleHintHot = lipgloss.NewStyle().Bold(true).Underline(true)
	styleHintHot        = defaultStyleHintHot
)

// renderFooter draws the hints dim, with the one under the pointer lit up so
// it reads as clickable.
func (m model) renderFooter(hs []hint) string {
	hot := ""
	if m.mouseY == m.height-1 {
		hot = hintAt(hs, m.mouseX)
	}
	parts := make([]string, len(hs))
	for i, h := range hs {
		if h.key != "" && h.key == hot {
			parts[i] = styleHintHot.Render(h.label)
		} else {
			parts[i] = styleDim.Render(h.label)
		}
	}
	return styleDim.Render(" ") + strings.Join(parts, styleDim.Render(hintSep))
}

// hintAt finds the footer entry under column x, laid out as renderFooter does.
func hintAt(hs []hint, x int) string {
	pos := 1
	for _, h := range hs {
		w := lipgloss.Width(h.label)
		if x >= pos && x < pos+w {
			return h.key
		}
		pos += w + lipgloss.Width(hintSep)
	}
	return ""
}

// keyMsg turns a hint's key back into the key press it stands for.
func keyMsg(k string) tea.KeyMsg {
	switch k {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

// handleMouse: hovering highlights, one click goes to the agent, the wheel
// scrolls. Footer hints are buttons.
func (m model) handleMouse(ev tea.MouseMsg) (tea.Model, tea.Cmd) {
	m.mouseX, m.mouseY = ev.X, ev.Y
	if ev.Action == tea.MouseActionMotion {
		// Hover highlights, like a launcher: the click then opens what you
		// see. Not while replying: the reply is to the selected agent.
		if i, ok := m.entryAt(ev.Y); ok && !m.replying && i != m.cursor {
			m.cursor = i
			m.scrollTo() // selected, it grows: keep it all on screen
		}
		return m, nil
	}
	if ev.Action != tea.MouseActionPress {
		return m, nil
	}
	switch ev.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		if !m.replying {
			if ev.Button == tea.MouseButtonWheelUp {
				m.move(-1)
			} else {
				m.move(1)
			}
		}
		return m, nil
	case tea.MouseButtonLeft:
	default:
		return m, nil
	}
	if ev.Y == m.height-1 {
		if k := hintAt(m.footer(), ev.X); k != "" {
			return m.handleKey(keyMsg(k))
		}
		return m, nil
	}
	// A click in the list while replying would leave, losing the reply.
	if i, ok := m.entryAt(ev.Y); ok && !m.replying {
		m.cursor, m.flash = i, ""
		return m.activate()
	}
	return m, nil
}
