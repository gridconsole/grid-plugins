#!/usr/bin/env bash
# A Grid workspace folder ./ws with task T-01 in the notes-app project, at its build step (kind run), plan accepted.
set -e
d=ws/projects/notes-app/tasks/T-01-sample
mkdir -p "$d" ws/projects/notes-app/src
printf 'add a way to search the notes by tag\n' > "$d/intent.md"
cat > "$d/task.md" <<'T'
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
  - {step: check, kind: check}
step: build
status: running
updated: 2026-03-04T10:00
summary: Let people filter the note list by tag.
next: Build the tag filter and the tag box.
---

- 4 Mar · plan · plan accepted, rev 1
T
cat > "$d/plan.md" <<'T'
# Plan, rev 1 · dfa6b6
- [ ] 1 Add a tag filter function to the note store
- [ ] 2 Add the tag box to the list view
- [ ] 3 Cover both with tests

Verify: `node --test` passes and the list narrows by tag
T
cat > "$d/history.md" <<'T'
- 2026-03-04T09:00 · created · by alex
- 2026-03-04T09:30 · plan ready · rev 1 · dfa6b6 · by grid-planner
- 2026-03-04T09:40 · plan accepted · rev 1 · dfa6b6 · by alex
T
cat > ws/projects/notes-app/src/store.js <<'T'
export const notes = [];
export function addNote(note) { notes.push(note); }
T
