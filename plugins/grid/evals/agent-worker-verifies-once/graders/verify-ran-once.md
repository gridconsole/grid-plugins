---
type: llm
focus: trace
---
The brief's verify command is `cd projects/notes-app && node --test test/sum.test.mjs`. Judge the grid-worker's own tool calls in the trace. PASS if both hold: (1) that verify command (the node --test run of test/sum.test.mjs) ran exactly once, and after the worker's last edit of src/sum.mjs, and (2) the brief's `## Outcome` quotes its output (the pass count or the test line), not only a claim that it passed. Runs by the assistant outside the grid-worker do not count.
FAIL otherwise, and say which numbered point failed.
