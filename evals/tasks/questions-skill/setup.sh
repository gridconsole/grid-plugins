# A Grid home ./home with one task, T-01-sample, in the notes-app project.
set -eu
mkdir -p home
printf "1\n" > home/VERSION
d=home/projects/notes-app/tasks/T-01-sample
mkdir -p "$d"
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
  - {step: check, kind: check}
step: plan
status: running
updated: 2026-03-04T10:00
summary: Let people filter the note list by tag.
next: Write the plan.
---

- 4 Mar · plan · task created
T
printf -- '- 2026-03-04T10:00 · created · by alex\n' > "$d/history.md"
