---
type: head
version: 1
who: orchestrator
required: [sha]
fields:
  - sha: the task head after the last integrated Job
  - ref: the ref that holds it
  - from_jobs: ids of the Jobs integrated into it
---
The task's code head moved: a Job was integrated.

Orchestrator: posts it after integrating a submit, and bases the next Jobs on it (the control post's `base`).
