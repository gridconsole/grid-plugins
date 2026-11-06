---
type: regex
pattern: '^\{"timestamp":"2026-03-03T10:00:00Z","type":"session_meta"[\s\S]*end-of-codex-session"\}\]\}\}\n$'
target:
  source: file
  path: ws/history/codex-listed.jsonl
---
