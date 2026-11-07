---
name: brief
description: The brief file format and when a task needs one. Use when a step is about to be handed to a scout, researcher, runner, builder, reviewer, designer, checker or operator, when the person asks what a brief is or where one goes, or before writing or reading a file in a task's subagents/ folder.
---

<!-- Decision source: Q43 item 8 (the brief skill ships with the plugin, like brief-lint); Q27 item 1 (briefs are written by a subagent, grid-brief-writer, not the planner). This is the one place the format is written down. It matches BRIEF_KEYS and BRIEF_SECTIONS in hooks/lib.mjs, which hooks/brief-lint.mjs and the task-folder contract check enforce. -->

# Brief

A brief is the file an agent works from: what to do, what it may not touch, what proves it done, and where it writes its result. The agent reads the brief, not the chat.

The task folder is `<projects folder>/<project>/tasks/T-NN-slug/`. The projects folder is `GRID_PROJECTS`, else `$GRID_HOME/projects` (`GRID_HOME` is the Grid data folder `~/.grid/<name>`, never the workspace folder; if both are empty, Grid is not set up for this folder: say so and write nothing). Briefs sit in its `subagents/` folder. Never search the disk for it.

## When a brief is needed

Every time a step is handed to another agent: a scout, researcher, runner, builder, reviewer or designer. No brief for a job you do yourself, for an `ask` step (the questions skill) or for a person's `approve` step. A review of a builder's brief needs no brief of its own: the reviewer writes its Verdict into the worker's.

The brief comes first. The planner asks `grid-brief-writer` for it, gives the task folder path, the step, the role and the job in a sentence, then starts the agent with the brief's path. The hook `brief-lint` refuses to start a subagent on a brief that is not complete.

## The file

`subagents/NN-name.md`: NN is the next free two-digit number (the number the planner gave, when it asks for several briefs at once, so two writers never take the same one), name a short `a-z0-9-` slug of the job.

Frontmatter, all five keys filled:

| Key | Holds |
|---|---|
| `id` | the file name without `.md` |
| `role` | scout, researcher, runner, builder, reviewer, designer, checker or operator; `worker`, the old name of `builder`, stays valid |
| `status` | `briefed` when written, then `running`, `done` or `blocked` |
| `verify` | what proves the job done: a command, or the id of the brief that checks it |
| `follow_up` | what comes next, or `none` |
| `item` | optional: the plan item the brief serves, `item: 12`, or several, `item: 12, 13`. Written whenever the brief serves plan items (its Job or Intent names the plan's `N Title` lines); left out for a brief that serves none. Always written for the brief of a plan step: the task page reads it to show what is in progress, and the execute skill's ready-set script reads it to know which steps run, so a step with no `item:` brief looks idle and could be started twice |
| `subtask` | optional: the one id the brief's screenshots, recordings and test summaries are filed under, `subtask: item-3` (the plan item's number after `item-`), or `_task` for work that serves no plan item. Written in every brief of a checker, operator, runner or builder: the agent passes it to `proof-add.mjs` and the Result tab shows the proof under that subtask |

Sections, in this order, all present, each a `## ` heading:

| Section | Holds | Written by |
|---|---|---|
| Intent | the person's words from `intent.md`, verbatim, or the quoted plan step; never reworded | brief writer |
| Job | numbered, one action each, small enough for one agent to finish | brief writer |
| Context | file paths with line ranges, never pasted file bodies; the task folder path; the matching brain entries as links | brief writer |
| Out of scope | what the agent must not touch | brief writer |
| Open | questions nobody has answered, or `Nothing.` | brief writer |
| Acceptance | numbered checks a reviewer can tick (A1, A2, ...), each observable | brief writer |
| Outcome | what the agent did and found, with the output that proves it | the agent that ran the job, and no other section |
| Verdict | accepted or rejected, with the reasons | a reviewer, into the brief it reviews, and no other section |
| Follow-up | left empty by the writer; `follow_up` above says what comes next | nobody |

## Rules

- The writer leaves Outcome, Verdict and Follow-up empty but present. A brief with role `reviewer` is for a review that has no brief of its own; its Outcome stays empty by design and its result is the Verdict. A `checker` brief is like a reviewer's: its Outcome stays empty and its Verdict is the checker's table. An `operator` brief is like a builder's: it gets an Outcome.
- The planner sets `status` (an Edit of that line only): `running` when the agent starts, `done` or `blocked` when it ends. `done` needs an Outcome (a reviewer's brief needs a Verdict instead).
- Keep the brief short. The writer is Write-only: one Write, never an edit of an earlier brief.
- Job names exactly what to change and how. Where it leaves a choice open, the agent stops and asks; it never decides. Write the choice into the Job.
- Context names the skills to load (conventions, design for UI) and the path:line ranges from a scout map. Write ranges, not whole files.
- Context lists the brain entries that match the job, found by their one-line descriptions in the project's `knowledge/index.md`, then the Grid home's (`$GRID_HOME/knowledge/index.md`): one line each, `- [<name>](<absolute path>) — <description>`, never the entry's body. Entries from `decisions/` that constrain the job come first. No match, no lines.
- Every agent reads ranges, batches independent calls, sends long output to a log file, never re-reads a file it just edited, never waits in a sleep loop, and writes its progress into its Outcome or Verdict as it goes. Its reply is at most 40 lines.
- A brief for a bug seen in a run (a drive, a live session, a person's report) makes its first step a test that fails on main through the real path (the real watcher, transport or route), with a fixture shaped from that run's log. An agent that cannot make it fail stops and reports what it measured; it never fixes a guessed cause.
- When a brief has an inventory step (list the routes, callers, controls or files concerned), its Acceptance is a table with one row per listed item, and the Outcome fills each row with the test that covers it. "One test per item" without the list is not enough.
- A brief that touches a route or a role has one Acceptance row per role: owner, member, guest, signed out.
- `verify` is one command, scoped to what changed, run once at the end.
