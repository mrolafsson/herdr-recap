package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadMetaTakesTheLatestOfEach(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	write(t, path, strings.Join([]string{
		`{"type":"permission-mode","permissionMode":"plan"}`,
		`{"type":"user","gitBranch":"old-branch","timestamp":"2026-09-25T08:00:00Z","message":{"content":"hi"}}`,
		`{"type":"assistant","gitBranch":"feature/x","timestamp":"2026-09-25T08:01:00Z","message":{"model":"claude-opus-5-5","usage":{"input_tokens":10,"cache_read_input_tokens":150000,"cache_creation_input_tokens":2000}}}`,
		`{"type":"last-prompt","lastPrompt":"  fix the\nlogin bug  "}`,
		`{"type":"permission-mode","permissionMode":"auto"}`,
		`{"type":"assistant","gitBranch":"HEAD","message":{"model":"<synthetic>"}}`,
		`{"type":"user","isMeta":true,"timestamp":"2026-09-25T09:00:00Z"}`,
		`not json`,
	}, "\n")+"\n")
	m := readMeta(path)
	if m.Branch != "feature/x" || m.Model != "claude-opus-5-5" || m.Context != 152010 || m.Mode != "auto" || m.LastPrompt != "fix the login bug" {
		t.Errorf("%+v", m)
	}
	if want := time.Date(2026, 9, 25, 8, 1, 0, 0, time.UTC); !m.Active.Equal(want) {
		t.Errorf("active %v: a meta record isn't activity", m.Active)
	}
	if (readMeta("") != sessionMeta{}) || (readMeta("/nowhere") != sessionMeta{}) {
		t.Error("no transcript, no meta")
	}
}

func TestReadMetaReadsOnlyTheTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	f, _ := os.Create(path)
	f.WriteString(`{"type":"assistant","gitBranch":"ancient","message":{"model":"claude-old-1"}}` + "\n")
	f.WriteString(strings.Repeat(`{"type":"attachment","x":"`+strings.Repeat("y", 1000)+`"}`+"\n", 600))
	f.WriteString(`{"type":"assistant","gitBranch":"recent","message":{"model":"claude-sonnet-5"}}` + "\n")
	f.Close()
	if m := readMeta(path); m.Branch != "recent" || m.Model != "claude-sonnet-5" {
		t.Errorf("%+v", m)
	}
}

func TestReadMetaCleansEscapes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	write(t, path, `{"type":"last-prompt","lastPrompt":"hi \u001b]52;c;aGk=\u0007there"}`+"\n")
	if m := readMeta(path); m.LastPrompt != "hi there" {
		t.Errorf("%q", m.LastPrompt)
	}
}

func TestShortModel(t *testing.T) {
	for in, want := range map[string]string{
		"claude-opus-5-5": "opus 5.5", "claude-sonnet-5": "sonnet 5", "claude-haiku-4-5-20251001": "haiku 4.5",
		"claude-fable-5-1": "fable 5.1", "claude-opus-5-5[1m]": "opus 5.5", "gpt-x": "gpt x",
	} {
		if got := shortModel(in); got != want {
			t.Errorf("%s: %q", in, got)
		}
	}
}

func TestShortTokens(t *testing.T) {
	for n, want := range map[int]string{999: "999", 182_400: "182k", 1_000_000: "1M", 1_240_000: "1.2M"} {
		if got := shortTokens(n); got != want {
			t.Errorf("%d: %q", n, got)
		}
	}
}

