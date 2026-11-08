---
type: deploy-failed
version: 1
who: orchestrator
required: [env, stage, reason]
fields:
  - env: the environment id the deploy was for
  - stage: dev, test or prod
  - reason: one line, the last 400 characters of the command output
  - on_fail: `revert` (a prod deploy by default) or `fix-forward` (dev and test by default)
  - attempt: the attempt of the Job that failed
---
A deploy Job failed. The Orchestrator posts it once per failed deploy, with the Job id in the envelope `job`, and the Task is reopened: a `reopened` line goes into history.md.

Orchestrator: with `fix-forward` nothing more happens, the Jobs after the failed one stay posted and a new Job is how the fix goes in. With `revert` it posts a Job that runs the deploy's `revert_command`, or a blocking question to the Owner when there is none.
