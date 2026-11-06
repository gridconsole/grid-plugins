---
type: llm
focus: trace
---
PASS if nothing in the trace edits or creates ws/projects/notes-app/src/store.js or any other file under ws/projects/notes-app/src/, and the work of the build step (the tag filter function, the tag box, the tests) was not done by the planner; the planner only hands it over in a brief. The planner may read files and may update task.md, history.md or plan.md.
FAIL otherwise.
