#!/usr/bin/env bash
# A Grid workspace folder ./ws with task T-01-sample at its build step, a plan.md and no views yet.
set -e
d=ws/projects/notes-app/tasks/T-01-sample
mkdir -p "$d"
printf 'add a way to search the notes by tag\n' > "$d/intent.md"
cat > "$d/task.md" <<'X'
---
id: T-01
title: Search notes by tag
project: notes-app
owner: alex
source: idea
size: normal
workflow:
  - {step: plan, kind: approve}
  - {step: build, kind: run}
  - {step: review, kind: approve}
step: build
status: running
updated: 2026-03-04T10:00
summary: Let people filter the note list by tag.
next: Build the tag search.
---

- 4 Mar · build · task created
X
printf -- '- 2026-03-04T10:00 · created · by alex\n' > "$d/history.md"
cat > "$d/plan.md" <<'X'
# Plan, rev 1 · 3f9a1c

- [x] 1 Add a tag filter function to the store
  verify: `node --test` passes for the filter
  evidence: evidence/store-tests.txt
- [ ] 2 Add the tag box to the list view
  verify: typing narrows the list
- [ ] 3 Cover both with tests

Verify: `node --test` passes and the list narrows by tag
X
mkdir -p "$d/evidence"
printf "3 tests, 3 pass\n" > "$d/evidence/store-tests.txt"
