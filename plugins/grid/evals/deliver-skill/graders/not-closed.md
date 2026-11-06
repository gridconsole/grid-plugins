---
type: regex
pattern: '^status: closed$'
flags: m
match: not_contains
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-release-note/task.md
---
