---
type: regex
pattern: '^From: a customer\nSubject: exporting notes\n\nHi, I keep my recipes in the notes app[\s\S]*Thanks, Maria\n$'
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-feedback/inputs/feedback.txt
---
