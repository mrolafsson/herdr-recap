package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseStatus(t *testing.T) {
	out := "# branch.oid abc\n# branch.head main\n# branch.upstream origin/main\n# branch.ab +3 -1\n" +
		"1 .M N... 100644 100644 100644 a b tui.go\n" +
		"2 R. N... 100644 100644 100644 a b R100 new.go\told.go\n" +
		"u UU N... 1 2 3 4 a b c conflict.go\n" +
		"? notes.txt\n! ignored.txt\n"
	if c := parseStatus(out); c != (gitChanges{Files: 4, Ahead: 3, Behind: 1}) {
		t.Errorf("%+v", c)
	}
	if c := parseStatus(""); !c.empty() {
		t.Errorf("not a repo: %+v", c)
	}
}

func TestParseShortstat(t *testing.T) {
	for out, want := range map[string][2]int{
		" 3 files changed, 120 insertions(+), 30 deletions(-)\n": {120, 30},
		" 1 file changed, 1 insertion(+)\n":                      {1, 0},
		" 1 file changed, 4 deletions(-)\n":                      {0, 4},
		"":                                                       {0, 0},
	} {
		if a, d := parseShortstat(out); a != want[0] || d != want[1] {
			t.Errorf("%q: %d %d", out, a, d)
		}
	}
}

func TestReadChangesInARealRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	write(t, filepath.Join(dir, "a.txt"), "one\ntwo\n")
	run("add", ".")
	run("commit", "-qm", "first")
	write(t, filepath.Join(dir, "a.txt"), "one\nthree\nfour\n")
	write(t, filepath.Join(dir, "new.txt"), "x\n")
	if c := readChanges(dir); c != (gitChanges{Files: 2, Added: 2, Deleted: 1}) {
		t.Errorf("%+v", c)
	}
	if c := readChanges(t.TempDir()); !c.empty() {
		t.Errorf("not a repo: %+v", c)
	}
}
