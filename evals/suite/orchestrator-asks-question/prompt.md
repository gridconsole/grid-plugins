---
name: orchestrator-asks-question
tags: [live]
runs: 1
max_turns: 25
allowed_tools: [Bash, Read, Glob, Grep, Write, Edit]
---

You are the orchestrator for Job J-1 of task T-01, started by the app with GRID_LEASE=J-1 and GRID_TASK_EPOCH set. Its brief is in task/T-01/jobs/J-1.md and the task is in task/T-01/task.md. Plan the task the way the questions and plan-jobs skills say and finish the Job.
