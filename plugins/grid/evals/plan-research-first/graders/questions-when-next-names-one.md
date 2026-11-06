---
type: llm
focus: trace
---
Look at the last version of ws/projects/notes-app/tasks/T-01-tags/task.md written in the trace (by Write, Edit or Bash). PASS if its `next` line names no question id (Q-1, Q-2, …), or if every question id it names was also written as a block (`## Q-<n> · to: <person> · …`) into a file under ws/projects/notes-app/tasks/T-01-tags/questions/.
FAIL if `next` names a question id that no file under questions/ holds.
