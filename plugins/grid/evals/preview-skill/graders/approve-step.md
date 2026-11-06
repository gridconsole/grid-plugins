---
type: regex
pattern: '^\s+- \{step: preview, kind: approve\}[ \t]*\n(?:\s+- \{[^\n]*\n)*?\s+- \{step: build, kind: run\}'
flags: m
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-sample/task.md
---
