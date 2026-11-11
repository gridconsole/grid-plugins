---
name: plan-jobs
description: How to write one Grid Job so it works as the brief: fields, acceptance rows, `after` dependencies, and pointers with a stated why instead of pasted text. Use when posting or planning Jobs for a task.
---

A Job is the brief. A builder or reviewer reads only the Job, so it must stand alone. Standing alone does not mean carrying everything: it means pointing at everything the worker needs, with a reason to follow each pointer.

Fields (see docs/job-contract.md in the plugin repo for the full list): `id`, `item`, `title`, `role`, `intent` (with `owner`), `links`, `read`, `write`, `out_of_scope`, `acceptance`, `verify`, `budget`, `on_blocked`, `after`.

- `intent`: three pointers (`owner`, `item`, `goal`, each `file#anchor`) to where the goal is written: the owner's request, the plan step, the plan's verify line. It states the goal and never pastes file or note text.
- `write`: the only files the Job may change; `read`: what to read first.
- `acceptance`: numbered rows A1, A2, ..., each checkable by a reviewer from files or command output.
- `verify`: the test command and a positive `min_tests`; for research or non-code work, `kind: file-exists` or `kind: reviewer-check`.
- `after`: bare Job ids that must be done first, or objects with `job`, `kind` (`done`, `failed`, `children-done`) and `why` (8 to 160 characters). A gate is `gate:Q-7`. A review Job names the build Job it checks.
- A Job for a person has role Person; its intent says what to decide and what the options are.

## Pointers

Point, do not paste. A pasted block goes stale, fills the worker's context and hides which part mattered; a pointer with a why lets the worker read only what it needs.

- Before writing a Job, run `grid brain search "<words>"` with the words of the goal. Link each note it finds that the worker should follow.
- `links`: entries `{to, rel, why}`. `to` is `brain:project/<id>`, a Job (`job:J-3`) or a person node. `rel` is one of depends-on, explains, decided-by, supersedes, duplicates, related, continues, validates, uses, owns, decides, approves, knows (blocks and same-files are derived by the CLI; do not write them). `why` is 8 to 160 characters and says what the worker gains by following the link; "see also", "related" and the note's own name fail lint.
- `read`: name the lines, not the file: `repo:path/file.go:40-95`, or a heading anchor such as `task:research.md#format`. A bare file name makes the worker read all of it.
- `intent` and `acceptance` state the goal and the check in your own words. If you are about to copy a paragraph from a file or a note, link or read it instead.
- Keep a Job small: one outcome, one owner, a read list of a few ranges. If it needs more, split it into Jobs joined by `after`.

One full example. It lints clean once the note and file it points at exist:

```yaml
id: J-4
item: 1
title: Document the export format
role: builder
skills: []
review: L1
risk: []
max_rounds: 2
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

Check before posting: `grid job lint <file> --task <task>`; for several Jobs `grid job cycles <items.json>` and `grid job floor <items.json>`; for a plan `grid plan check`.
