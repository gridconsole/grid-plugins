---
type: regex
pattern: 'preview approved|plan accepted|ship approved|phase set'
match: not_contains
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-sample/history.md
---
