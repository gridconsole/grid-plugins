#!/usr/bin/env bash
# A Grid workspace folder ./ws with a worker brief for a one-line fix whose verify is a node test, in a tiny git repo.
set -e
d=ws/projects/notes-app/tasks/T-01-sample
mkdir -p "$d/subagents" "$d/artifacts" ws/projects/notes-app/src ws/projects/notes-app/test
printf 'sum adds the two numbers, it subtracts them now\n' > "$d/intent.md"
cat > "$d/task.md" <<'T'
---
id: T-01
title: Fix sum
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
summary: Make sum add its arguments.
next: Run the build step.
---

- 4 Mar · build · brief written
T
printf -- '- 2026-03-04T09:00 · created · by alex\n' > "$d/history.md"
cat > ws/projects/notes-app/src/sum.mjs <<'T'
export function sum(a, b) {
  return a - b;
}
T
cat > ws/projects/notes-app/test/sum.test.mjs <<'T'
import test from 'node:test';
import assert from 'node:assert/strict';
import { sum } from '../src/sum.mjs';

test('sum adds', () => {
  assert.equal(sum(2, 3), 5);
});
T
printf 'tasks/\n' > ws/projects/notes-app/.gitignore
git -C ws/projects/notes-app init -q
git -C ws/projects/notes-app add -A
git -C ws/projects/notes-app -c user.name=alex -c user.email=alex@example.com commit -q -m "Start the notes app"
cat > "$d/subagents/01-fix-sum.md" <<'T'
---
id: 01-fix-sum
role: worker
status: briefed
verify: cd projects/notes-app && node --test test/sum.test.mjs
follow_up: none
---

## Intent
"sum adds the two numbers, it subtracts them now"

## Job
1. In `projects/notes-app/src/sum.mjs`, make `sum(a, b)` return `a + b`.

## Context
- projects/notes-app/src/sum.mjs:1-3
- projects/notes-app/test/sum.test.mjs:1-8

## Out of scope
The test file and every other file.

## Open
Nothing.

## Acceptance
A1. `sum(2, 3)` returns 5.
A2. `cd projects/notes-app && node --test test/sum.test.mjs` passes.

## Outcome

## Verdict

## Follow-up
T
