# A Grid home at GRID_HOME with task T-01 running and no questions; a project mock that lists recent tags.
set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects"
printf "1\n" > "$GRID_HOME/VERSION"
t="$GRID_HOME"/projects/notes-app/tasks/T-01-tag-box
mkdir -p work/notes-app/docs "$t"
printf -- '---\n---\n' > "$GRID_HOME/projects/notes-app/project.md"
printf 'The mock lists the last five tags under the tag box.\n' > work/notes-app/docs/mock.md
printf 'add a tag box to the note form\n' > "$t/intent.md"
cat > "$t/task.md" <<'T'
---
id: T-01
title: Tag box on the note form
project: notes-app
owner: alex
source: idea
size: small
workflow:
  - {step: build, kind: run}
step: build
state: running
created: 2026-03-04T09:00:00Z
updated: 2026-03-04T10:00
summary: Add a tag box to the note form.
next: Build the tag box.
outcome: —
---

- 4 Mar · build · started
T
printf -- '- 2026-03-04T09:00 · created · by alex\n' > "$t/history.md"
