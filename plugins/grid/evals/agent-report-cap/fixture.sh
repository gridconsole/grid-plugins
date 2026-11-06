#!/usr/bin/env bash
# A Grid workspace folder ./ws with a worker brief that asks for a 300-line generated file under the task's artifacts/.
set -e
d=ws/projects/notes-app/tasks/T-01-sample
mkdir -p "$d/subagents" "$d/artifacts" ws/projects/notes-app/src
printf 'make a long sample list of rows for the export test\n' > "$d/intent.md"
cat > "$d/task.md" <<'T'
---
id: T-01
title: Sample rows
project: notes-app
owner: alex
source: idea
size: normal
workflow:
  - {step: build, kind: run}
  - {step: review, kind: approve}
step: build
status: running
updated: 2026-03-04T10:00
summary: Generate a long sample list for the export test.
next: Run the build step.
---

- 4 Mar · build · brief written
T
printf -- '- 2026-03-04T09:00 · created · by alex\n' > "$d/history.md"
printf 'export const notes = [];\n' > ws/projects/notes-app/src/store.js
printf 'tasks/\n' > ws/projects/notes-app/.gitignore
git -C ws/projects/notes-app init -q
git -C ws/projects/notes-app add -A
git -C ws/projects/notes-app -c user.name=alex -c user.email=alex@example.com commit -q -m "Start the notes app"
cat > "$d/subagents/01-sample-rows.md" <<'T'
---
id: 01-sample-rows
role: worker
status: briefed
verify: self
follow_up: none
---

## Intent
"make a long sample list of rows for the export test"

## Job
1. Generate `artifacts/rows.txt` in this task folder: exactly 300 lines, line N reading `row N: S` where S is N squared (`row 1: 1`, `row 2: 4`, ... `row 300: 90000`).

## Context
- The task folder: ws/projects/notes-app/tasks/T-01-sample

## Out of scope
The project's code and every other task file.

## Open
Nothing.

## Acceptance
A1. `artifacts/rows.txt` has exactly 300 lines.
A2. Line 300 is `row 300: 90000`.

## Outcome

## Verdict

## Follow-up
T
