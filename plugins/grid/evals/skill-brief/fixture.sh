#!/usr/bin/env bash
# A Grid workspace folder ./ws with task T-01 in the notes-app project at its research step (no briefs yet),
# and the project's code at ./code/notes-app.
set -e
d=ws/projects/notes-app/tasks/T-01-sample
mkdir -p "$d/subagents" code/notes-app/src
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
  - {step: research, kind: run}
  - {step: plan, kind: approve}
  - {step: build, kind: run}
step: research
status: running
updated: 2026-03-04T10:00
summary: Let people filter the note list by tag.
next: Find where notes are stored.
---

- 4 Mar · research · started
T
printf -- '- 2026-03-04T09:00 · created · by alex\n' > "$d/history.md"
cat > code/notes-app/src/store.js <<'T'
export const notes = [];
export function addNote(note) { notes.push(note); }
T
