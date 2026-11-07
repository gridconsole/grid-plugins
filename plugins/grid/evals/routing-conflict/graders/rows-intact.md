---
type: regex
pattern: '\| kind: make-together, project: notes-app \| grid-designer \| Shared design sessions \|\n\| kind: make-together, project: notes-app \| grid-builder \| Shared build sessions \|\n\| kind: check \| grid-runner \| Checks are commands \|\n?$'
target:
  source: file
  path: ws/projects/notes-app/routing.md
---
