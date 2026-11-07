---
type: result
version: 1
who: worker
required: [outcome]
fields:
  - outcome: what happened, in a few lines
  - changed: what changed, as files or things
  - evidence: how it was checked, with the output that proves it
  - next: what the Worker thinks should come next
  - knowledge: anything the brain should keep, in a few lines
---
A Worker's report when its Job is done. The Worker posts it with `job` set, then closes.

Orchestrator: reads it against the Job, checks the evidence, turns `next` into new Job posts, passes `knowledge` on to the Brain agent, then posts a control post (submitted, rejected or done).
