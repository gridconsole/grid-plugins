#!/usr/bin/env bash
# A Grid workspace folder ./ws whose notes-app project has a routing.md with a topic table, Members and an Agents table.
set -e
mkdir -p ws/projects/notes-app
cat > ws/projects/notes-app/routing.md <<'R'
# Routing · project notes-app

| Topic | Goes to | Rule | Why |
|---|---|---|---|
| Release wording | sam | Asks before it ships | Sam owns the changelog |
| Storage choices | alex | Decides | Alex owns the data layer |

## Members

- alex · owner
- sam · reviewer

## Agents

| Applies to | Agent | Why |
|---|---|---|
| kind: run, project: notes-app | grid-worker | Notes-app builds go to a worker |
| kind: check | grid-runner | Checks are commands |
| project: notes-app | grid-reviewer | Fallback for other notes-app steps |
R
