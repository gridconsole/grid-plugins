---
type: llm
focus:
  source: file
  path: ws/projects/site/tasks/T-01-team-blurb/subagents/02-links.md
---
This file is the brief written for the task's `links` step, a check (role runner). An llm judge on the trace sees only its first and last 12 messages, so the brief is read from the file instead.
PASS if the brief only runs the check and records the command, its output and the exit code: its Job has no instruction to edit, remove, fix or rewrite anything, the dead links included. Saying the check should exit 0, or that a failure goes back to the step before, is about the run, not a fix, and passes. The Outcome says the check failed and that no project file was edited.
FAIL if the Job or Acceptance asks for the links to be removed or any file to be changed, if the Outcome says a file was edited, or if the file is missing.
