# Changelog

## 0.1.0 — 2026-09-25

- A popup with every agent in herdr, in the sidebar's status glyphs and
  colours, what needs you first and within a status what has waited longest.
  Enter or a click goes to the agent.
- For Claude agents, a recap of where each got to (Claude Code's own
  `/recap`), written ahead of time: an agent that finishes, or stops to ask
  you something, and that you don't look at within `recap_after_seconds`
  (3 minutes) gets its recap then, while its prompt cache is still warm.
  Opening the popup rewrites any that are out of date.
- What a blocked agent is waiting for, in red: its question, or the
  command or edit it wants approved.
- Under each title: git branch, uncommitted and unpushed work (*6 files
  +214 −58 ↑2*), task progress (*4/7 tasks*) and the pane's tokens (a PR
  badge from herdr-github…; `tokens` in config.json picks which). The
  selected agent also shows your last prompt, its model and its permission
  mode. Right of the title, how long it's been in its state (*waiting 3m*).
- `r` replies to the selected agent without leaving the popup, on as many
  lines as you like; `ctrl+r` writes its recap again.
- In herdr's theme colours: the title in its brightest text, each detail in
  its own colour. Glyphs are ones common monospace fonts have.
- Sessions are found through the agent's own process, so profile switchers
  that set `CLAUDE_CONFIG_DIR` (clauth and the like) work, and recaps run the
  agent's own `claude` with its PATH: herdr's server often has a bare PATH.
- `herdr-recap list` shows what it reads from each agent; a demo on
  fictional agents, and `scripts/screenshots.sh` for the README's images.
