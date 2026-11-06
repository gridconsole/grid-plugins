---
type: regex
pattern: '^(?:phase|branch|worktree|runner|gate_raised|paused):'
flags: m
match: not_contains
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-feedback/task.md
---
