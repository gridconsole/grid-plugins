#!/usr/bin/env bash
# A Grid workspace folder ./ws with task T-01 on its last step, a done worker brief and the deliverable in the project's docs folder.
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
mkdir -p ws/projects/notes-app/tasks/T-01-release-note/subagents ws/projects/notes-app/tasks/T-01-release-note/artifacts ws/projects/notes-app/docs
cat > ws/projects/notes-app/docs/release-note.md <<'N'
Notes can now be searched by tag.
Type a tag in the search box to filter the list.
N
cat > ws/projects/notes-app/tasks/T-01-release-note/intent.md <<'I'
write a two-line release note for the new tag search
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
step: verify
status: running
updated: 2026-03-04T10:00
summary: Release note drafted, check passed.
next: Deliver the note.
outcome: —
---

- 4 Mar · verify · started
T
cat > ws/projects/notes-app/tasks/T-01-release-note/subagents/01-draft.md <<'B'
---
id: 01-draft
role: worker
status: done
verify: self
follow_up: none
---

## Intent
Write a two-line release note for the new tag search.

## Job
1. Write the note to docs/release-note.md in the project.

## Context
- projects/notes-app/src/store.js

## Out of scope
Any change to the code.

## Open
Nothing.

## Acceptance
A1. docs/release-note.md exists and has two lines.

## Outcome
docs/release-note.md written, two lines. It says notes can be searched by tag.

## Verdict

## Follow-up
None.
B
printf -- '- 2026-03-04T10:00 · created · by alex\n' > ws/projects/notes-app/tasks/T-01-release-note/history.md
