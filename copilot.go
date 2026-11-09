package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// GitHub Copilot CLI agents are recapped from the session's own files, with no
// model call: herdr reports the session ID, and Copilot keeps the session's
// name (workspace.yaml) and its event log (events.jsonl) under
// ~/.copilot/session-state/<id>/. The recap is the session's name, the last
// thing the user asked, and how the agent last answered.

// copilotTail is how much of the end of events.jsonl is read, since a
// long-running session's log is big.
const copilotTail = 2 << 20

func copilotHome() string {
	if d := os.Getenv("COPILOT_HOME"); d != "" {
		return d
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(h, ".copilot")
}

func copilotSessionDir(id string) string {
	home := copilotHome()
	if home == "" || !sessionIDPattern.MatchString(id) {
		return ""
	}
	return filepath.Join(home, "session-state", id)
}

func resolveCopilotSession(a agentInfo) (claudeSession, error) {
	id := a.AgentSession.Value
	if a.AgentSession.Kind != "id" || copilotSessionDir(id) == "" {
		return claudeSession{}, errors.New("herdr has no Copilot session ID for this pane")
	}
	dir := copilotSessionDir(id)
	s := claudeSession{ID: id, Agent: "copilot", Native: true, Cwd: a.Cwd}
	if st, err := os.Stat(filepath.Join(dir, "events.jsonl")); err == nil {
		s.Transcript = filepath.Join(dir, "events.jsonl")
		s.Stamp = st.ModTime().UTC().Format(time.RFC3339Nano)
	}
	return s, nil
}

var copilotName = regexp.MustCompile(`(?m)^name:\s*(.+)$`)

// readTail returns the last n bytes of a file, from a line start.
func readTail(path string, n int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	skip := false
	if st.Size() > n {
		if _, err := f.Seek(st.Size()-n, io.SeekStart); err != nil {
			return nil, err
		}
		skip = true
	}
	r := bufio.NewReaderSize(f, 1<<20)
	if skip {
		_, _ = r.ReadBytes('\n')
	}
	return io.ReadAll(r)
}

// copilotSaid is the last user message and the last assistant text in the log.
func copilotSaid(path string) (asked, answered string, err error) {
	data, err := readTail(path, copilotTail)
	if err != nil {
		return "", "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		var e struct {
			Type string `json:"type"`
			Data struct {
				Content string `json:"content"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		text := strings.Join(strings.Fields(clean(e.Data.Content, false)), " ")
		switch {
		case e.Type == "user.message" && text != "":
			asked, answered = text, ""
		case e.Type == "assistant.message" && text != "":
			answered = text
		}
	}
	return asked, answered, nil
}

func runCopilotRecap(_ context.Context, _ config, s claudeSession) (recap, error) {
	dir := copilotSessionDir(s.ID)
	if dir == "" || s.Transcript == "" {
		return recap{}, errors.New("Nothing to recap yet")
	}
	asked, answered, err := copilotSaid(s.Transcript)
	if err != nil {
		return recap{}, err
	}
	if asked == "" && answered == "" {
		return recap{}, errors.New("Nothing to recap yet")
	}
	var parts []string
	if y, err := os.ReadFile(filepath.Join(dir, "workspace.yaml")); err == nil {
		if m := copilotName.FindSubmatch(y); m != nil {
			parts = append(parts, strings.Trim(strings.TrimSpace(string(m[1])), `"'`)+".")
		}
	}
	if asked != "" {
		parts = append(parts, "Asked: "+shorten(asked, 160)+".")
	}
	if answered != "" {
		parts = append(parts, "Last reply: "+shorten(answered, 280))
	}
	return recap{Session: s.ID, Source: "copilot", Stamp: s.Stamp,
		Text: strings.Join(parts, " "), At: time.Now()}, nil
}
