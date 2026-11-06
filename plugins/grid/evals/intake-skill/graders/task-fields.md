---
type: llm
focus: trace
---
PASS if the task.md the assistant wrote (in the trace) starts with frontmatter that has id T-01, a title, project notes-app, owner alex, a source, a size that is small, normal or large, a workflow of 2 to 6 items each written {step: <name>, kind: <kind>} with a kind from check, ask, approve, make-together, run or schedule, a step that is one of the workflow's step names, a status, an updated time, a non-empty summary that reflects the customer email, and a non-empty next line. It must have no phase, branch or worktree field. The assistant must not have edited or deleted inputs/feedback.txt.
FAIL otherwise.
