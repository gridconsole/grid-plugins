# A Grid home at GRID_HOME with a Large task, T-01-sample, at its build step and no spec.md.
set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects"
printf "1\n" > "$GRID_HOME/VERSION"
d="$GRID_HOME"/projects/notes-app/tasks/T-01-sample
mkdir -p "$d"
printf 'add a way to search the notes by tag\n' > "$d/intent.md"
cat > "$d/task.md" <<'X'
---
id: T-01
title: Search notes by tag
project: notes-app
owner: alex
source: idea
size: large
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
