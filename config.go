package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	// RecapAfterSeconds is how long an agent has to sit finished (done) or
	// waiting on you (blocked) without you looking before its recap is
	// written. Short glances away cost nothing. Past the prompt cache's
	// lifetime (5 minutes by default) each recap costs several times more.
	RecapAfterSeconds int `json:"recap_after_seconds"`
	// Claude is the claude binary recaps run with. Empty = the one the agent
	// runs, or "claude" on PATH.
	Claude string `json:"claude"`
	// TimeoutSeconds bounds one recap.
	TimeoutSeconds int `json:"timeout_seconds"`
	// Theme: "dark", "light", or empty to ask the terminal.
	Theme string `json:"theme"`
}

func pluginID() string {
	if id := os.Getenv("HERDR_PLUGIN_ID"); id != "" {
		return id
	}
	return "herdr-recap"
}

func xdgDir(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, fallback)
}

func configDir() string {
	if d := os.Getenv("HERDR_PLUGIN_CONFIG_DIR"); d != "" {
		return d
	}
	return filepath.Join(xdgDir("XDG_CONFIG_HOME", ".config"), "herdr", "plugins", "config", pluginID())
}

func stateDir() string {
	if d := os.Getenv("HERDR_PLUGIN_STATE_DIR"); d != "" {
		return d
	}
	return filepath.Join(xdgDir("XDG_STATE_HOME", ".local/state"), "herdr", "plugins", pluginID())
}

// loadConfig reads config.json. On error it still returns the defaults.
func loadConfig() (config, error) {
	cfg, err := readConfig()
	if err != nil {
		cfg = config{}
	}
	return withDefaults(cfg), err
}

func readConfig() (config, error) {
	cfg := config{}
	data, err := os.ReadFile(filepath.Join(configDir(), "config.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, errors.New("config.json: " + err.Error())
		}
	}
	return cfg, nil
}

func withDefaults(cfg config) config {
	if cfg.RecapAfterSeconds <= 0 {
		cfg.RecapAfterSeconds = 180
	}
	cfg.Claude = expandHome(cfg.Claude)
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 120
	}
	return cfg
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}
