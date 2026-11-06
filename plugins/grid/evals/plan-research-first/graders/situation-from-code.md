---
type: llm
focus:
  source: file
  path: ws/projects/notes-app/tasks/T-01-tags/plan.md
---
PASS if `## Where things stand now` states facts that are true of the code: notes keep a `tags` array in src/store.js, tags differ in case ("Work" and "home"), and the list in src/list.js renders every note with nothing filtering it; at least two of these, each with a source (a file and line or a research.md section). Nothing it states may contradict the code.
FAIL if the section is missing, invents a fact the code does not show, or gives no sources.
