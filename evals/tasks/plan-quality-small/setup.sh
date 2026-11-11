# A Grid home with a small task T-02 (a one-file change) that has no plan yet, and a tiny notes-app repo beside it.
set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects"
printf "1\n" > "$GRID_HOME/VERSION"
t="$GRID_HOME"/projects/notes-app/tasks/T-02-fix-title
mkdir -p work/notes-app/src "$t"
cat > work/notes-app/src/title.js <<'S'
export function pageTitle(name) {
  return "Notes - " + name.trim();
}
S
cat > "$t/intent.md" <<'I'
the page title shows two spaces when the note name is empty, make it show just Notes
I
cat > "$t/task.md" <<'T'
---
id: T-02
title: Fix the empty page title
project: notes-app
owner: alex
source: idea
size: small
workflow:
  - {step: plan, kind: ask}
  - {step: build, kind: run}
step: plan
status: running
updated: 2026-03-04T10:00
summary: Show just Notes as the page title when the note name is empty.
next: Write the plan.
---

- 4 Mar · plan · started
T
printf -- '- 2026-03-04T10:00 · created · by alex\n' > "$t/history.md"
