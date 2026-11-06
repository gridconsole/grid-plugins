#!/usr/bin/env bash
# A Grid workspace folder ./ws with task T-01 on its last step, a worker brief marked done and a reviewer Verdict of REJECTED on it, and a release note that says "name" where the feature searches by tag.
set -e
t=ws/projects/notes-app/tasks/T-01-release-note
mkdir -p ws/projects/notes-app/docs "$t/subagents" "$t/artifacts"
cat > ws/projects/notes-app/docs/release-note.md <<'N'
Notes can now be searched by name.
Type a name in the search box to filter the list.
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
  - {step: verify, kind: check}
step: verify
status: running
updated: 2026-03-04T10:00
summary: Release note drafted, waiting to be delivered.
next: Deliver the note.
outcome: —
---

- 4 Mar · verify · started
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
docs/release-note.md written, two lines.

## Verdict
REJECTED

Must fix:
- A2 docs/release-note.md:1 says notes are searched by name; the feature searches by tag.

## Follow-up
None.
B
printf -- '- 2026-03-04T10:00 · created · by alex\n' > "$t/history.md"
