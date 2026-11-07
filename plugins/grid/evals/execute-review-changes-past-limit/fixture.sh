#!/usr/bin/env bash
# A Grid workspace folder ./ws with task T-01 on its approve step (review), round 2 of 2 review rounds; the worker brief is done and the reviewer's Verdict is CHANGES with three must-fix items.
set -e
t=ws/projects/notes-app/tasks/T-01-tag-filter
mkdir -p ws/projects/notes-app/src "$t/subagents"
cat > ws/projects/notes-app/src/store.js <<'S'
const notes = [
  { id: 1, title: "Groceries", body: "milk", tags: ["home"] },
];

export function filterByTag(tag) {
  return notes.filter((n) => n.tags.includes(tag));
}
S
cat > ws/projects/notes-app/workflows.md <<'W'
# Workflows

## Standard
applies when: the title has tag
review rounds: 2

| # | step | kind | who | on reject, back to |
|---|---|---|---|---|
| 1 | Build | run | grid-builder |  |
| 2 | Review | approve | alex |  |
W
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
  - {step: review, kind: approve}
step: review
round: 2
status: running
updated: 2026-03-04T10:00
summary: Tag filter built and reviewed.
next: The reviewer's verdict is read.
---

- 4 Mar · build · tag filter written
- 4 Mar · review · the reviewer wrote its verdict
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
1. Make filterByTag in src/store.js match a tag whatever its case.
2. Add a test for it.
3. Document the tag filter in the README.

## Context
- projects/notes-app/src/store.js

## Out of scope
Any change to how notes are stored.

## Open
Nothing.

## Acceptance
A1. filterByTag("Home") finds the note tagged "home".
A2. A test covers the case-insensitive match.
A3. The README describes the tag filter.

## Outcome
filterByTag written in src/store.js.

## Verdict
CHANGES
- A1 src/store.js:7 compares tags with includes, so "Home" finds nothing.
- A2 no test file exists for filterByTag.
- A3 README.md has no line about the tag filter.

## Follow-up
None.
B
printf -- '- 2026-03-04T10:00 · created · by alex\n' > "$t/history.md"
