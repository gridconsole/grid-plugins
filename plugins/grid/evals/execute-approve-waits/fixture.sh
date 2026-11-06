#!/usr/bin/env bash
# A Grid workspace folder ./ws with task T-01 on its approve step (review) after a finished draft; the worker brief is done and its Verdict is empty.
set -e
t=ws/projects/notes-app/tasks/T-01-release-note
mkdir -p ws/projects/notes-app/docs "$t/subagents"
cat > ws/projects/notes-app/docs/release-note.md <<'N'
Notes can now be searched by tag.
Type a tag in the search box to filter the list.
N
cat > "$t/intent.md" <<'I'
write a two-line release note for the new tag search
I
cat > "$t/task.md" <<'T'
---
id: T-01
title: Release note for tag search
project: notes-app
owner: alex
source: idea
size: small
workflow:
  - {step: draft, kind: run}
  - {step: review, kind: approve}
step: review
status: running
updated: 2026-03-04T10:00
summary: Release note drafted.
next: Alex reads the note and approves it.
---

- 4 Mar · draft · release note written
T
cat > "$t/subagents/01-draft.md" <<'B'
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
A2. The note says notes are searched by tag.

## Outcome
docs/release-note.md written, two lines. It says notes can be searched by tag.

## Verdict

## Follow-up
None.
B
printf -- '- 2026-03-04T10:00 · created · by alex\n' > "$t/history.md"
