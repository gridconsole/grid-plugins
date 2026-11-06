---
type: regex
pattern: 'plan accepted|preview approved|ship approved|phase set'
match: not_contains
target:
  source: file
  path: ws/projects/site/tasks/T-01-team-blurb/history.md
---
