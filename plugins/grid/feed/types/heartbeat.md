---
type: heartbeat
version: 1
who: worker
required: []
fields:
  - note: what the Worker is doing, a few words
---
A Worker is alive. Hooks post it after tool calls and when the Worker stops; the post carries `job` and `worker`.

Orchestrator: the fold extends the lease of the Job on each beat from its assigned Worker. A Job whose lease ran out shows as stale and the Orchestrator reassigns it or waits.
