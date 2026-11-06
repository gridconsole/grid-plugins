#!/usr/bin/env bash
# A Grid workspace folder ./ws with task T-01 (release note) at its draft step (kind run), then a check step, and a notes-app routing.md.
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
cat > ws/projects/notes-app/routing.md <<'R'
# Routing · project notes-app

| Topic | Goes to | Rule | Why |
|---|---|---|---|
| Release wording | sam | Asks before it ships | Sam owns the changelog |
| Storage choices | alex | Decides | Alex owns the data layer |

## Members

- alex · owner
- sam · reviewer

## Agents

| Applies to | Agent | Why |
|---|---|---|
| kind: run, project: notes-app | grid-designer | Notes-app run steps are written up by the designer |
| kind: check | grid-runner | Checks are commands |
| project: notes-app | grid-reviewer | Fallback for other notes-app steps |
R
mkdir -p ws/projects/notes-app/tasks/T-01-release-note
cat > ws/projects/notes-app/tasks/T-01-release-note/intent.md <<'I'
write a two-line release note for the new tag search and save it as artifacts/release-note.md
I
cat > ws/projects/notes-app/tasks/T-01-release-note/task.md <<'T'
---
id: T-01
title: Release note for tag search
project: notes-app
owner: alex
source: idea
size: small
workflow:
  - {step: draft, kind: run}
  - {step: verify, kind: check}
step: draft
status: running
updated: 2026-03-04T10:00
summary: Write the release note for tag search.
next: Draft the release note.
---

- 4 Mar · draft · started
T
printf -- '- 2026-03-04T10:00 · created · by alex\n' > ws/projects/notes-app/tasks/T-01-release-note/history.md
