---
type: regex
pattern: '^needs: \[[^\]]*Q-'
flags: m
match: not_contains
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-tag-filter/task.md
---
