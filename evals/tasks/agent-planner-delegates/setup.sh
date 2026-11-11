set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects"
printf "1\n" > "$GRID_HOME/VERSION"
d="$GRID_HOME"/projects/notes-app/tasks/T-01-sample
mkdir -p "$d" work/notes-app/src
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
step: build
status: running
updated: 2026-03-04T10:00
summary: Let people filter the note list by tag.
next: Build the tag filter.
---

- 4 Mar · plan · plan accepted
T
cat > "$d/plan.md" <<'T'
# Plan, rev 1
- [ ] 1 Add filterByTag(list, tag) to work/notes-app/src/store.js, case-insensitive
  needs: none

Verify: `node --test` passes
T
printf -- '- 2026-03-04T09:00 · created · by alex\n' > "$d/history.md"
cat > work/notes-app/src/store.js <<'T'
export const notes = [];
export function addNote(note) { notes.push(note); }
T
