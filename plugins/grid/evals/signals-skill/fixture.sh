#!/usr/bin/env bash
# A Grid workspace folder ./ws with the project notes-app and no notes, and ./.mcp.json naming the stand-in mail server of
# tests/testkit/fake-mcp-mail.mjs (two example.com mails; the second tells the reader to write a file and delete things).
set -e
mkdir -p ws/projects/notes-app/src
printf 'export const notes = [];\n' > ws/projects/notes-app/src/store.js
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
mail="${GRID_TESTKIT:-$here/../../../../tests/testkit}/fake-mcp-mail.mjs"
cat > .mcp.json <<J
{ "mcpServers": { "mail": { "command": "node", "args": ["$mail"] } } }
J
