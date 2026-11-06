#!/usr/bin/env bash
# A Grid workspace folder ./ws with task T-01 at its verify step (kind check), after a finished build, then an approve step; the project's check.sh always exits 1.
set -e
t=ws/projects/notes-app/tasks/T-01-tag-filter
mkdir -p ws/projects/notes-app/src "$t/subagents"
cat > ws/projects/notes-app/src/store.js <<'S'
const notes = [
  { id: 1, title: "Groceries", body: "milk", tags: ["home"] },
];

export function listNotes() {
  return [...notes];
}

export function filterByTag(tag) {
  return notes.filter((n) => n.tags.includes(tag));
}
S
cat > ws/projects/notes-app/check.sh <<'S'
#!/bin/sh
echo "notes-app checks"
echo "FAIL filterByTag: expected 1 note for tag Home, got 0 (the match is case-sensitive)"
echo "1 failed, 3 passed"
exit 1
S
cat > "$t/intent.md" <<'I'
let people filter the note list by tag
I
cat > "$t/task.md" <<'T'
---
id: T-01
title: Filter notes by tag
project: notes-app
owner: alex
source: idea
size: normal
workflow:
  - {step: build, kind: run}
  - {step: verify, kind: check}
  - {step: review, kind: approve}
step: verify
status: running
updated: 2026-03-04T10:00
summary: Let people filter the note list by tag.
next: Run the check, sh check.sh in the project folder; it must exit 0.
---

- 4 Mar · build · tag filter written
T
cat > "$t/subagents/01-build.md" <<'B'
---
id: 01-build
role: worker
status: done
verify: self
follow_up: none
---

## Intent
Let people filter the note list by tag.

## Job
1. Add filterByTag to src/store.js.

## Context
- projects/notes-app/src/store.js

## Out of scope
Any UI.

## Open
Nothing.

## Acceptance
A1. filterByTag(tag) returns the notes that carry the tag.

## Outcome
filterByTag added to src/store.js.

## Verdict

## Follow-up
None.
B
printf -- '- 2026-03-04T10:00 · created · by alex\n' > "$t/history.md"
