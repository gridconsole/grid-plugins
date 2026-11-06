---
type: regex
pattern: '^\{"type":"user","cwd":"/work/notes-app","timestamp":"2026-03-02T09:00:00Z"[\s\S]*end-of-claude-session"\}\}\n$'
target:
  source: file
  path: ws/history/claude-listed.jsonl
---
