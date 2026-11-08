---
type: handover
version: 1
who: orchestrator
required: []
fields:
  - from: the Orchestrator that handed over, as member@host
  - reason: why the task changes hands
  - epoch: the new epoch; without one the next number is used
  - via: transfer, takeover, or absent (a plain handover)
---
A new Orchestrator takes the task. Only a handover posted by an Orchestrator counts, and only when its epoch is the highest so far in (at, id) order; on a tie the later one counts, and a handover with a lower epoch is kept and ignored with a reason. Moves made before it keep their states; from it on, control posts count only from the member@host that posted it and with its epoch, and any other is kept and ignored with a reason.

`via: transfer` counts only when an earlier transfer post of the current Owner names the author (member@host); otherwise it is ignored with "no transfer to <who> from the Owner". `via: takeover` counts only when the Owner has written no post of any type for 15 minutes before its `at`; otherwise it is ignored with "the Owner <who> was seen <n> min ago, take-over opens after 15". Time is read from the posts' `at`, never the clock. Nothing takes over by itself: a person's click posts it (feed_handover).

Orchestrator: the new one posts it first, then reads the fold and carries on from the Jobs' current states.