func TestGitBranch(t *testing.T) {
	repo := t.TempDir()
	write(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/feature/login\n")
	os.MkdirAll(filepath.Join(repo, "src", "deep"), 0o700)
	if got := gitBranch(filepath.Join(repo, "src", "deep")); got != "feature/login" {
		t.Errorf("from a subfolder: %q", got)
	}
	// A worktree: .git is a file naming the real git dir.
	wt, gd := t.TempDir(), t.TempDir()
	write(t, filepath.Join(gd, "HEAD"), "ref: refs/heads/act-1638\n")
	write(t, filepath.Join(wt, ".git"), "gitdir: "+gd+"\n")
	if got := gitBranch(wt); got != "act-1638" {
		t.Errorf("worktree: %q", got)
	}
	write(t, filepath.Join(repo, ".git", "HEAD"), "4f2a9c1e\n")
	if got := gitBranch(repo); got != "" {
		t.Errorf("detached: %q", got)
	}
	if got := gitBranch(t.TempDir()); got != "" {
		t.Errorf("no repo: %q", got)
	}
}

func TestPendingRequestAndPromptTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	write(t, path, strings.Join([]string{
		`{"type":"user","timestamp":"2026-09-25T08:00:00Z","message":{"content":"push it"}}`,
		`{"type":"assistant","timestamp":"2026-09-25T08:00:05Z","message":{"content":[{"type":"tool_use","id":"a","name":"Bash","input":{"command":"git status"}}]}}`,
		`{"type":"user","timestamp":"2026-09-25T08:00:06Z","message":{"content":[{"type":"tool_result","tool_use_id":"a"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-25T08:00:09Z","message":{"content":[{"type":"tool_use","id":"b","name":"Bash","input":{"command":"git push\norigin main"}}]}}`,
	}, "\n")+"\n")
	m := readMeta(path)
	if m.Pending != "Run git push origin main" {
		t.Errorf("pending %q", m.Pending)
	}
	if !m.PendingAt.Equal(time.Date(2026, 9, 25, 8, 0, 9, 0, time.UTC)) || !m.PromptAt.Equal(time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)) {
		t.Errorf("pending at %v, prompt at %v: a tool result isn't a prompt", m.PendingAt, m.PromptAt)
	}
	// Answered: nothing pending.
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"b"}]}}` + "\n")
	f.Close()
	if m := readMeta(path); m.Pending != "" {
		t.Errorf("answered, still pending %q", m.Pending)
	}
}

func TestPendingText(t *testing.T) {
	for _, c := range []struct{ name, input, want string }{
		{"AskUserQuestion", `{"questions":[{"question":"Public or private?"}]}`, "Public or private?"},
		{"ExitPlanMode", `{}`, "Approve its plan?"},
		{"Bash", `{"command":"rm -rf dist"}`, "Run rm -rf dist"},
		{"Edit", `{"file_path":"/a/b/tui.go"}`, "Edit tui.go"},
		{"Write", `{"file_path":"/a/README.md"}`, "Write README.md"},
		{"WebFetch", `{"url":"https://x.dev"}`, "Fetch https://x.dev"},
		{"mcp__linear__save_issue", `{}`, "Use save_issue (linear)"},
		{"Frobnicate", `{}`, "Use Frobnicate"},
	} {
		if got := pendingText(c.name, []byte(c.input)); got != c.want {
			t.Errorf("%s: %q", c.name, got)
		}
	}
}

func TestTasks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	use := func(name, input string) string {
		return `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"x","name":"` + name + `","input":` + input + `}]}}`
	}
	// TodoWrite: the latest list counts.
	write(t, path, strings.Join([]string{
		use("TodoWrite", `{"todos":[{"status":"completed"},{"status":"pending"}]}`),
		use("TodoWrite", `{"todos":[{"status":"completed"},{"status":"completed"},{"status":"in_progress"}]}`),
	}, "\n")+"\n")
	if m := readMeta(path); m.TasksDone != 2 || m.TasksTotal != 3 {
		t.Errorf("todos %d/%d", m.TasksDone, m.TasksTotal)
	}
	// TaskCreate and TaskUpdate: created, less deleted; done by latest status.
	write(t, path, strings.Join([]string{
		use("TaskCreate", `{"subject":"a"}`), use("TaskCreate", `{"subject":"b"}`),
		use("TaskCreate", `{"subject":"c"}`), use("TaskCreate", `{"subject":"d"}`),
		use("TaskUpdate", `{"taskId":"1","status":"in_progress"}`),
		use("TaskUpdate", `{"taskId":"1","status":"completed"}`),
		use("TaskUpdate", `{"taskId":2,"status":"completed"}`),
		use("TaskUpdate", `{"taskId":"4","status":"deleted"}`),
	}, "\n")+"\n")
	if m := readMeta(path); m.TasksDone != 2 || m.TasksTotal != 3 {
		t.Errorf("tasks %d/%d", m.TasksDone, m.TasksTotal)
	}
}
