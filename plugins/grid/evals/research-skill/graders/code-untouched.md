---
type: regex
pattern: 'export function (?!listNotes)'
match: not_contains
target:
  source: file
  path: ws/projects/notes-app/src/store.js
---
