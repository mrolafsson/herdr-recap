package main

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// gitChanges is how an agent's worktree differs from what's committed and
// pushed: work waiting to be committed, or pushed.
type gitChanges struct {
	Files          int // changed or new files
	Added, Deleted int // lines, against HEAD
	Ahead, Behind  int // commits, against its upstream
}

func (c gitChanges) empty() bool { return c == gitChanges{} }

// readChanges asks git about dir. herdr's own PATH usually has git (in
// /usr/bin); without it, or outside a repo, there's nothing to show.
func readChanges(dir string) gitChanges {
	if dir == "" {
		return gitChanges{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	git := func(args ...string) string {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(cmd.Environ(), "GIT_OPTIONAL_LOCKS=0") // only reading: don't take the index lock
		out, _ := cmd.Output()
		return string(out)
	}
	c := parseStatus(git("status", "--porcelain=v2", "--branch", "--untracked-files=normal"))
	c.Added, c.Deleted = parseShortstat(git("diff", "--shortstat", "HEAD"))
	return c
}

// parseStatus reads `git status --porcelain=v2 --branch`: the ahead/behind
// header, and one line per changed or untracked file.
func parseStatus(out string) gitChanges {
	var c gitChanges
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.ab "):
			for _, f := range strings.Fields(strings.TrimPrefix(line, "# branch.ab ")) {
				n, _ := strconv.Atoi(f[1:])
				if f[0] == '+' {
					c.Ahead = n
				} else if f[0] == '-' {
					c.Behind = n
				}
			}
		case len(line) > 1 && strings.ContainsRune("12u?", rune(line[0])) && line[1] == ' ':
			c.Files++
		}
	}
	return c
}

var shortstatNumber = regexp.MustCompile(`(\d+) (insertion|deletion)`)

// parseShortstat reads `git diff --shortstat`: "3 files changed, 120
// insertions(+), 30 deletions(-)", either count left out when it's none.
func parseShortstat(out string) (added, deleted int) {
	for _, m := range shortstatNumber.FindAllStringSubmatch(out, -1) {
		n, _ := strconv.Atoi(m[1])
		if m[2] == "insertion" {
			added = n
		} else {
			deleted = n
		}
	}
	return added, deleted
}
