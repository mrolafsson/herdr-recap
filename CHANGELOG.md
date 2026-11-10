# Changelog

## Unreleased

- **GitHub Copilot CLI agents are recapped.** Read from the session's own
  files (`~/.copilot/session-state/<id>/`, or `$COPILOT_HOME`) with no model
  call: the session's name, the last thing asked, and the last reply. On by
  default, as Codex and omp are.

## 0.12.11 - 2026-11-04

- **omp agents are recapped,** by
  [@rstacruz](https://github.com/rstacruz)
  ([#4](https://github.com/mrolafsson/herdr-recap/issues/4)). With herdr's
  omp integration (`herdr integration install omp`), an Oh-My-Pi pane
  recaps itself in a fork of its own session, run with no tools, extensions
  or MCP, and removed once the recap is taken. On by default, as Codex is;
  in the herdr TUI.
- **Recaps and suggested titles in your language,** by
  [@alwinlin23](https://github.com/alwinlin23)
  ([#7](https://github.com/mrolafsson/herdr-recap/pull/7)).
  `"language": "zh-CN"` (or `"Japanese"`, `"简体中文"`) in `config.json`
  asks Claude, Codex, omp and the Memex summarizer in it. Recaps and
  suggestions in another language are written again; titles you took or
  turned down are kept. Unset, nothing changes.
- **A tool call in DeepSeek's spelling is refused too.** Its bars are
  full-width (`｜DSML`), which 0.12.10's refusal missed.

## 0.12.10 - 2026-11-03

- **Recaps and titles don't carry on with the work.** A session can stop
  mid-step, and the fork asked for its recap or title could take the
  question as the next thing to do. The Codex recap and the title now tell
  it not to, and an answer that writes out a tool call is refused rather
  than shown, and asked for again later
  ([#5](https://github.com/mrolafsson/herdr-recap/issues/5)).

## 0.12.9 - 2026-11-03

- **Recaps are written on a fresh app-mod install.** With only the Herdr
  app's mod installed, and never the TUI plugin, the folder recaps run from
  didn't exist, so none were ever written
  ([#6](https://github.com/mrolafsson/herdr-recap/issues/6)).

## 0.12.8 - 2026-11-01

- **Teleport's demo, from Recap's.** In the demo, `ctrl+t` opens
  [herdr-teleport](https://github.com/mrolafsson/herdr-teleport)'s demo on
  made-up machines, and touches nothing; the key shows in the demo even
  with one machine connected.
- **Teleport opens wider,** so what each of its steps did fits on its line.
- **herdr-teleport is out,** and listed with the plugins Recap works with.

## 0.12.7 - 2026-11-01

- **Linear issues open in the Linear mod.** With
  [herdr-linear](https://github.com/mrolafsson/herdr-linear)'s mod in the
  Herdr app, a click on an agent's issue, or `ctrl+l`, opens it there on its
  screen: change its status, open its worktree or start it. `esc` comes
  back to Recap. In the demo too.

## 0.12.6 - 2026-11-01

- **Pull requests open in the GitHub mod.** With
  [herdr-github](https://github.com/mrolafsson/herdr-github)'s mod in the
  Herdr app, a click on an agent's pull request, or `ctrl+g`, opens it
  there on its screen, to merge it or open its worktree; `esc` comes back
  to Recap. Without it, the browser as before. Pull requests come from the
  herdr-github the GitHub mod fetched, when there's no TUI install.

- **Teleport an agent from the list.** With
  [herdr-teleport](https://github.com/mrolafsson/herdr-teleport)'s mod in
  the Herdr app, `ctrl+t` (or *teleport* in the footer) moves the chosen
  agent to another machine: its conversation, branch and uncommitted work
  go with it. `esc` in Teleport comes back to Recap.

## 0.12.5 - 2026-11-01

- **Renaming an agent yourself is a click,** on the chosen card's title or
  *rename* in the footer, with no key: the app's keys stay to ctrl and the
  ones the TUI has.

## 0.12.4 - 2026-11-01

- **Recaps and titles asked for from the Herdr app are written.** On Linux
  the app runs commands through herdr-core, a systemd service that ends
  whatever a command leaves running when it returns, so the background copy
  that writes them was ended at once. Where systemd-run is, that copy is now
  a unit of its own.
- **A rename shows at once.** The card takes the new title straight away
  (Claude Code updates the terminal's own at its next turn), and the
  agent's space takes it too when nobody named it and the agent is alone
  in it, as in the TUI.
- **⌘E renames, not ctrl+e,** which the filter took to move to the end of
  the line; so does a click on the chosen card's title.
- **The machine on each card is easier to read.**

## 0.12.3 - 2026-11-01

- **The chosen card's mark sits inside it,** set in from its edge as far as
  from its top and bottom.
- **The app mod's recipe tells the agent installing it what to do,** and
  not to install herdr-recap itself: the mod fetches it onto each machine.

## 0.12.2 - 2026-11-01

- **Titles on demand in the app, and your own.** As in the TUI, `ctrl+r` on
  an agent with no suggested title asks for one there and then (the card
  says so until it comes), for you to take or ignore. `ctrl+e` renames an
  agent to a title you type: its own, to edit, and `/rename` once it's
  between turns. `herdr-recap title` is how the app asks, in the background.

## 0.12.1 - 2026-11-01

- **The app's Recap always opens on your own agents.** Its demo lasted
  past the popup it was switched on in, so an install could open on made-up
  agents. Now it's for that opening only, and the header says *Demo ·
  made-up agents* while it's on.

## 0.12.0 - 2026-11-01

- **Recap in the Herdr app.** The app's Recap mod now lives here, in
  `app/`: every agent on every machine the app is connected to, in a popup
  designed for the app (drawn icons, colour only on state, cards that lift),
  with the same recaps, freshness and titles as the TUI's. It needs nothing
  else installed: the first time it looks at a machine with agents, it
  fetches this release of herdr-recap for that machine, checks it against
  the release's checksums, and keeps it apart from a TUI install, sharing
  its recaps. Install it by asking your Herdr agent to add the mod at
  github.com/mrolafsson/herdr-recap/tree/main/app, or with `install.sh
  --app`.
- **One installer for both.** `install.sh` installs the TUI plugin, the app
  mod, or both, whichever you have; it's one `curl … | sh` line.

## 0.11.0 - 2026-11-01

- **Recaps for the Herdr app.** `herdr-recap sessions` gives the session and
  recap of Claude agents named by their process rather than a herdr pane,
  and with `--write` writes the out-of-date ones in the background. The
  Herdr app runs its agents in herdr-core, which herdr doesn't see, and
  the app's Recap mod (`app/`, from 0.12.0) uses this to show
  their recaps there. Recaps are shared with this plugin's.

## 0.10.1 - 2026-10-21

- **A rename shows in herdr straight away.** Where a plugin gives herdr the
  agent's title from Claude's transcript (such as claude-session-title),
  herdr went on showing the old one after a rename until the agent's next
  turn, so a busy agent renamed when it finished looked as if it hadn't
  been. The new title is now shown as soon as it's given.

## 0.10.0 - 2026-10-20

- **How the machine is doing, beside the tabs.** With
  [herdr-vitals](https://github.com/mrolafsson/herdr-vitals) installed, the
  space right of the tabs has a small graph of the machine's CPU since the
  popup opened and a gauge of its memory, as there's room for them. A click
  opens herdr-vitals' popup; `esc` there comes back here.

## 0.9.0 - 2026-10-17

- **A rename takes the agent's space along.** When the agent is the only
  one in its space and the space is named for its folder, repo, branch or
  the agent's old title, *Rename* renames the space too, as `prefix+W`
  would, and the suggestion says *(and its space)*. A space you named
  yourself, or one shared with other agents, keeps its name.

## 0.8.1 - 2026-10-13

- **herdr-file-transfer joins the plugins that work together.** The README's
  table has it: files between the machine you're at and the server, and the
  files a new Linear issue attaches.

## 0.8.0 - 2026-10-13

- **What just woke.** An agent that herdr-snooze woke by itself (its time
  was up, or it was done) says so on its row, `woke 16:32 · it's done`,
  and has a tab of its own, *⏰ Just woke*, beside Snoozed, until you've
  been to it. In All it stays under its stage (Done, Needs you…). When the
  tabs don't fit even shortened, they close up to a space apart.

## 0.7.1 - 2026-10-13

- **A snooze until something happens says so**: an agent herdr-snooze has
  hidden till it's done, or till its pull request is merged, reads *till
  done* or *till merged* on its row, not a time.

## 0.7.0 - 2026-10-12

- **Snoozed agents have a stage of their own.** An agent that
  [herdr-snooze](https://github.com/mrolafsson/herdr-snooze) has hidden
  from herdr's Agents panel is listed under *Snoozed*, the last tab,
  whatever it's doing: you've said it can wait. The right of its row says
  when it wakes (*until 17:30*), in snooze's colour, and its tokens aren't
  listed under its title. The filter finds them by `snoozed`.
- **`ctrl+s` snoozes the selected agent**, in herdr-snooze's popup (a
  snoozed one, for a new time). `esc` there, or snoozing it, comes back to
  Recap on the same agent.
- **`ctrl+w` wakes a snoozed agent**, there and then, with no popup. On any
  other agent it's still the filter's: it deletes a word. Both keys are only
  there with herdr-snooze installed and enabled.
- **The demo has a snoozed agent**, and the screenshots its tab.

## 0.6.1 - 2026-10-12

- **"Recap" is no longer written before the tabs**: the popup's border
  says it already.

## 0.6.0 - 2026-10-12

Recaps for other agents. `agents` in `config.json` lists the ones recapped,
`["claude", "codex"]` unless you say otherwise.

- **Codex agents are recapped, with nothing to set up.** If you run Codex
  in herdr, its agents now get recaps as Claude's do, each one a short
  request against your Codex plan or key. `"agents": ["claude"]` turns that
  off.
- **Codex recaps itself**, as Claude does: a fork of the pane's session that
  is never saved (`codex exec fork --ephemeral`, read-only), so the
  conversation goes nowhere it hadn't been. The session is found from the
  pane's Codex process; herdr's Codex integration isn't needed.
- **OpenCode and Hermes, through Memex, when you add them to `agents`**, by
  [@saforem2](https://github.com/saforem2) (#1): Memex reads the end of the
  session, and the user and assistant text is summarized by Claude Code
  (`haiku` by default, or your own `summarizer` command). Sessions are
  matched by the exact ID herdr reports, and the transcript is passed as
  data, with no tools, so it can't instruct the summarizer.
- **A long conversation read through Memex is recapped from its end.** It
  was read forward from 500 records back, for at most 20 pages, which on an
  agent that had run many tools stopped short of where it had got to. It's
  now read back from the end until there's enough of what was said. Found
  on the first run against a real Memex (0.27.2), which the lookup, the
  read and the recap have now had.
- **Better titles for Codex agents too**: asked for in a fork of its own,
  and given with Codex's `/rename`. Not for agents read through Memex.
- **When no row matches the filter, the agents' conversations are searched
  for it.** The ones where it came up are listed, each with what was said
  there: for when you remember what an agent said but not which agent. It
  searches what you and the agent said, in Claude's and Codex's transcripts
  on your disk (and the recent page Memex gives for the others), not what
  tools read or printed. Try it in the demo with `playwright`. With Memex
  installed, they're listed best match first, by its search's score.

## 0.5.4 - 2026-10-12

- **An agent is found by a pull request it's done with.** Type a merged
  PR's number and the agent that opened it is listed, even once it has
  moved on to another branch. herdr-github 0.8.0 reports those PRs in a
  token of their own (`prs_past`), which the filter searches and the row
  doesn't show; name it in `tokens` in `config.json` if you want it shown.

## 0.5.3 - 2026-10-12

- **The selection stays on the top row until you pick one.** It used to
  follow whichever agent was first when the popup opened, and drift down
  the list as statuses came in and the order changed. Once you move, point
  at or act on a row, it follows that agent as before.

## 0.5.2 - 2026-10-11

- **Pull requests and Linear issues open in their plugins.** With
  herdr-github installed, an agent's pull request is underlined: a click on
  it, or `ctrl+g`, opens it in the GitHub popup. With herdr-linear, the same
  for its Linear issue (`ctrl+l`), which is now shown as one piece with its
  status (`ENG-142 In Progress`). `esc` there comes back to Recap, on the
  agent you left. The plugins are found by themselves; without them nothing
  changes.

## 0.5.1 - 2026-10-11

- **A recap runs the width of the popup**, where it stopped at 80 columns:
  on a wide terminal it's a line or two, and more agents fit. It stops short
  of the right edge by a margin that grows with the popup, 3 cells on a
  narrow one and up to 8 on a wide one.

## 0.5.0 - 2026-10-10

- **Better titles, suggested with the recap.** An agent with no title, a slug
  for one (`fix-missing-share-notify`) or one that leaves out its
  branch's ticket has a better one on the line under it, such as *HAL-1921
  Offline Edits Survive a Conflict*, followed by *Rename · Ignore*. *Rename*
  (a click, or `ctrl+r`) types `/rename` into the agent, once it's between
  turns, and the row says *Renaming to …* until its title shows it. *Ignore*
  (a click, or `ctrl+x`) drops the suggestion for good. It's asked for once
  per session, at the same time as the recap.
- **`ctrl+r` renames; it no longer writes the recap again.** Out-of-date
  recaps are rewritten on opening the popup as before. With no title
  suggested, `ctrl+r` asks for one.
- A rename asked of an agent that's working or waiting on you is kept, and
  given when it's next between turns, with the popup open or not.
- **A rename doesn't put a recap out of date.** Nor does anything else Claude
  writes to a transcript that isn't conversation: only an agent's message or
  one of yours does.

## 0.4.2 - 2026-10-10

- **Idle is said only of the Idle stage.** An idle agent whose PR puts it in
  *In review*, *Needs fixing* or *Merged* says *for 3h* on the right, not
  *idle 3h*, and typing `idle` lists the Idle stage alone.

## 0.4.1 - 2026-10-10

- **Two stages renamed.** *Finished* is *Done*, herdr's own word for it and
  the one on the row; *In progress* is *Idle*: nothing is in progress there,
  it's waiting for you to set it going.
- **The stages are in the order of whose move it is.** *Needs you*, *Done*,
  *Needs fixing* and *Idle* are yours; then *Working*, *In review* and
  *Merged*. *Idle* was after *In review*.

## 0.4.0 - 2026-10-10

- **The list is grouped by stage.** Under a heading with a count each: *Needs
  you*, *Finished*, *Needs fixing*, *Working*, *In review*, *In progress*,
  *Merged*. The first, second and fourth are the agent's status, as before;
  an idle agent is placed by its pull request (herdr-github's tokens), so the
  one whose PR is failing is above the ones waiting on a review, and the
  merged ones are at the bottom. An agent started on something new after a
  merge is back in *Working*, or *In progress* between turns. A stage's name
  filters the list.
- **The stages are tabs along the top**, each in its colour with its count,
  in place of the counts by status. `←` / `→` or a click shows one stage
  alone; `esc` goes back to *All* before it closes.

## 0.3.0 - 2026-10-06

- **One look, shared with herdr-github and herdr-linear.** A title, a
  branch, a pull request, an agent and a key are drawn the same in all three
  (see *Colours* in the README). Here: a PR's badge is coloured as
  herdr-github colours a PR (draft peach, open green, merged mauve, closed
  red, with its checks and review in their own colours) instead of all
  green, and the count that needs you is a pill.
- **Keys are pills.** The keys along the bottom are solid chips, coloured by
  what they do; the one under the pointer underlines. Too many for the
  popup's width, and labels shorten, then the least missed keys are left out.
- **The `terminal` theme keeps its hierarchy.** Branches and the dim details
  on the right no longer share the recap's grey.

## 0.2.1 - 2026-10-06

- A PR from herdr-github is on a row once (*#148 merged*), not twice
  (*#148 · #148 merged*): its `prs` token stands for `pr` as well as the
  `pr_*` details.

## 0.2.0 - 2026-10-06

- Type to filter the list, as in herdr-github and herdr-linear: every word
  must match somewhere in an agent's row (its title, status, space, branch,
  recap, what it's waiting for or a token). `esc` clears the filter, then
  closes.
- With letters going to the filter, reply is `tab` (was `r`), and `k` `j`,
  `g` `G` and `q` no longer move or close: use the arrows or `ctrl+p`
  `ctrl+n`, `home` `end`, and `esc`.

## 0.1.2 — 2026-09-28

- Rows keep their height when selected: the selected agent's last prompt,
  model and mode are on the line above the footer instead of in its row, so
  moving down the list no longer shifts everything under it.
- The recap comes right under the title (after what a blocked agent is
  asking), with branch, changes and tasks below it, and on the selected row
  it's drawn in the title's colour so it stays readable on the highlight.
- Recap lines wrap at 80 columns at most, however wide the popup.

## 0.1.1 — 2026-09-25

- Opening the popup while an earlier one is still up (left on another
  client, where you can't see it) closes that one and opens it here,
  instead of failing with *a popup pane is already open*. Another plugin's
  popup is left alone, with a toast saying so.
- The popup's size is in the manifest, so it's the same however it's opened.

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
