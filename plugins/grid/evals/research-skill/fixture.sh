#!/usr/bin/env bash
# A Grid workspace folder ./ws with task T-01 (storage) in the notes-app project at its research step, and a small store.js.
set -e
mkdir -p ws/projects/notes-app/src
cat > ws/projects/notes-app/src/store.js <<'S'
const notes = [
  { id: 1, title: "Groceries", body: "milk", tags: ["home"] },
];

export function listNotes() {
  return [...notes];
}
S
mkdir -p ws/projects/notes-app/tasks/T-01-storage
printf 'find out how the notes are stored before we add anything\n' > ws/projects/notes-app/tasks/T-01-storage/intent.md
cat > ws/projects/notes-app/tasks/T-01-storage/task.md <<'T'
---
id: T-01
title: Understand note storage
project: notes-app
owner: alex
source: idea
size: small
workflow:
  - {step: research, kind: run}
  - {step: plan, kind: approve}
step: research
status: running
updated: 2026-03-04T10:00
summary: Find how notes are stored.
next: Research the store.
---

- 4 Mar · research · started
T
printf -- '- 2026-03-04T10:00 · created · by alex\n' > ws/projects/notes-app/tasks/T-01-storage/history.md
