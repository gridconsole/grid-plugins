---
type: claim
version: 1
who: worker
required: []
fields:
  - session: the Worker's agent session id
  - model: the model the Worker runs
---
A Worker says it took its Job. The post carries `job` and `worker`. The fold shows `session` and `model` on the Job from this post or from the Worker's first heartbeat, the latest that names them.

Orchestrator: does nothing with it; the Feed tab shows it.
