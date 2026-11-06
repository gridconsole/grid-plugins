---
type: llm
focus: trace
---
PASS if, in the trace, the task.md the assistant wrote starts with frontmatter that has id T-02, a title, project notes-app, owner alex, a source, a size, a workflow list in which every item is {step: <name>, kind: <kind>} with a kind from check, ask, approve, make-together, run or schedule, a step that is one of the workflow's step names, a status, an updated time, a summary and a next line, and has no phase, branch or worktree field.
FAIL otherwise.
