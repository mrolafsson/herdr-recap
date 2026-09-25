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
}

// metaTail is how much of the end of a transcript is read: the latest of
// each record is all that's wanted, and transcripts run to megabytes.
const metaTail = 512 << 10

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
				Model string `json:"model"`
				Usage struct {
					Input       int `json:"input_tokens"`
					CacheRead   int `json:"cache_read_input_tokens"`
					CacheCreate int `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &r) != nil {
			continue
		}
		if r.GitBranch != "" && r.GitBranch != "HEAD" {
			m.Branch = r.GitBranch
		}
		switch r.Type {
		case "assistant":
			if r.Message.Model != "" && !strings.HasPrefix(r.Message.Model, "<") {
				m.Model = r.Message.Model
			}
			if n := r.Message.Usage.Input + r.Message.Usage.CacheRead + r.Message.Usage.CacheCreate; n > 0 {
				m.Context = n
			}
		case "permission-mode":
			m.Mode = r.Mode
		case "last-prompt":
			m.LastPrompt = r.LastPrompt
		}
		if (r.Type == "assistant" || (r.Type == "user" && !r.IsMeta)) && r.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, r.Timestamp); err == nil && t.After(m.Active) {
				m.Active = t
			}
		}
	}
	sanitize(&m)
	m.LastPrompt = strings.Join(strings.Fields(m.LastPrompt), " ")
	return m
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
