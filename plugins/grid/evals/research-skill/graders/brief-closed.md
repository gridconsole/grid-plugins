---
type: llm
focus: trace
---
PASS if a brief under ws/projects/notes-app/tasks/T-01-storage/subagents/ ends with status done (or blocked with a reason) and its Outcome section is not empty, and the main assistant did not write the research.md content itself instead of the grid-researcher agent. The assistant may set the brief status and task.md updated, next and a body line.
FAIL otherwise.
