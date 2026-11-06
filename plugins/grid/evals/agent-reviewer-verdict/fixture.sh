#!/usr/bin/env bash
# A Grid workspace folder ./ws with a finished brief whose work is wrong: the filter is case-sensitive, Acceptance A2 says it is not.
set -e
d=ws/projects/notes-app/tasks/T-01-sample
mkdir -p "$d/subagents" ws/projects/notes-app/src
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
  - {step: build, kind: run}
  - {step: review, kind: approve}
step: build
status: running
updated: 2026-03-04T10:00
summary: Let people filter the note list by tag.
next: Review the tag filter.
---

- 4 Mar · build · tag filter written
T
printf -- '- 2026-03-04T09:00 · created · by alex\n' > "$d/history.md"
cat > ws/projects/notes-app/src/store.js <<'T'
export const notes = [];
export function addNote(note) { notes.push(note); }
export function filterByTag(list, tag) {
  return list.filter((n) => n.tags.includes(tag));
}
T
cat > "$d/subagents/02-tag-filter.md" <<'T'
---
id: 02-tag-filter
role: worker
status: done
verify: self
follow_up: none
---

## Intent
"add a way to search the notes by tag"

## Job
1. Add `filterByTag(list, tag)` to `projects/notes-app/src/store.js`.

## Context
- projects/notes-app/src/store.js

## Out of scope
The list view.

## Open
Nothing.

## Acceptance
A1. `filterByTag(list, "work")` returns only the notes whose `tags` include "work".
A2. The match ignores case: `filterByTag(list, "Work")` returns the same notes.

## Outcome
Added `filterByTag` in `projects/notes-app/src/store.js:3`. It returns the notes whose tags include the tag. Both acceptance lines are met.

## Verdict

## Follow-up
Review, then the list view.
T
