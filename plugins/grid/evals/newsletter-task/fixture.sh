#!/usr/bin/env bash
# A Grid workspace folder ./ws that already holds task T-01 in the club-news project, so the next id is T-02.
set -e
d=ws/projects/club-news/tasks/T-01-february
mkdir -p "$d"
printf 'the February newsletter\n' > "$d/intent.md"
cat > "$d/task.md" <<'T'
---
id: T-01
title: February newsletter
project: club-news
owner: sam
source: idea
size: small
workflow:
  - {step: draft, kind: make-together}
  - {step: send, kind: schedule}
step: send
status: closed
updated: 2026-02-06T09:00
summary: The February members newsletter.
next: Nothing; it was sent.
---

- 6 Feb · send · sent
T
printf -- '- 2026-02-02T10:00 · created · by sam\n' > "$d/history.md"
