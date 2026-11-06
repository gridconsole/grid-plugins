---
type: llm
focus:
  source: file
  path: ws/projects/notes-app/tasks/T-01-sample/plan.md
---
PASS if most steps (all but at most one) have, under their `- [ ] <n> <title>` line and before their `verify:` line, a short indented line or two saying what the step does and why, and every step has a `verify:` line saying how its result is proved.
FAIL if the steps are titles with a verify line and nothing explaining them, a step's line only restates its title or gives no reason (why this step, and why at this point in the order), or a step has no verify line.
