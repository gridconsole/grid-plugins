---
type: handover
version: 1
who: orchestrator
required: []
fields:
  - from: the Orchestrator that handed over, as member@host
  - reason: why the task changes hands
  - epoch: the new epoch; without one the next number is used
---
A new Orchestrator takes the task. Only a handover posted by an Orchestrator counts, and only when its epoch is the highest so far in (at, id) order; on a tie the later one counts, and a handover with a lower epoch is kept and ignored with a reason. Moves made before it keep their states; from it on, control posts count only from the member@host that posted it and with its epoch, and any other is kept and ignored with a reason.

Orchestrator: the new one posts it first, then reads the fold and carries on from the Jobs' current states.
