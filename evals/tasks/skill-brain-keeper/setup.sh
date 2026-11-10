# A Grid home with project notes-app, task T-01-sample and one accepted convention note, tag-storage.
set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects"
printf "1\n" > "$GRID_HOME/VERSION"
p="$GRID_HOME"/projects/notes-app
d="$p"/tasks/T-01-sample
mkdir -p "$d" "$p/knowledge/conventions" "$p/brain-proposals" work/notes-app/src
printf 'add a way to search the notes by tag\n' > "$d/intent.md"
cat > "$d/task.md" <<'X'
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
next: Build the tag search.
---

- 4 Mar · build · task created
X
printf -- '- 2026-03-04T10:00 · created · by alex\n' > "$d/history.md"
cat > "$p/knowledge/conventions/tag-storage.md" <<'X'
---
name: tag-storage
topic: conventions
type: reference
description: How notes store their tags; read before changing any tag code
summary: Tags are a string array on each note, kept in work/notes-app/src/store.js.
updated: 2026-03-01T10:00:00Z
source: T-00
superseded_by: ""
links: []
level: project
status: accepted
reason: ""
origin:
  kind: job
  id: seed-1
  tier: agent
---
Each note has a `tags` array of strings. Code that reads tags reads that array.
X
cat > work/notes-app/src/store.js <<'X'
export const notes = [];
export function filterByTag(list, tag) {
  return list.filter((n) => n.tags.map((t) => t.toLowerCase()).includes(tag.toLowerCase()));
}
X
