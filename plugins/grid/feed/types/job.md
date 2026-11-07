---
type: job
version: 1
who: orchestrator
required: [title]
fields:
  - title: one line saying what the Job is
  - brief: path of the brief file in the task folder, or the instructions in a few lines
  - role: the Worker role that should run it (scout, researcher, builder, runner, reviewer, designer, checker, operator)
  - repo: the repo the Job works in, when it works in one
  - after: ids of Jobs that must be done first
---
One piece of work that one Worker picks up. The envelope `job` is its id (J-1, J-2, ...); a Job post without one gets the next number.

Orchestrator: posts it, then moves it with control posts: ready when nothing blocks it, assigned to a Worker and Host, and on from there. A Job nobody moved stays posted.
