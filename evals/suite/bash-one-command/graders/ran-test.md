---
type: tool_used
tool: Bash
input_match: 'go test'
min: 1
---
The Job asks for the tests of the `calc` folder, so the agent must have run
`go test` at least once. This keeps the two graders above honest: a run that
did nothing would pass them and must not pass the case.
