# A Grid home with two accepted notes: one decision about tags (the question says "labels"), one unrelated.
# The agent command copies agent-templates/grid-library.md to .claude/agents/ before claude starts (see the lane report).
set -eu
: "${GRID_HOME:?}"
mkdir -p .claude/agents "$GRID_HOME/knowledge/decisions"
printf "1\n" > "$GRID_HOME/VERSION"
# claude -p cannot ask for approval, so it refuses every Bash call. Allow only the read-only brain verbs the template names.
# Claude Code ignores this file in a workspace it has not been told to trust (the stored run says so on stderr), so the
# agent command must also pass the same list with --allowedTools; the lane report gives the command.
cat > .claude/settings.json <<'T'
{"permissions":{"allow":["Bash(grid brain index --read-only:*)","Bash(grid brain topics:*)","Bash(grid brain search:*)","Bash(grid brain show:*)"]}}
T
cat > "$GRID_HOME/knowledge/decisions/tag-storage.md" <<'T'
---
name: tag-storage
topic: decisions
type: reference
description: Read before changing how tags are stored on a note.
summary: Tags are stored as a lowercase array on each note, never in a separate table.
updated: '2026-03-04T10:00:00Z'
source: task:T-01
superseded_by: ''
links: []
level: home
status: accepted
reason: ''
origin:
  kind: session
  id: 009aeae78436f5cf7e2ce0eb
  tier: agent
---
# Tag storage

Tags are a lowercase array on the note. A separate table was rejected because notes are read whole.
T
cat > "$GRID_HOME/knowledge/decisions/export-format.md" <<'T'
---
name: export-format
topic: decisions
type: reference
description: Read before changing the CSV export of notes.
summary: The CSV export separates columns with semicolons.
updated: '2026-03-05T10:00:00Z'
source: task:T-02
superseded_by: ''
links: []
level: home
status: accepted
reason: ''
origin:
  kind: session
  id: 119aeae78436f5cf7e2ce0ec
  tier: agent
---
# Export format

The CSV export separates columns with semicolons because the target spreadsheet reads them.
T
