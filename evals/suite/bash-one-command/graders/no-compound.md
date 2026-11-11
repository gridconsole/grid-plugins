---
type: tool_used
tool: Bash
input_match: '&&|;|\||\$\('
min: 0
max: 0
---
Every Bash call is one command: no `;`, no `&&`, no pipe and no `$(...)`. A
compound command is refused by the sandbox rules and costs a retry. The run
passes when no Bash input matches the pattern.
