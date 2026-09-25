package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const testSession = "aa81ff42-9e20-4d21-ab0c-8eba1d667245"

// withEnv stands in a process environment for readProcessEnv.
func withEnv(t *testing.T, env map[string]string, err error) {
	t.Helper()
	old := processEnv
	processEnv = func(int) (map[string]string, error) { return env, err }
	t.Cleanup(func() { processEnv = old })
}

// claudeDir makes a Claude config dir with a running session for pid, started
// in cwd, and (with transcript) its conversation.
func claudeDir(t *testing.T, pid, cwd string, transcript bool) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "sessions", pid+".json"), `{"pid":`+pid+`,"sessionId":"`+testSession+`","cwd":"`+cwd+`","status":"busy"}`)
	if transcript {
		write(t, filepath.Join(dir, "projects", "-home-me-code-app", testSession+".jsonl"), `{"type":"user"}`+"\n")
	}
	return dir
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSessionComesFromTheProcessNotHerdr(t *testing.T) {
	// herdr's agent_session can name another session under a profile
	// switcher; the claude process's own config dir and session file don't.
	dir := claudeDir(t, "4242", "/home/me/code/app", true)
	withEnv(t, map[string]string{"CLAUDE_CONFIG_DIR": dir, "PATH": "/bin"}, nil)
	s, err := sessionForPID(4242, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != testSession || s.ConfigDir != dir || !s.ConfigDirSet || s.Cwd != "/home/me/code/app" {
		t.Errorf("%+v", s)
	}
	if want := filepath.Join(dir, "projects", "-home-me-code-app", testSession+".jsonl"); s.Transcript != want {
		t.Errorf("transcript %q, want %q", s.Transcript, want)
	}
}

func TestSessionDefaultsToDotClaude(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude")
	write(t, filepath.Join(dir, "sessions", "7.json"), `{"sessionId":"`+testSession+`","cwd":"/x"}`)
	withEnv(t, map[string]string{}, nil)
	s, err := sessionForPID(7, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.ConfigDir != dir || s.ConfigDirSet {
		t.Errorf("%+v", s)
	}
	if s.Transcript != "" {
		t.Errorf("no conversation yet, but transcript %q", s.Transcript)
	}
}

func TestSessionErrors(t *testing.T) {
	dir := t.TempDir()
	withEnv(t, nil, errors.New("permission denied"))
	if _, err := sessionForPID(1, ""); err == nil {
		t.Error("an unreadable environment")
	}
	withEnv(t, map[string]string{"CLAUDE_CONFIG_DIR": dir}, nil)
	if _, err := sessionForPID(1, ""); err == nil {
		t.Error("no session file")
	}
	write(t, filepath.Join(dir, "sessions", "1.json"), `{"sessionId":"../../etc/passwd"}`)
	if _, err := sessionForPID(1, ""); err == nil {
		t.Error("a session ID that isn't one is refused, not used in a path")
	}
}

func TestTranscriptStartedElsewhere(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "projects", "-somewhere-else", testSession+".jsonl"), "{}\n")
	if got := findTranscript(dir, "/home/me/code/app", testSession); got != filepath.Join(dir, "projects", "-somewhere-else", testSession+".jsonl") {
		t.Errorf("got %q", got)
	}
}

func TestProjectDirIsTheCwdWithDashes(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "projects", "-home-me-my-app-v2-x", testSession+".jsonl")
	write(t, want, "{}\n")
	if got := findTranscript(dir, "/home/me/my_app.v2 x", testSession); got != want {
		t.Errorf("got %q", got)
	}
}

func TestClaudePIDSkipsTheWrapper(t *testing.T) {
	// A profile switcher (clauth start me) runs claude as its child: both
	// are in the foreground, the wrapper first.
	procs := []process{
		{PID: 10, Name: "clauth", Argv: []string{"clauth", "start", "me"}},
		{PID: 11, Name: "claude", Argv: []string{"/home/me/.local/bin/claude", "--permission-mode", "default"}},
	}
	if pid, argv0 := claudePID(procs); pid != 11 || argv0 != "/home/me/.local/bin/claude" {
		t.Errorf("got %d %q", pid, argv0)
	}
	if pid, argv0 := claudePID([]process{{PID: 3, Name: "claude", Argv: []string{"claude"}}}); pid != 3 || argv0 != "claude" {
		t.Errorf("claude from PATH: got %d %q", pid, argv0)
	}
	if pid, _ := claudePID([]process{{PID: 3, Name: "zsh", Argv: []string{"-zsh"}}}); pid != 0 {
		t.Errorf("no claude: got %d", pid)
	}
}

func TestTheRecapRunsTheAgentsClaude(t *testing.T) {
	// herdr's server has a bare PATH: "claude" alone isn't found from it.
	s := claudeSession{Binary: "/home/me/.local/bin/claude"}
	if got := claudeBinary(withDefaults(config{}), s); got != s.Binary {
		t.Errorf("got %q", got)
	}
	if got := claudeBinary(withDefaults(config{Claude: "/opt/claude"}), s); got != "/opt/claude" {
		t.Errorf("config.json's claude: got %q", got)
	}
	if got := claudeBinary(withDefaults(config{}), claudeSession{}); got != "claude" {
		t.Errorf("nothing known: got %q", got)
	}
}

func TestTheAgentsClaudeIsFoundOnItsOwnPath(t *testing.T) {
	// Started as plain "claude": looked up on the agent's PATH, not herdr's.
	bin := t.TempDir()
	write(t, filepath.Join(bin, "claude"), "#!/bin/sh\n")
	os.Chmod(filepath.Join(bin, "claude"), 0o755)
	env := map[string]string{"PATH": "/nowhere:" + bin}
	if got := agentBinary("claude", env); got != filepath.Join(bin, "claude") {
		t.Errorf("got %q", got)
	}
	if got := agentBinary("/opt/x/claude", env); got != "/opt/x/claude" {
		t.Errorf("a path: got %q", got)
	}
	if got := agentBinary("claude", map[string]string{"PATH": "/nowhere"}); got != "" {
		t.Errorf("not on its PATH: got %q", got)
	}
	dir := claudeDir(t, "5", "/home/me/code/app", true)
	withEnv(t, map[string]string{"CLAUDE_CONFIG_DIR": dir, "PATH": bin}, nil)
	if s, err := sessionForPID(5, "claude"); err != nil || s.Binary != filepath.Join(bin, "claude") {
		t.Errorf("%+v %v", s, err)
	}
}
