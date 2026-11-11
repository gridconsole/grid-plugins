set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects"
printf "1\n" > "$GRID_HOME/VERSION"
d="$GRID_HOME"/projects/notes-app/tasks/T-01-sample
mkdir -p "$d" work/notes-app/vendor/fastcache
printf 'cache the note list\n' > "$d/intent.md"
cat > "$d/task.md" <<'T'
---
id: T-01
title: Cache the note list
project: notes-app
owner: alex
source: idea
size: normal
workflow:
  - {step: research, kind: run}
step: research
status: running
updated: 2026-03-04T10:00
summary: Cache the note list.
next: Research fastcache.
---

- 4 Mar · research · started
T
printf -- '- 2026-03-04T09:00 · created · by alex\n' > "$d/history.md"
printf '{ "name": "notes-app", "dependencies": { "fastcache": "2.4.1" } }\n' > work/notes-app/package.json
cat > work/notes-app/vendor/fastcache/README.md <<'T'
# fastcache 2.4.1

A small in-memory cache.

## Options

- `max`: how many entries the cache keeps.
- `ttlSeconds`: how long an entry lives. Default 90 seconds.
- `clock`: a function that returns the time.
T
