---
type: regex
pattern: '^(?:runner|gate_raised|paused|lane|lane_step|lane_round):'
flags: m
match: not_contains
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-release-note/task.md
---
