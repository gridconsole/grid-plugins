---
type: llm
focus: trace
---
PASS if the assistant told the person that task T-01 is closed and stopped without delivering: it did not copy a file into artifacts/, did not set an outcome, did not add a delivered line and did not reopen the task. A question to the person about reopening the task is fine.
FAIL otherwise.
