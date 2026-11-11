---
name: plan-jobs
description: How the orchestrator writes and checks one Grid Job before posting it: the Job is the brief, and it points at what the worker needs with a stated why instead of pasting text. Use when posting, planning or splitting Jobs for a task, and before any move, removal, UI or code-fact Job goes out.
---

A Job is the brief. A builder or reviewer reads only the Job, so it stands alone: intent, files to read and write, out of scope, acceptance rows, verify, one budget number, `after`. Standing alone does not mean carrying everything: it means pointing at everything the worker needs, with a reason to follow each pointer. The field list is in `docs/job-contract.md`.

Owner decisions that apply here:

- "No Grid-made guards, caps or limits": use the Claude Code built-ins. Add no hook, hold or cap to a Job.
- Approvals and gates come from the workflow in the brain with a default: read it with `grid workflow show`, never hard-code one. A step only a person can do is a Job with role Person (`grid job post --person`).
- Everything is a Job and the Job is the brief. Tasks carry on by themselves until a person point. No plan mode.
- The brain needs no attention from the person: a subagent curates proposals (`grid brain propose`), nobody asks the person to review one.
- Every link carries a why: each `links` entry and each `after` object has a `why` of 8 to 160 characters.

Checks the orchestrator runs on each Job before posting it (the rules of 10 Nov). A Job that fails one is rewritten, not posted:

1. Move or removal: the Job lists the items, one acceptance row per item, by name. "Move the helpers" without the list is not enough.
2. Code fact: every claim about code in the Job has a `path:line` that you read in this session (Read or `grep -n`). A fact from memory or from an older note is read again or left out.
3. UI field: each field names its data source, a served op answer, `frame/model.mjs` or a decision doc. A field with no source in the read is written as a row the builder must mark NOT DONE, never guessed.
4. UI rows are checked at widths 390, 1440 and 3840; the acceptance says so for each.
5. One budget number: `budget.turns_estimate`. If the step will not fit in about 60 calls, split it into Jobs before launch, each with its own `after` and a why.
6. Hand-off: acceptance rows are written so each can be answered HOLDS (with a command and exit code or `path:line`) or NOT DONE. The builder hands off a proof table; a row it could not prove is NOT DONE.
7. Verify: a test command with a positive `min_tests`, or `kind: file-exists` or `kind: reviewer-check` for non-code work.

## Bash

- Send one command per Bash call: no `;`, no `&&`, no pipe, no `$(...)` and no `echo $?`. The exit code is in the tool result.
- `grid` and `git` are pre-approved. A Job's verify command is still run, as the Job writes it, in a call of its own.
- Put no `VAR=value` in front of a command; use the command's own flags.
- Use Read, Glob and Grep instead of ls, cat, find and grep. Use Write and Edit instead of sed, cp and heredocs.
- Never set `dangerouslyDisableSandbox`.

## Pointers

Point, do not paste. A pasted block goes stale, fills the worker's context and hides which part mattered; a pointer with a why lets the worker read only what it needs.

- `intent`: three pointers (`owner`, `item`, `goal`, each `file#anchor`) to where the goal is written: the owner's request, the plan step, the plan's verify line. It states the goal and never pastes file or note text.
- Before writing a Job, run `grid brain search "<words>"` with the words of the goal. Link each note it finds that the worker should follow.
- `links`: entries `{to, rel, why}`. `to` is `brain:project/<id>`, a Job (`job:J-3`) or a person node. `rel` is one of depends-on, explains, decided-by, supersedes, duplicates, related, continues, validates, uses, owns, decides, approves, knows (blocks and same-files are derived by the CLI; do not write them). `why` is 8 to 160 characters and says what the worker gains by following the link; "see also", "related" and the note's own name fail lint.
- `read`: name the lines, not the file: `repo:path/file.go:40-95`, or a heading anchor such as `task:research.md#format`. A bare file name makes the worker read all of it.
- `intent` and `acceptance` state the goal and the check in your own words. If you are about to copy a paragraph from a file or a note, link or read it instead.
- `after`: bare Job ids that must be done first, or objects with `job`, `kind` (`done`, `failed`, `children-done`) and `why` (8 to 160 characters). A gate is `gate:Q-7`. A review Job names the build Job it checks.
- Keep a Job small: one outcome, one owner, a read list of a few ranges. If it needs more, split it into Jobs joined by `after`.
- The last Job of a task is the deliver Job: title "Deliver the task", role orchestrator, `after` names every other Job (each with a why), and it runs `grid deliver --task <task> --outcome <text> --json`.

## Pointers into plan.md

`plan.md#step-N` resolves only a checklist line `- [ ] N title` (under `## Steps`); a `### Step 1: ...` heading gives `anchor "step-1" not found`. `plan.md#verify` needs a `## Verify` heading with a paragraph under it. plan.md has no other anchors: a slug such as `plan.md#update-install-md` fails. In other files a heading anchor is the heading's slug or its `{#id}`. `## Where it stands` is never a source for `intent` or `read`.

```markdown
## Steps
- [ ] 1 Update install.md
  jobs: []
## Verify
go test ./internal/plan/
## Where it stands
```

Run `grid job lint <file> --task <task>` and post only a Job with no errors. A post that prints "render will refuse this Job" is fixed and posted again as a new Job.

One full example. It lints clean once the note and file it points at exist:

```yaml
id: J-4
item: 1
title: Document the export format
role: builder
skills: []
risk: []
intent:
  owner: intent.md#request
  item: plan.md#step-1
  goal: plan.md#verify
links:
  - to: brain:project/export-format
    rel: decided-by
    why: Sets the column order the document must state and keep stable.
keep: []
decisions: []
read:
  - task:research.md#format
  - repo:docs/export.md:10-30
write:
  - docs/export.md
out_of_scope:
  - runtime/**
acceptance:
  - id: A1
    text: docs/export.md lists the export columns in the order the project note fixes.
    traces: item
  - id: A2
    text: The export tests named in verify pass.
    traces: item
verify:
  command: go test ./internal/export/
  min_tests: 2
budget:
  turns_estimate: 8
  reads_before_first_edit: 3
on_blocked: ask-and-continue
after:
  - J-3
```

## Re-plan Job

A Job with role orchestrator and skills plan-jobs, titled "Re-plan after <cancelled> was cancelled", means a Job was cancelled and the Jobs after it may no longer run. Steps:

1. Read the cancelled Job and why it stopped: `grid job show <job> --task <task>` (its `exit_code` and result), then `grid job tree --task <task>` for the Jobs that wait on it.
2. Decide for each waiter, by the task's goal: keep it, or cancel it.
3. Keep: post a replacement for the cancelled Job (`grid job post --task <task> --body <json>`, lint first) and point the waiter's `after` at it, so the waiter can still run.
4. Cancel: a waiter that can no longer run or no longer matters is stopped with `grid job cancel --task <task> --job <job> --reason <why>`.
5. A step only a person can do is a person Job (`grid job post --task <task> --person <name>`), as in the owner decisions above.
6. Post each replacement by the checks above, then read `grid job tree --task <task>`: no waiter is left waiting on the cancelled Job.

## Run

Then run, in this order:

- `grid job lint <file> --task <task>` for each Job file;
- `grid job cycles <items.json>` for a list;
- `grid plan check` for a plan;
- `grid job post --task <task> --body <json>` once each check exits 0, after the Jobs it names in `after`.

Done: every Job posted passed the seven checks and `grid job tree --task <task>` shows it.
