---
name: deliver
description: Finish a task with proof and an Outcome so the person can review it. Use when the work of a Grid task is done, the person says "deliver it", "wrap it up" or "what did we get", or the last workflow step is complete.
---

Delivery writes down what exists and what proves it. It never approves, never closes the task and never asks the person about the brain.

Rules that apply here:
- "No Grid-made guards, caps or limits": the only refusals are the ones `grid deliver` itself returns. Do not add your own.
- "A person step is a Job for a person": `grid deliver` writes the Outcome and sets the task to wait for the person's review (`step: ship`); the person's answer is a Job for a person. Never write an approval.
- "The brain needs no attention from the person": a subagent curates proposals; you never ask the person.
- "Every link carries a why."
- "A row you could not prove is NOT DONE": never write an Outcome that claims it.

Steps (all use `--task <task>`). Pass the task folder path `$GRID_HOME/projects/<project>/tasks/<folder>` as `<task>`; `grid where` prints the project and the folder. The folder path works for every verb below. A bare id such as `T-01` works only for `proof`, `brain`, `task history` and `deliver`; `plan items` and `question list` answer "needs --project" to a bare id:

1. Read the plan: `grid plan items --task <task> --json`. Each item names the evidence kind it needs (`proof: check|log|screenshot|video`; none means check).
2. For each item, run the check yourself and record it: `grid proof add <n> --text '<command and exit code>' --kind check --by <you> --task <task>`, or `grid proof add <n> <file> --kind screenshot --note '<size or page>' --task <task>`. Code facts carry a `path:line` read in this session; UI evidence names its data source and the widths 390, 1440 and 3840.
3. An item you could not prove stays unproven. Say so in the Outcome as NOT DONE with the reason; do not record text evidence that is not true.
4. Rebuild and read the index: `grid proof index --task <task>`.
5. Brain proposal step. Read what the task taught: answered questions (`grid question list --task <task>`), job results, `grid task history <task>`, surprises and failed checks. For each fact that would help another task, one proposal: `grid brain propose --name <id> --topic <topic> --description '<when it applies>' --summary '<what>' --source <T-NN/J-NN or file:line> --body '<text>' --link '<json with to, rel, summary and why>' --task <task>` (`--link` is optional; `to` is `brain:<level>/<id>`, `job:<task>/<job>` or `person:<id>`, and `rel` is one of the relations `grid brain propose` lists, for example `related`). The proposal is written to `<task folder>/brain-proposals/`, where `grid brain decide <task> <proposal-id>` reads it. Pick the topic from `grid brain topics`; a topic with no definition is refused. Use `--level home` for a personal fact. Skip a fact the brain already holds (`grid brain search <words> --task <task>`). `grid deliver` posts the curator Job for any proposals that wait; do not start a curator or run `grid brain decide` yourself, and do not ask the person.
6. Dry run: `grid deliver --outcome '<text>' <files> --dry-run --task <task>`. Read the refusal, if any, and fix its cause: a plan item that lacks proof (run `grid proof add`), a plan with no items or no plan.md, a deliverable file that cannot be imported, a task that is closed, stopped, paused or already waiting. Waiting proposals and plan approval are not refusals.
7. `grid deliver --outcome '<text>' --file <deliverable> ... --task <task>`. The Outcome's first line is the summary: what was delivered, in the person's terms, with the main path. Name every plan item and its proof, and every NOT DONE row.

Done looks like: `grid deliver` exits 0, `state: waiting` and `step: ship` in task.md, one proof record per plan item, and the curator Job posted when proposals wait. The person approves or sends it back; tasks carry on by themselves until that point.
