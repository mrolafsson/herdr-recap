package main

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// readProcessEnv asks ps for a process's environment, which macOS shows for
// your own processes after its command line (`ps -E`). There is no separator
// between the two, so only the variables this plugin needs are looked for,
// each running to the next " NAME=". A value with a space followed by
// something that looks like another variable would be cut short there.
func readProcessEnv(pid int) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-E", "-ww", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil, err
	}
	return envFromPS(string(out), "CLAUDE_CONFIG_DIR", "PATH"), nil
}

func envFromPS(line string, names ...string) map[string]string {
	env := map[string]string{}
	line = strings.TrimRight(line, "\n")
	for _, name := range names {
		i := strings.Index(line, " "+name+"=")
		if i < 0 {
			continue
		}
		v := line[i+len(name)+2:]
		// The value runs to the next " NAME=" (an upper-case word and =).
		for j := 0; j < len(v); j++ {
			if v[j] == ' ' && nextIsVar(v[j+1:]) {
				v = v[:j]
				break
			}
		}
		env[name] = v
	}
	return env
}

func nextIsVar(s string) bool {
	k, _, ok := strings.Cut(s, "=")
	if !ok || k == "" || strings.ContainsAny(k, " /") {
		return false
	}
	for _, r := range k {
		if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
