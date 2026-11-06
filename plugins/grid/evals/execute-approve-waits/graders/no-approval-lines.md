---
type: regex
pattern: 'plan accepted|preview approved|ship approved|phase set'
match: not_contains
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-release-note/history.md
---
