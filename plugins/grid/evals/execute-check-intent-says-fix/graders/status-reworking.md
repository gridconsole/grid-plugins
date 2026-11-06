---
type: regex
pattern: '^status: (?:reworking|running|waiting|stuck)$'
flags: m
target:
  source: file
  path: ws/projects/site/tasks/T-01-team-blurb/task.md
---
