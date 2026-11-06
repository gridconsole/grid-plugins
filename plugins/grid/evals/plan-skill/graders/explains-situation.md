---
type: llm
focus:
  source: file
  path: ws/projects/notes-app/tasks/T-01-sample/plan.md
---
PASS if plan.md, before its steps, has a `## Where things stand now` section that tells a reader who has not opened the code how notes and their tags work today (for example that notes carry tags in src/store.js and the list shows every note), and gives a source for each finding: a file and line, a research.md section, or a link.
FAIL if that section is missing, only restates the request, or gives no source.
