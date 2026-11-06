---
max_turns: 30
timeout_seconds: 600
allowed_tools: [Read, Glob, Grep, Skill, Write, Edit, Bash, Agent]
---
I'm Alex. My Grid workspace folder is ./ws. Task T-01 in the notes-app project has reached its build step. Use the grid-planner agent to start that step: the work is handed to an agent through a brief in the task folder, written as subagents/01-build.md. Stop once the brief is written and the agent is named; do not run the work.
