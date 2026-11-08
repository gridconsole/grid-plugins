---
type: control
version: 1
who: orchestrator
required: [target]
fields:
  - target: a Job id, or the id of a post (a question, a result, a submit)
  - status: posted, ready, assigned, running, submitted, integrated, done, stale, parked, rejected, conflict or cancelled
  - assign: the Worker and Host as an object, {worker, host}
  - priority: high, normal or low (a whole number from older posts, higher first); the latest wins
  - after: ids of Jobs this one waits on; on a hold post (see reason) the Jobs the reason names
  - base: the task head commit the Job starts from
  - lease: the time the Worker's lease runs out, an ISO time; without one an assigned or running Job gets 10 minutes
  - attempt: the attempt number, 1 for the first
  - verdict: accept or disagree, on a result or submit
  - reason: why, in one line. With no status and one of dependency, worker_cap, needs_person, question, host_offline, conflict_hold, starting or orchestrator_stopped it says why an unclaimed Job waits, and then `after` (Job ids), `question` (a question id) and `blocker` (text) may go with it
---
Moves a Job or closes a question. A control post counts only when it comes from the Orchestrator of the epoch current at its place in the Feed, read in (at, id) order: before any handover, any Orchestrator at epoch 0; after a handover that counts (see handover.md), only the member@host that posted it, with its epoch. Any other is kept, shown and ignored with a reason.

Orchestrator: is the only one who posts it. Every move of a Job is a control post with a reason, so the Job's history reads as a log of decisions.

A hold post moves nothing; the Orchestrator writes one only when the reason of a Job changed, and any move of the Job clears it.
