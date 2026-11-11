---
name: replan-dead-job
tags: [live]
runs: 1
max_turns: 40
allowed_tools: [Bash, Read, Glob, Grep]
---

The task folder `task-replan` holds a task whose Job J-2 was cancelled. Job J-5 in its Feed is the Re-plan Job. Run J-5 the way the plan-jobs skill says for a Re-plan Job: read the cancelled Job and why it stopped, decide for each Job that waits on it by the plan, and act on it.
