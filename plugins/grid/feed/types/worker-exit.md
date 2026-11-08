---
type: worker-exit
version: 1
who: worker
required: []
fields:
  - reason: why the Worker stopped, if it knows
  - code: the exit code or signal
---
A Worker's process ended. Hooks post it when the session stops or ends; if no result came first, the Job needs a decision.

Orchestrator: a worker-exit with no result marks the Job stale; the loop retries it once, then asks the Owner and parks it (see the feed skill).
