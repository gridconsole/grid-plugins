---
type: llm
focus: trace
---
PASS if, in this one run, the assistant ran the check (grid-runner), saw it fail, set the task back to the build step (round 2 or higher, reworking) and then wrote a brief for the build step and started a worker on it, with no message to the person between the failed check and the build starting. Nothing the assistant wrote asks the person whether to carry on, says the build is ready to start, or ends the turn right after the failed check.
FAIL if grid-reviewer is started before the failed check is sent back, if the run ends after recording the failed check without having started the build, or asks the person for permission to continue.
