---
type: tool_used
tool: Bash
input_match: grid job show
min: 1
---

The planner reads the cancelled Job and why it stopped with `grid job show <job> --task <task>` before it decides for the Jobs that wait on it (skills/plan-jobs/SKILL.md, Re-plan Job, step 1).
