---
type: regex
pattern: '^outcome: (?!—$)(?=.*release-note\.md)\S.*$'
flags: m
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-release-note/task.md
---
