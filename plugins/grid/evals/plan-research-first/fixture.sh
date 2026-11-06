#!/usr/bin/env bash
# A Grid workspace folder ./ws with task T-01 (tags) at its plan step in the notes-app project, Normal size, no research yet.
# The intent leaves one choice to Alex, so the plan has a decision to ask.
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
d=ws/projects/notes-app/tasks/T-01-tags
mkdir -p "$d"
cat > "$d/intent.md" <<'I'
let people find notes by their tags. I have not decided whether the tags show as a sidebar you click or as a filter box you type into: that is my call, so ask me and say which you would pick.
I
cat > "$d/task.md" <<'T'
---
id: T-01
title: Find notes by tag
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
summary: Let people find notes by tag.
next: Write the plan.
---

- 4 Mar · plan · task created
T
printf -- '- 2026-03-04T10:00 · created · by alex\n' > "$d/history.md"
