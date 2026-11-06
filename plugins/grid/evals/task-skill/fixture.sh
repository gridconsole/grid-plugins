#!/usr/bin/env bash
# A Grid workspace folder ./ws that already holds task T-01 in the notes-app project, so the next id is T-02.
set -e
d=ws/projects/notes-app/tasks/T-01-sample
mkdir -p "$d"
printf 'first task\n' > "$d/intent.md"
cat > "$d/task.md" <<'T'
---
id: T-01
title: First task
project: notes-app
owner: alex
source: idea
size: small
workflow:
  - {step: plan, kind: approve}
  - {step: build, kind: run}
  - {step: check, kind: check}
step: build
status: running
updated: 2026-03-04T10:00
summary: An earlier task.
next: Build it.
---

- 4 Mar · build · started
T
printf -- '- 2026-03-04T10:00 · created · by alex\n' > "$d/history.md"
