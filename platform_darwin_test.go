package main

import "testing"

func TestEnvFromPS(t *testing.T) {
	line := "/Users/me/.local/bin/claude --resume x TERM=xterm CLAUDE_CONFIG_DIR=/Users/me/.clauth/p PATH=/Users/me/.local/bin:/usr/bin HOME=/Users/me\n"
	env := envFromPS(line, "CLAUDE_CONFIG_DIR", "PATH", "MISSING")
	if env["CLAUDE_CONFIG_DIR"] != "/Users/me/.clauth/p" || env["PATH"] != "/Users/me/.local/bin:/usr/bin" {
		t.Errorf("%v", env)
	}
	if _, ok := env["MISSING"]; ok {
		t.Error("a variable that isn't there")
	}
}
