---
type: llm
focus:
  source: file
  path: ws/projects/notes-app/tasks/T-01-sample/plan.md
---
The task's size is normal. PASS if plan.md has the sections `## Where things stand now`, `## Approach and options`, `## Risks` and `## Steps`, in that order, with 3 to 9 numbered steps under `## Steps`, and its last line starts with `Verify:`.
FAIL if a section is missing or out of order, there are fewer than 3 or more than 9 steps, the steps are split into chapters, or anything (assumptions, open questions) follows the `Verify:` line.
