---
type: llm
focus:
  source: file
  path: ws/projects/notes-app/tasks/T-01-sample/plan.md
---
PASS if the `## Risks` section lists at least one risk, each as a bullet with a level (high, medium or low), the risk itself, and the number of the step that retires it, and each named step exists in the plan.
FAIL if a risk has no level, names no retiring step, or names a step the plan does not have.
