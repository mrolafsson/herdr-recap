package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopilotRecapFromSessionFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("COPILOT_HOME", home)
	id := "4d58707b-d497-4d49-94b0-931a57cb7694"
	dir := filepath.Join(home, "session-state", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "workspace.yaml"), []byte("id: x\nname: Demo Session\n"), 0o644)
	log := `{"type":"user.message","data":{"content":"first question"}}
{"type":"assistant.message","data":{"content":"first answer"}}
{"type":"user.message","data":{"content":"add a hotkey"}}
{"type":"assistant.message","data":{"content":"Done, bound alt+r."}}
`
	os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(log), 0o644)
	a := agentInfo{Agent: "copilot", AgentSession: agentSession{Kind: "id", Value: id}}
	if !recapCapable(config{Agents: []string{"copilot"}}, a) {
		t.Fatal("copilot pane with an ID should be recapable")
	}
	s, err := resolveAgentSession(context.Background(), config{}, a)
	if err != nil {
		t.Fatal(err)
	}
	r, err := runRecap(context.Background(), config{}, s)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Demo Session", "Asked: add a hotkey", "Last reply: Done, bound alt+r."} {
		if !strings.Contains(r.Text, want) {
			t.Errorf("recap %q lacks %q", r.Text, want)
		}
	}
}
