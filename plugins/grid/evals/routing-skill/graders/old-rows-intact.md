---
type: regex
pattern: '\| kind: run, project: notes-app \| grid-builder \| Notes-app builds go to a worker \|\n\| kind: check \| grid-runner \| Checks are commands \|\n\| project: notes-app \| grid-reviewer \| Fallback for other notes-app steps \|'
target:
  source: file
  path: ws/projects/notes-app/routing.md
---
