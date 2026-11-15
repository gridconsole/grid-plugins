---
name: plan-jobs
description: How the orchestrator writes and checks one Grid Job before posting it: the Job is the brief, and it points at what the worker needs with a stated why instead of pasting text. Use when posting, planning or splitting Jobs for a task, and before any move, removal, UI or code-fact Job goes out.
---

A Job is the brief. A builder or reviewer reads only the Job, so it stands alone: intent, files to read and write, out of scope, acceptance rows, verify, one budget number, `after`. Standing alone does not mean carrying everything: it means pointing at everything the worker needs, with a reason to follow each pointer. The field list is in `docs/job-contract.md`.

Owner decisions that apply here:

- "No Grid-made guards, caps or limits": use the Claude Code built-ins. Add no hook, hold or cap to a Job.
- Approvals and gates come from the workflow in the brain with a default: read the workflow this task runs with `grid workflow show --task <task> --json` and use its `task.steps` (kind, agent, approvers) and `task.rounds`, never hard-code one. A step only a person can do is a Job with role Person (`grid job post --person`). An approve step whose `posted_by` is `deliver` is not a Job you post: `grid deliver` posts that person Job, so your Jobs end at the step before it. A `check` step right after a `run` step is not a Job you post either: the worker posts the review of a run step's Job when that Job posts its result (`J-2.1` for `J-2`). A review Job you post is one asked for on top of it (a second check step, or the person's words), and its `review_reason` says who asked; `grid job lint` warns `job.review-twice` on a review Job that waits on a Job and has neither `review_reason` nor `discovered_from`.
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
7. Verify: a test command with a positive `min_tests`, or `kind: file-exists` or `kind: reviewer-check` for non-code work. Before you name a whole-suite verify (`go test ./...`), run it once on the base; when it is already red, name the tests the change touches in verify and write the pre-existing failure in the Job's out of scope, so no builder is rejected for it.
8. Note to keep: when the owner's request or the plan step asks for a note, a decision or knowledge to keep ("leave a note for whoever works on it next", "record why we chose it", "write down how X is handled"), the Job that does the work has an acceptance row of its own that names it: what it says and its topic (`conventions` for a practice or a lesson, `decisions` for a choice and its reason; `grid brain topics` lists them). The row says the builder proposes it with `grid brain propose --task <task>` and lists it in the result's `knowledge`. A file the request also names (a README, a doc) is a second row; a file never stands in for the proposal. Without that row the builder reads the ask as a file to write, and no curator Job follows (eval round 12, E3).

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
- `after`: bare Job ids that must be done first, or objects with `job`, `kind` (`done`, `failed`, `children-done`) and `why` (8 to 160 characters). A gate is `gate:Q-7`. A review Job you post (only one asked for on top of the worker's, see the workflow rule above) names the build Job it checks and carries `review_reason`.
- `repo`: in a project with more than one repo, a code Job names its `repo` (one of the project's repos); with none named the Job runs in the first repo of project.md.
- Keep a Job small: one outcome, one owner, a read list of a few ranges. If it needs more, split it into Jobs joined by `after`.
- Post it with `grid job post --task <task> --type job --body '<json>'`, the JSON body in single quotes: the shell runs backticks and `$` inside double quotes, so a backticked `grid deliver ...` in an acceptance row would run there and vanish from the body. A single quote in the text is the JSON escape `\u0027`; or write the body to a file and post it with `--body -` and a `<` redirect from that file.
- The last Job of a task is the deliver Job: title "Deliver the task", role orchestrator, `after` names every other Job (each with a why; a Job the worker reviews is named with kind `children-done`, so delivery waits until that Job and its review are accepted, or, when the review sends it back, its rework and the rework's review), and it runs `grid deliver --task <task> --outcome <text> --json`.

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

Run `grid job lint <file> --task <task>` and post only a Job with no errors. A post that prints "render will refuse this Job" is fixed and posted again as a new Job. A step's `who` (its agent, without `grid-`; `worker` is builder) is the Job's role, and `grid job lint` warns with `job.role-agent` when no agent runs a role.

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

1. Read why the cancelled Job stopped: its attempt lines in `grid job tree --task <task>` (state, exit and code) and its result, if any, with `grid feed read --task <task> --job <job> --type result`; then the same tree shows the Jobs that wait on it.
2. Decide for each waiter, by the task's goal: keep it, or cancel it.
3. Keep: post the cancelled Job's work again as a new Job with a new id (`grid job post --task <task> --type job --body <json>`, lint first), and cancel each waiter on the dead Job and post it again as a new Job whose `after` names the replacement; a posted Job is never edited.
4. Cancel: a waiter that can no longer run or no longer matters is stopped with `grid job cancel --task <task> --job <job> --reason <why>`.
5. A step only a person can do is a person Job (`grid job post --task <task> --person <name>`), as in the owner decisions above.
6. Post each replacement by the checks above, then read `grid job tree --task <task>`: no waiter is left waiting on the cancelled Job.

## Run

Then run, in this order:

- `grid job lint <file> --task <task>` for each Job file;
- `grid job cycles <items.json>` for a list;
- `grid plan check` for a plan;
- `grid job post --task <task> --type job --body <json>` once each check exits 0, after the Jobs it names in `after`.

Done: every Job posted passed the eight checks and `grid job tree --task <task>` shows it.
