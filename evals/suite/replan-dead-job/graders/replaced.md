---
name: replaced
type: tool_used
tool: Bash
input_match: grid job post
min: 1
---

The planner posts a replacement for the cancelled Job with `grid job post --task <task> --body <json>` (Re-plan Job, step 3), so the waiter whose plan step is still open can run.
