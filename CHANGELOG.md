# Changelog

## Unreleased

- What a blocked agent is waiting for, in red under its title: its
  question, or the command or edit it wants approved.
- The details line shows uncommitted and unpushed work (*4 files +120 −30
  ↑1*) and task progress (*3/7 tasks*). Model, context and mode moved to
  the selected row, with your last prompt.
- Right of the title, how long the agent has been in its state (*waiting
  3m*, *done 25m*), and within a status what has waited longest is first.
- `p` replies to the selected agent without leaving the popup. Replies can
  run to several lines: `alt+enter` or `ctrl+j` for a new line, or paste.
- `herdr-recap list` shows the metadata it reads, for checking.

- A details line under each agent: git branch, model, context size,
  permission mode and the pane's tokens (`tokens` in config.json picks
  which). The selected agent also shows your last prompt to it.
- Titles are bold in the theme's brightest text, recaps a step below. Each
  detail has its own theme colour: branch mauve, model blue, context teal,
  mode yellow, tokens green. The right side says when the agent was last active.
- Recaps run the agent's own `claude`, found on the agent's PATH: herdr's
  server often has a bare PATH without `~/.local/bin`.

## 0.1.0 — 2026-09-25

- First release. A popup with every agent in herdr: its status, in the
  sidebar's glyphs and colours, its title, and for Claude agents a recap of
  where it got to (Claude Code's own `/recap`). Enter or a click goes to it.
- Recaps are written ahead of time: a Claude agent that finishes, or stops to
  ask you something, and that you don't look at within
  `recap_after_seconds` (3 minutes) gets its recap then, while its prompt
  cache is still warm. Opening the popup rewrites any that are out of date.
- Sessions are found through the agent's own process, so profile switchers
  that set `CLAUDE_CONFIG_DIR` (clauth and the like) work.
- In herdr's theme colours, and a demo on fictional agents.
