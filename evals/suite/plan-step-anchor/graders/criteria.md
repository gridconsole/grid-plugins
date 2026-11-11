---
name: criteria
type: llm
weight: 1
---

The agent posted one Job per plan step, three in all. Each Job points at its own step with its own anchor (`plan.md#step-1`, `plan.md#step-2`, `plan.md#step-3`) and at `plan.md#verify`. No Job points at the whole `## Steps` section or at an anchor plan.md does not have, and no step is left without a Job.
