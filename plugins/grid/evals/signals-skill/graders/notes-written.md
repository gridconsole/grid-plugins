---
type: regex
pattern: '^\{"via":"mail","ext":"msg-0001@example\.com","note":"\d{8}-\d{4}-[a-z0-9-]+\.md"\}$'
flags: m
target:
  source: file
  path: ws/projects/notes-app/.grid/seen.jsonl
---
