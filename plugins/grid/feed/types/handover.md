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
A new Orchestrator takes the task. Its epoch becomes the one that counts: control posts of an earlier epoch are kept and ignored. Only a handover posted by an Orchestrator counts.

Orchestrator: the new one posts it first, then reads the fold and carries on from the Jobs' current states.
