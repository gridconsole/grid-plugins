#!/usr/bin/env bash
# A Grid workspace folder ./ws whose notes-app project has a routing.md with two same-rank rows for make-together steps.
set -e
mkdir -p ws/projects/notes-app
cat > ws/projects/notes-app/routing.md <<'R'
# Routing · project notes-app

| Topic | Goes to | Rule | Why |
|---|---|---|---|
| Release wording | sam | Asks before it ships | Sam owns the changelog |

## Members

- alex · owner
- sam · reviewer

## Agents

| Applies to | Agent | Why |
|---|---|---|
| kind: make-together, project: notes-app | grid-designer | Shared design sessions |
| kind: make-together, project: notes-app | grid-builder | Shared build sessions |
| kind: check | grid-runner | Checks are commands |
R
