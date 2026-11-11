---
name: found-before-result
type: tool_order
before_tool: Bash
before_match: --type found
after_tool: Bash
after_match: grid job result
---

The reviewer posts the `found` record before it posts its result with `grid job result`.
