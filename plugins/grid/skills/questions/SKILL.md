---
name: questions
description: Ask a person a question on a Grid task, record their answer, or withdraw it. Use when work needs a decision only a person can make, or the person says "ask Sam which ...", gives an answer, or a question is no longer needed.
---

A question is for a person point only. "Tasks carry on by themselves until a person point": never ask whether to rework, never ask about a refused command, never ask about the brain, and never ask what a default or the brain already answers (`grid brain search <words> --task <task>`).

Rules that apply here:
- "Everything is a Job, the Job is the brief": a person's decision is a Job with role Person (see plan-jobs); the question is that Job's text.
- "Approvals and gates come from the workflow in the brain": do not make up a gate; a Job waits only by naming the person Job (or `gate:Q-<n>`) in its own `after`. `--blocking` is a label that raises the question's priority; it holds no Job.
- "No Grid-made guards, caps or limits."
- "Every link carries a why."

Steps (all use `--task <task>`). Pass the task folder path `$GRID_HOME/projects/<project>/tasks/<folder>` as `<task>`; `grid where` prints the project and the folder. The folder path works for every verb below. A bare id such as `T-01` works for every verb below when you run it inside the task's project folder; from a folder in no project, `proof`, `brain`, `task history` and `deliver` still find it, and `question`, `answer`, `person` and `plan` answer "needs --project" (a role with no Personal scope gets "task not found"); the folder path works from anywhere (`person` also takes `--project <project>`):

1. Pick the person. `grid person approvers --area <area> --task <task>` names who answers for the area; no match means the task owner (`grid person show <handle>`).
2. Ask: `grid question ask --task <task> --who <handle> --question '<one question in plain words>' --option '<answer A>' --option '<answer B>' --recommended '<option label>' --applies <RFC3339 time with timezone> --default '<option label applied at that time>' --why '<why the answer matters>' [--waits-on '<option label>'] [--item <n>] [--job <id>]`. Options are concrete answers only. `--waits-on` (repeatable) names an option that keeps waiting, such as `Not yet` for a person's own action: that answer is recorded, and the question and its Job stay open for the next answer. Always give a recommendation, a why, a deadline (`--applies`) and the default (`--default`) that applies at the deadline; the CLI refuses `--applies` that is not an RFC3339 time with a timezone, and a gate comes with a default. No money in a question: say tokens, minutes or a share of the usage limit.
   Run by a person of the team or by the holder of the task lease (the orchestrator), `ask` also posts a person Job and the question names it; run by any other writer it posts no Job. The question is itself a person gate, and the Feed takes a person gate only from the roles the workflow's `gate_post_roles` lists for `person` (by default cli, owner, member, lead). A Job agent with GRID_TASK_EPOCH set (every Job agent the app starts, whatever its role) asks as the task holder, so `ask` posts its person Job: do not post a second one. An agent without GRID_TASK_EPOCH (GRID_ROLE=worker, its own Job in GRID_LEASE) gets `role worker may not post person gates` from `grid question ask` under the default workflow and may not post a person Job either: it names the decision, with the options and its recommendation, in its result's `decisions_needed` (`grid job result --task <task> --job <job> --body <json>`) and stops; the orchestrator asks the person. A Job that needs the answer lists `gate:Q-<n>` (or the person Job id, when `ask` printed one) in its own `after` (see plan-jobs); every other Job carries on. `grid question add` takes the same flags and posts no Job: use it only for a note to the person that holds nothing.
3. A question that must reach people outside the task goes on the Feed as a `question` post (`grid feed types` lists it): `grid feed post --task <task> --type question --body <json>`.
4. Read open questions: `grid question list --task <task>`.
5. Record an answer only when the person gave it in their words: `grid answer <task> Q-<n> "<their words>"` (here `<task>` is the first operand, not a flag). Never invent one. An answer is final; a new answer is a new question.
6. Withdraw a question that is not needed: `grid question withdraw --task <task> Q-<n>`.

Done looks like: `grid question list` shows the question open with its options, recommendation and why, or the answer recorded, and the task carries on without waiting for anything else.
