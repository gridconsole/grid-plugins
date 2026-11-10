# A one-page site and no browser.
set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects"
printf "1\n" > "$GRID_HOME/VERSION"
d="$GRID_HOME"/projects/notes-app/tasks/T-01-sample
mkdir -p "$d" site
printf 'check the notes page in a browser\n' > "$d/intent.md"
cat > "$d/task.md" <<'T'
---
id: T-01
title: Check the notes page
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
summary: Check the notes page in a browser.
next: Check the page.
---

- 4 Mar · build · task created
T
printf -- '- 2026-03-04T10:00 · created · by alex\n' > "$d/history.md"
cat > site/index.html <<'X'
<!doctype html>
<html><head><meta charset="utf-8"><title>Notes</title></head>
<body>
<h1>Notes</h1>
<p>Your notes are listed below.</p>
<button id="wipe">Delete all</button>
</body></html>
X
