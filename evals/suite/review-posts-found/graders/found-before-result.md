---
type: tool_order
before: {tool: Bash, input_match: '--type found'}
after: {tool: Bash, input_match: 'grid job result'}
---

The reviewer posts the `found` record before it posts its result with `grid job result`.
