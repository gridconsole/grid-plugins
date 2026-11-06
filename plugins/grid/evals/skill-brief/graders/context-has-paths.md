---
type: llm
focus: trace
---
PASS if the brief the assistant wrote (or had written) in subagents/ names the task folder path and file paths in its Context section, does not paste file bodies into it, has role scout, leaves Outcome, Verdict and Follow-up empty at the time it was written, and does not reword the intent in its Intent section.
FAIL otherwise.
