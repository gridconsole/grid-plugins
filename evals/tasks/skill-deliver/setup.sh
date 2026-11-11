# A Grid home at GRID_HOME with task T-01: an accepted two-item plan, no proof yet, and a lesson the task taught in its history.
set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects"
printf "1\n" > "$GRID_HOME/VERSION"
t="$GRID_HOME"/projects/notes-app/tasks/T-01-release-note
mkdir -p work/notes-app/docs "$t"
cat > work/notes-app/docs/release-note.md <<'N'
Notes can now be searched by tag.
Type a tag in the search box to filter the list.
N
printf 'write a two-line release note for the new tag search\n' > "$t/intent.md"
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
  - {step: ship, kind: approve}
step: draft
status: running
updated: 2026-03-04T10:00
summary: Release note drafted.
next: Deliver the note.
outcome: —
---

- 4 Mar · draft · note written
T
cat > "$t/plan.md" <<'P'
# Plan 1
- [x] 1 The release note has exactly two lines
  proof: check
  jobs: []
- [x] 2 The release note names the tag search
  proof: check
  jobs: []
P
{
  printf -- '- 2026-03-04T09:00 · created · by alex\n'
  printf -- '- 2026-03-04T09:05 · plan accepted · 000000 · by alex\n'
  printf -- '- 2026-03-04T10:30 · note · Sam said release notes always say "tag" and never "label"; the first draft said label and was corrected · by alex\n'
} > "$t/history.md"
