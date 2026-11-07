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

Orchestrator: sees `exited` on the Job in the fold and, with no result, reassigns the Job or parks it with a reason.
