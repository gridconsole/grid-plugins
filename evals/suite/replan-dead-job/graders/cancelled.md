---
type: tool_used
tool: Bash
input_match: grid job cancel
min: 1
---

The planner cancels the waiter whose plan step was dropped with `grid job cancel --task <task> --job <job> --reason <why>` (Re-plan Job, step 4). The cancelled Job itself is already dead and is not the one to cancel.
