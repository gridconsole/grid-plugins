---
type: transfer
version: 1
who: orchestrator
required: [to]
fields:
  - to: the teammate who may take the task over, as member@host
  - reason: why the task changes hands
---
The Owner names the teammate who takes the task next. It does nothing alone: the epoch changes only when that teammate posts a handover with `via: transfer`, and only a transfer of the current epoch's Owner counts.

Orchestrator: post it with feed_handover (action transfer), then keep working until the handover appears; running Jobs keep their states.
