# herdr-recap

Every agent in [herdr](https://herdr.dev) in one popup: its live status, its
title, and for Claude agents a line or two on where it got to and what's
next. Come back to your desk, open it, and you know what each agent was doing
before you pick one. Enter or a click goes to it.

- **Live status** in herdr's sidebar glyphs and colours: `◉` needs you, a
  spinner while working, `●` done, `✓` idle. What needs you comes first.
- **Titles** are the agents' own: Claude's title for the conversation,
  bold, with how long it's been in its state: *waiting 3m*, *working 12m*,
  *done 25m*. Within a status, what has waited longest comes first.
- **What a blocked agent is waiting for**: its question, or the command or
  edit it wants approved.
- **Details** under each title: git branch, uncommitted and unpushed work
  (*4 files +120 −30 ↑1*), task progress (*3/7 tasks*), and the pane's
  tokens (a PR badge from herdr-github, a profile…). The selected agent also
  shows your last prompt to it, its model and (when it isn't the default)
  its permission mode.
- **Reply without leaving**: `r` sends the selected agent a prompt.
- **Recaps** are Claude Code's own `/recap`, the summary it shows when you
  come back to a session, for every session at once.
- **Written while you're away**, so opening the popup is instant (see
  [When recaps are written](#when-recaps-are-written)).
- In **herdr's theme colours**, whichever theme you've picked there.
- Keyboard first, and the mouse works: hover, click, scroll.

**Contents:** [Requirements](#requirements) · [Install](#install) ·
[Use](#use) · [When recaps are written](#when-recaps-are-written) ·
[What it costs](#what-it-costs) · [Configuration](#configuration) ·
[Privacy](#privacy) · [Troubleshooting](#troubleshooting) ·
[How it works](#how-it-works) · [Development](#development)

## Requirements

- **herdr 0.9.0** or later.
- **Claude Code** with `/recap` (2.1.x), signed in. Recaps are Claude-only;
  other agents are listed with their status and title.
- **Linux** or **macOS**, on arm64 or x86-64.

## Install

```sh
herdr plugin install mrolafsson/herdr-recap
```

The build step compiles it with Go if you have it, or downloads the release
binary for your platform and checks its SHA-256.

Bind it to a key in `~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "prefix+i"
type = "plugin_action"
command = "herdr-recap.open"
```

`Recap: demo (fictional agents)` in the command palette shows it on made-up
agents: no Claude calls, safe to screenshot.

## Use

| Key | Does |
| --- | --- |
| `↑` `↓`, `k` `j`, `ctrl+p` `ctrl+n` | move |
| `pgup` / `pgdn`, `g` / `G` | page, first / last |
| `enter`, click | go to that agent and close |
| `r` | reply to that agent: type, then `enter` to send (`esc` drops it). `alt+enter` or `ctrl+j` starts a new line, and pasted text keeps its lines. An agent waiting on a question or approval can't take one: go to it to answer |
| `ctrl+r` | write that agent's recap again, now |
| `esc`, `q` | close |

A recap written before the agent carried on says how old it is (`from 20m
ago`) and is drawn dimmer until it's been rewritten, which opening the popup
does straight away.

## When recaps are written

Soon after a turn ends. When a Claude agent finishes (done) or stops to ask
you something (blocked), herdr tells the plugin, which waits
`recap_after_seconds` (3 minutes). If by then you still haven't looked at the
agent, its recap is written; if you have (done turns to idle) or you answered
it, nothing is spent. So the agents you're working with cost nothing, and the
ones you walked away from are recapped by the time you're back.

Opening the popup also rewrites any recap that no longer matches its
conversation, for example an agent that has carried on working, a few at a
time, in the background. The rows show what's already there meanwhile.

## What it costs

A recap is one short Claude request on the agent's own conversation, billed
like any other (or counted against your plan). Written soon after the turn,
it reuses the agent's prompt cache: in testing, about 2¢ at list price on a
~70k-token conversation, and a few cents on bigger ones. Once the cache has
expired (5 minutes by default, an hour on some plans) the whole conversation
is read again, which costs several times more. That's why the wait is 3
minutes: keep `recap_after_seconds` under your cache's lifetime.

At most one recap per agent each time you walk away from it: an agent
sitting in done starts no new turns.

## Configuration

Optional: `config.json` in the plugin's config directory
(`~/.config/herdr/plugins/config/herdr-recap/`). See
[config.example.json](config.example.json).

| Key | Default | Meaning |
| --- | --- | --- |
| `recap_after_seconds` | `180` | How long an agent waits on you, unlooked at, before its recap is written |
| `claude` | the agent's own | The Claude Code binary to run recaps with. By default the one the agent runs, found on the agent's PATH (herdr's own PATH often lacks `~/.local/bin`) |
| `timeout_seconds` | `120` | The longest one recap may take |
| `theme` | ask the terminal | `"dark"` or `"light"` background |
| `tokens` | all | Which pane tokens to show under a title, in order, e.g. `["pr", "clauth"]`. By default all, less herdr-github's `pr_*` details when its `pr` is there |

## Privacy

Recaps are made by the same Claude Code, account and settings the agent uses,
on its own conversation, so nothing goes anywhere it hadn't already. They're
kept in `~/.local/state/herdr/plugins/herdr-recap/recaps/`, one small file
per session, readable only by you.

The recap is a fork of the conversation that is never saved: the agent's own
session isn't written to, and it doesn't show up in `claude --resume`. It
does run with your Claude Code settings, so your own hooks (a notification on
stop, say) run for it too.

## Troubleshooting

- **`herdr-recap list`** shows, without drawing the popup, each agent, the
  Claude session it was found to be running, and its cached recap.
- **`herdr-recap recap w1:p2`** writes one pane's recap now and prints it,
  with any error in full.
- **`recap.log`** in the state directory has every recap written in the
  background, with its cost, and any errors.
- *Couldn't find its conversation*: the plugin reads Claude's process to find
  its session (its `CLAUDE_CONFIG_DIR` and `sessions/<pid>.json`). A Claude
  Code too old to write that file, or one running as another user, can't be
  read.

## How it works

The popup reads herdr's agent list every second over its socket, so statuses
stay live. For each Claude agent it finds the session through the pane's
`claude` process rather than herdr's reported session ID, which under a
profile switcher (clauth and the like) can name a different session: the
process's `CLAUDE_CONFIG_DIR` (from `/proc` on Linux, `ps` on macOS) leads
to `sessions/<pid>.json`, which names the session and where it was started.

A recap runs

```sh
claude -p --resume <session> --fork-session --no-session-persistence --output-format json /recap
```

in the session's directory with the agent's `CLAUDE_CONFIG_DIR` and without
herdr's environment (so herdr's own Claude hooks don't mistake it for an
agent). It's cached with the transcript's size and time, and counts as
current until the transcript changes. One recap per session runs at a time:
another asking for the same one waits and takes its result.

The `pane.agent_status_changed` hook (`tick`) only reads the agent list and
starts a detached `recap --after` for an agent now waiting on you, once per
status change. That process sleeps, checks the agent's status hasn't moved
since, and writes the recap.

## Development

```sh
go test -race ./...
go run . picker --demo       # the popup on fictional agents, in any terminal
herdr plugin link .          # try it in herdr
```

`themes_herdr.go` is generated from herdr's palettes by
`scripts/gen-themes.py` (see the script for how).
