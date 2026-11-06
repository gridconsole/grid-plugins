#!/usr/bin/env bash
# A Grid workspace folder ./ws with one task, T-01-sample, in the notes-app project.
set -e
d=ws/projects/notes-app/tasks/T-01-sample
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
mkdir -p ws/projects/notes-app/knowledge/people
cat > ws/projects/notes-app/knowledge/people/ilse.md <<'P'
---
name: ilse
type: person
description: Ilse knows the tag storage and answers questions about it.
membership: ilse
links:
  - {to: tag-storage, relation: knows}
---
Questions about tag storage go to Ilse.
P
printf -- '- name: ilse\n  type: person\n  path: people/ilse.md\n' > ws/projects/notes-app/knowledge/index.md
