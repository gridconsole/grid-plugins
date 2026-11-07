---
type: control
version: 1
who: orchestrator
required: [target]
fields:
  - target: a Job id, or the id of a post (a question, a result, a submit)
  - status: posted, ready, assigned, running, submitted, integrated, done, stale, parked, rejected, conflict or cancelled
  - assign: the Worker and Host as an object, {worker, host}
  - priority: a number, higher first
  - after: ids of Jobs this one waits on
  - base: the task head commit the Job starts from
  - lease: the time the Worker's lease runs out, an ISO time; without one an assigned or running Job gets 10 minutes
  - attempt: the attempt number, 1 for the first
  - verdict: accept or disagree, on a result or submit
  - reason: why, in one line
---
Moves a Job or closes a question. Only a control post from the Orchestrator of the current epoch counts; any other is kept and shown and ignored.

Orchestrator: is the only one who posts it. Every move of a Job is a control post with a reason, so the Job's history reads as a log of decisions.
