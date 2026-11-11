---
name: plan-step-anchor
tags: [live]
runs: 1
max_turns: 40
allowed_tools: [Bash, Read, Glob, Grep]
---

The task folder `task-export` holds an accepted plan.md with three steps and a verify section. Turn the plan into Jobs: post one Job for each plan step with `grid job post`, using the plan-jobs skill. Each Job must point at its own step in plan.md and at the verify section, and `grid job lint` must pass before you post it.
