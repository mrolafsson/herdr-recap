package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// sessionMeta is what a Claude transcript says about the session, for the
// line under an agent's title.
type sessionMeta struct {
	Branch     string    // the git branch it's working on
	Model      string    // e.g. claude-opus-5-5
	Context    int       // tokens in its context at the last reply
	Mode       string    // permission mode: default, plan, auto, acceptEdits…
	LastPrompt string    // what you last asked it
	Active     time.Time // its last message, either way
	PromptAt   time.Time // when you last asked it something: how long it's worked
	// Pending is the request it's waiting on you for, as a line to show:
	// the last tool call that has no result yet ("" when there's none).
	Pending   string
	PendingAt time.Time
	// Tasks it's tracking (its todo list): how many, and how many are done.
	TasksDone, TasksTotal int
}

// metaTail is how much of the end of a transcript is read: the latest of
// each record is all that's wanted, and transcripts run to megabytes.
const metaTail = 512 << 10

// block is one piece of a message's content.
type block struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
}

// readMeta reads the latest of each field from the end of a transcript.
func readMeta(path string) sessionMeta {
	var m sessionMeta
	if path == "" {
		return m
	}
	f, err := os.Open(path)
	if err != nil {
		return m
	}
	defer f.Close()
	if fi, err := f.Stat(); err == nil && fi.Size() > metaTail {
		_, _ = f.Seek(fi.Size()-metaTail, 0)
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), metaTail)
	type call struct {
		id, text string
		at       time.Time
	}
	var open []call // tool calls without a result yet, oldest first
	var tasks taskCount
	first := true
	for sc.Scan() {
		line := sc.Bytes()
		if first {
			first = false
			if len(line) == 0 || line[0] != '{' {
				continue // the tail starts inside a record
			}
		}
		if !bytes.Contains(line, []byte(`"type"`)) {
			continue
		}
		var r struct {
			Type       string `json:"type"`
			GitBranch  string `json:"gitBranch"`
			Timestamp  string `json:"timestamp"`
			Mode       string `json:"permissionMode"`
			LastPrompt string `json:"lastPrompt"`
			IsMeta     bool   `json:"isMeta"`
			Message    struct {
				Model   string          `json:"model"`
				Content json.RawMessage `json:"content"`
				Usage   struct {
					Input       int `json:"input_tokens"`
					CacheRead   int `json:"cache_read_input_tokens"`
					CacheCreate int `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &r) != nil {
			continue
		}
		at, _ := time.Parse(time.RFC3339Nano, r.Timestamp)
		if r.GitBranch != "" && r.GitBranch != "HEAD" {
			m.Branch = r.GitBranch
		}
		// Content is a string (a typed prompt) or a list of blocks.
		var blocks []block
		_ = json.Unmarshal(r.Message.Content, &blocks)
		switch r.Type {
		case "assistant":
			if r.Message.Model != "" && !strings.HasPrefix(r.Message.Model, "<") {
				m.Model = r.Message.Model
			}
			if n := r.Message.Usage.Input + r.Message.Usage.CacheRead + r.Message.Usage.CacheCreate; n > 0 {
				m.Context = n
			}
			for _, b := range blocks {
				if b.Type == "tool_use" {
					open = append(open, call{b.ID, pendingText(b.Name, b.Input), at})
					tasks.see(b.Name, b.Input)
				}
			}
		case "user":
			human := !r.IsMeta && len(r.Message.Content) > 0 && r.Message.Content[0] == '"'
			for _, b := range blocks {
				switch b.Type {
				case "tool_result":
					for i := range open {
						if open[i].id == b.ToolUseID {
							open = append(open[:i], open[i+1:]...)
							break
						}
					}
				case "text":
					human = human || !r.IsMeta
				}
			}
			if human && !at.IsZero() {
				m.PromptAt = at
			}
		case "permission-mode":
			m.Mode = r.Mode
		case "last-prompt":
			m.LastPrompt = r.LastPrompt
		}
		if (r.Type == "assistant" || (r.Type == "user" && !r.IsMeta)) && at.After(m.Active) {
			m.Active = at
		}
	}
	if n := len(open); n > 0 {
		m.Pending, m.PendingAt = open[n-1].text, open[n-1].at
	}
	m.TasksDone, m.TasksTotal = tasks.done(), tasks.total()
	sanitize(&m)
	m.LastPrompt = strings.Join(strings.Fields(m.LastPrompt), " ")
	m.Pending = strings.Join(strings.Fields(m.Pending), " ")
	return m
}

// pendingText says what a tool call waiting on you wants, briefly.
func pendingText(name string, raw json.RawMessage) string {
	var in map[string]any
	_ = json.Unmarshal(raw, &in)
	str := func(k string) string { s, _ := in[k].(string); return s }
	switch name {
	case "AskUserQuestion":
		if qs, ok := in["questions"].([]any); ok && len(qs) > 0 {
			if q, ok := qs[0].(map[string]any); ok {
				if s, _ := q["question"].(string); s != "" {
					return s
				}
			}
		}
		return "It has a question for you"
	case "ExitPlanMode":
		return "Approve its plan?"
	case "Bash", "PowerShell":
		return "Run " + str("command")
	case "Edit", "MultiEdit", "Write", "NotebookEdit":
		p := str("file_path")
		if p == "" {
			p = str("notebook_path")
		}
		verb := "Edit "
		if name == "Write" {
			verb = "Write "
		}
		return verb + filepath.Base(p)
	case "WebFetch":
		return "Fetch " + str("url")
	case "WebSearch":
		return "Search the web for " + str("query")
	}
	if rest, ok := strings.CutPrefix(name, "mcp__"); ok {
		server, tool, _ := strings.Cut(rest, "__")
		return "Use " + tool + " (" + server + ")"
	}
	return "Use " + name
}

// taskCount follows its todo list: TodoWrite writes the whole list each
// time; TaskCreate and TaskUpdate add tasks and change their status.
type taskCount struct {
	todos    []string          // TodoWrite's statuses, the latest list
	created  int               // TaskCreate calls
	statuses map[string]string // TaskUpdate's latest status by task
	usesTodo bool              // the latest to be used was TodoWrite
}

func (t *taskCount) see(name string, raw json.RawMessage) {
	switch name {
	case "TodoWrite":
		var in struct {
			Todos []struct {
				Status string `json:"status"`
			} `json:"todos"`
		}
		if json.Unmarshal(raw, &in) == nil {
			t.todos = t.todos[:0]
			for _, td := range in.Todos {
				t.todos = append(t.todos, td.Status)
			}
			t.usesTodo = true
		}
	case "TaskCreate":
		t.created++
		t.usesTodo = false
	case "TaskUpdate":
		var in struct {
			TaskID any    `json:"taskId"`
			Status string `json:"status"`
		}
		if json.Unmarshal(raw, &in) == nil && in.Status != "" {
			if t.statuses == nil {
				t.statuses = map[string]string{}
			}
			t.statuses[fmt.Sprint(in.TaskID)] = in.Status
		}
		t.usesTodo = false
	}
}

func (t taskCount) total() int {
	if t.usesTodo {
		return len(t.todos)
	}
	n := t.created
	for _, s := range t.statuses {
		if s == "deleted" {
			n--
		}
	}
	// Updates to tasks created before the part of the transcript read.
	return max(n, len(t.statuses)-t.deletedCount())
}

func (t taskCount) deletedCount() int {
	n := 0
	for _, s := range t.statuses {
		if s == "deleted" {
			n++
		}
	}
	return n
}

func (t taskCount) done() int {
	n := 0
	list := t.todos
	if !t.usesTodo {
		list = nil
		for _, s := range t.statuses {
			list = append(list, s)
		}
	}
	for _, s := range list {
		if s == "completed" {
			n++
		}
	}
	return n
}

var modelDate = regexp.MustCompile(`-\d{8}$`)

// shortModel names a model briefly: claude-opus-5-5 is "opus 5.5",
// claude-haiku-4-5-20251001 "haiku 4.5".
func shortModel(id string) string {
	id = strings.TrimSuffix(id, "[1m]")
	id = modelDate.ReplaceAllString(strings.TrimPrefix(id, "claude-"), "")
	name, version, ok := strings.Cut(id, "-")
	if !ok {
		return id
	}
	return name + " " + strings.ReplaceAll(version, "-", ".")
}

// shortTokens is a token count as 182k or 1.2M.
func shortTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1e6), ".0") + "M"
	case n >= 1000:
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprint(n)
}

// gitBranch reads the branch checked out in dir, or in the repo above it,
// without running git: .git is a directory, or for a worktree a file
// pointing at one, and its HEAD names the branch ("" when detached).
func gitBranch(dir string) string {
	for d := dir; d != "" && d != "/" && d != "."; d = filepath.Dir(d) {
		gitPath := filepath.Join(d, ".git")
		fi, err := os.Stat(gitPath)
		if err != nil {
			continue
		}
		if !fi.IsDir() {
			data, err := os.ReadFile(gitPath)
			if err != nil {
				return ""
			}
			target, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: ")
			if !ok {
				return ""
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(d, target)
			}
			gitPath = target
		}
		head, err := os.ReadFile(filepath.Join(gitPath, "HEAD"))
		if err != nil {
			return ""
		}
		ref, ok := strings.CutPrefix(strings.TrimSpace(string(head)), "ref: refs/heads/")
		if !ok {
			return ""
		}
		return clean(ref, false)
	}
	return ""
}
