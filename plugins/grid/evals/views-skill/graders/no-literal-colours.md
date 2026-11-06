---
type: regex
pattern: ':\s*#[0-9a-fA-F]{3,8}\b|rgba?\('
match: not_contains
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-sample/views/progress.html
---
