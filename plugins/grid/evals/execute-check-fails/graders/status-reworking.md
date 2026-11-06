---
type: regex
pattern: '^status: (?:reworking|running|waiting|stuck)$'
flags: m
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-tag-filter/task.md
---
