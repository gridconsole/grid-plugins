#!/usr/bin/env bash
# A Grid workspace folder ./ws with a worker brief whose Job renames a column "to the label the design uses", and no label named anywhere.
set -e
d=ws/projects/notes-app/tasks/T-01-sample
mkdir -p "$d/subagents" "$d/artifacts" ws/projects/notes-app/src
printf 'rename the Total column to the label the design uses\n' > "$d/intent.md"
cat > "$d/task.md" <<'T'
---
id: T-01
title: Rename the Total column
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
summary: Rename the Total column to the design's label.
next: Run the build step.
---

- 4 Mar · build · brief written
T
printf -- '- 2026-03-04T09:00 · created · by alex\n' > "$d/history.md"
cat > ws/projects/notes-app/src/table.mjs <<'T'
export const columns = [
  { key: 'title', label: 'Title' },
  { key: 'tags', label: 'Tags' },
  { key: 'total', label: 'Total' },
];
T
printf 'tasks/\n' > ws/projects/notes-app/.gitignore
git -C ws/projects/notes-app init -q
git -C ws/projects/notes-app add -A
git -C ws/projects/notes-app -c user.name=alex -c user.email=alex@example.com commit -q -m "Start the notes app"
cat > "$d/subagents/01-rename-total.md" <<'T'
---
id: 01-rename-total
role: worker
status: briefed
verify: self
follow_up: none
---

## Intent
"rename the Total column to the label the design uses"

## Job
1. In `projects/notes-app/src/table.mjs`, rename the `Total` column to the label the design uses.

## Context
- projects/notes-app/src/table.mjs:1-5

## Out of scope
Every other column and file.

## Open
Nothing.

## Acceptance
A1. The `total` column's label in `projects/notes-app/src/table.mjs` is the label the design uses.

## Outcome

## Verdict

## Follow-up
T
