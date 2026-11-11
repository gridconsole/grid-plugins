# A task with an intent, a design system README and a read shape.
# The agent command copies agent-templates/grid-designer.md to .claude/agents/ before claude starts (see the report's run command).
set -eu
mkdir -p .claude/agents task design
printf 'show my notes: a list of them, and a page for one note\n' > task/intent.md
cat > design/README.md <<'T'
# Notes design system
Components: Card, List, Tag. Tokens: --space-2, --space-4, --color-text, --color-muted.
Both themes (light and dark) come from the tokens; never use a raw colour.
T
cat > design/model.md <<'T'
# notes.list answers
Each item has: id, title, qty (number), updated (date), tags (list of names).
# notes.get answers
One item with the same fields and body (Markdown text). There is no author field.
T
cat > job.md <<'T'
## Job
Write the design spec (task/spec.md) for the notes screen: a list page and a note page behind it.
Use design/README.md and the read shapes in design/model.md.

## Acceptance
A1. spec.md has one numbered template per page type.
A2. Every block names its data source and says what it shows when the data is absent.
A3. The spec states how the screen is checked at three widths, and covers both themes.
T
