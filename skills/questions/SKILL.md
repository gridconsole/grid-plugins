---
name: questions
description: Ask a person a question on a Grid task, record their answer, or withdraw it. Use when work needs a decision only a person can make, or the person says "ask Sam which ...", gives an answer, or a question is no longer needed.
---

A question is for a person point only. "Tasks carry on by themselves until a person point": never ask whether to rework, never ask about a refused command, never ask about the brain, and never ask what a default or the brain already answers (`grid brain search <words>`).

Rules that apply here:
- "Everything is a Job, the Job is the brief": a person's decision is a Job with role Person (see plan-jobs); the question is that Job's text.
- "Approvals and gates come from the workflow in the brain": do not make up a gate; `grid question add --blocking` is the only way work waits.
- "No Grid-made guards, caps or limits."
- "Every link carries a why."

Steps (all use `--task <task>` or the current task; `question` and `person` take no `--project`, so when a bare id answers "needs --project", pass the task folder path `<grid home>/projects/<project>/tasks/<folder>` as `<task>`):

1. Pick the person. `grid person approvers --area <area> --task <task>` names who answers for the area; no match means the task owner (`grid person show <handle>`).
2. Ask: `grid question add --task <task> --who <handle> --question '<one question in plain words>' --option '<answer A>' --option '<answer B>' --recommended '<option label>' --applies <RFC3339 time with timezone> --default '<option label applied at that time>' --why '<why the answer matters>' [--blocking] [--item <n>] [--job <id>]`. Options are concrete answers only. Always give a recommendation, a why, a deadline (`--applies`) and the default (`--default`) that applies at the deadline; the CLI refuses `--applies` that is not an RFC3339 time with a timezone, and a gate comes with a default. Use `--blocking` only when work cannot go on without the answer. No money in a question: say tokens, minutes or a share of the usage limit.
3. A question that must reach people outside the task goes on the Feed as a `question` post (`grid feed types` lists it): `grid feed post --task <task> --type question --body <json>`.
4. Read open questions: `grid question list --task <task>`.
5. Record an answer only when the person gave it in their words: `grid answer <task> Q-<n> "<their words>"`. Never invent one. An answer is final; a new answer is a new question.
6. Withdraw a question that is not needed: `grid question withdraw --task <task> Q-<n>`.

Not built: the CLI lists an `ask` verb that answers "not built yet" (exit 2). Use `grid question add`, which does the same work, and name the gap in your report.

Done looks like: `grid question list` shows the question open with its options, recommendation and why, or the answer recorded, and the task carries on without waiting for anything else.
