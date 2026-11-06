#!/usr/bin/env bash
# A Grid workspace folder ./ws with one task, T-01-sample, in the notes-app project, and the code the plan rests on.
set -e
mkdir -p ws/projects/notes-app/src
cat > ws/projects/notes-app/src/store.js <<'S'
const notes = [
  { id: 1, title: "Groceries", body: "milk", tags: ["home"] },
  { id: 2, title: "Standup", body: "ship search", tags: ["Work", "daily"] },
];

export function listNotes() {
  return [...notes];
}
S
cat > ws/projects/notes-app/src/list.js <<'L'
import { listNotes } from "./store.js";

export function renderList(root) {
  root.innerHTML = listNotes().map((n) => `<li>${n.title}</li>`).join("");
}
L
d=ws/projects/notes-app/tasks/T-01-sample
mkdir -p "$d"
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
step: plan
status: running
updated: 2026-03-04T10:00
summary: Let people filter the note list by tag.
next: Write the plan.
---

- 4 Mar · plan · task created
T
printf -- '- 2026-03-04T10:00 · created · by alex\n' > "$d/history.md"
