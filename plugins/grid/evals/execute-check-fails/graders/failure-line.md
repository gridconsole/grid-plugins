---
type: regex
pattern: '^- \d+ \w+ · verify · .*(?:fail|exit 1|not pass|did not pass)'
flags: mi
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-tag-filter/task.md
---
