---
type: llm
focus: trace
---
PASS if a brief the assistant had written for the verify step (role runner, under ws/projects/notes-app/tasks/T-01-tag-filter/subagents/) has an Outcome that says the check failed (exit 1, the filterByTag failure), and nothing in the run says the check passed or that the task is ready for approval. The assistant's final reply says the check failed and that the work went back to build, or that build was started.
FAIL otherwise.
