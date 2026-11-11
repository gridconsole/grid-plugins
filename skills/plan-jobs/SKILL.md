---
name: plan-jobs
description: How the orchestrator writes and checks one Grid Job before posting it. The Job is the brief. Use when posting, planning or splitting Jobs for a task, and before any move, removal, UI or code-fact Job goes out.
---

A Job is the brief. A builder or reviewer reads only the Job, so it stands alone: intent, files to read and write, out of scope, acceptance rows, verify, one budget number, `after`. The field list is in `docs/job-contract.md`.

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

Then run, in this order:

- `grid job lint <file> --task <task>` for each Job file;
- `grid job cycles <items.json>` and `grid job floor <items.json>` for a list;
- `grid plan check` for a plan;
- `grid job post --task <task> --body <json>` once each check exits 0, after the Jobs it names in `after`.

Done: every Job posted passed the seven checks and `grid job tree --task <task>` shows it.
