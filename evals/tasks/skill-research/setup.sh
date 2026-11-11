# A Grid home at GRID_HOME with task T-01 at its research step, and a small notes-app project where tag validation lives on a known line.
set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects"
printf "1\n" > "$GRID_HOME/VERSION"
t="$GRID_HOME"/projects/notes-app/tasks/T-01-tag-rules
mkdir -p work/notes-app/src "$t"
printf -- '---\n---\n' > "$GRID_HOME/projects/notes-app/project.md"
cat > work/notes-app/src/store.js <<'S'
const notes = [];
const MAX_TAGS = 5;

export function listNotes() {
  return [...notes];
}

export function validateTag(tag) {
  return /^[a-z0-9-]{1,20}$/.test(tag);
}

export function addNote(note) {
  const tags = (note.tags || []).slice(0, MAX_TAGS);
  if (!tags.every(validateTag)) throw new Error("bad tag");
  notes.push({ ...note, tags });
}
S
printf 'find out how notes-app checks a tag before it is saved\n' > "$t/intent.md"
cat > "$t/task.md" <<'T'
---
id: T-01
title: How tags are checked
project: notes-app
owner: alex
source: idea
size: small
workflow:
  - {step: research, kind: run}
step: research
state: running
created: 2026-03-04T09:00:00Z
updated: 2026-03-04T10:00
summary: Find the tag rules before planning the tag search.
next: Research the tag rules.
outcome: —
---

- 4 Mar · research · started
T
printf -- '- 2026-03-04T09:00 · created · by alex\n' > "$t/history.md"
