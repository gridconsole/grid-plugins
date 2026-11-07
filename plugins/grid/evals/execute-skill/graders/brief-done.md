---
type: llm
focus: trace
---
PASS if a brief under ws/projects/notes-app/tasks/T-01-release-note/subagents/ has role designer and status done with a non-empty Outcome, and the assistant started grid-designer (the agent routing.md names for kind run, not grid-builder) through the Agent tool after the brief was written by grid-brief-writer (the assistant itself did not write the brief or the release note). Task.md moved from step draft to step verify only after that.
FAIL otherwise.
