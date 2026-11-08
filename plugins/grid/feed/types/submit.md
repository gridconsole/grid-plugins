---
type: submit
version: 1
who: worker
required: [sha]
fields:
  - sha: the commit the Job produced
  - ref: the ref it was pushed to
  - summary: one line saying what the commit holds; `nothing new: the task head already holds the work` on an empty submission
---
A Worker hands in the code of a Job: the commit it made and where it pushed it. It goes with the result.

An empty submission is a clean worktree whose base is the task head: the Worker hands in the head itself, and integrating it moves nothing.

Orchestrator: integrates the commit onto the task head (a head post says so) or posts a control post saying conflict or rejected with the reason.
