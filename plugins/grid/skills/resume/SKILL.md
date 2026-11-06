---
name: resume
description: Pick a task up again after a pause or a stop. Use when the first turn is "/grid:resume T-NN" (Grid sends it when a paused or stuck task is resumed), when Grid starts a turn after the person approved or sent back a plan, or the person says "resume T-02", "carry on after the pause". At the plan's approve step, history.md decides: a plan still waiting ends the turn, an accepted plan carries on with the execute skill, a plan sent back is revised and the turn ends again; otherwise carry on with the step it was on.
---

# Resume

Grid starts a resumed session with `/grid:resume T-NN`, or `/grid:resume T-NN from <n>` when the person chose a plan item to start from. This skill finds where the task stood and goes on from there. It is also the turn Grid starts on the planner's session after the person approves the plan or sends it back.

The task folder is `<projects folder>/<project>/tasks/T-NN-slug/`. The projects folder is `GRID_PROJECTS`, else `$GRID_HOME/projects` (`GRID_HOME` is the Grid data folder `~/.grid/<name>`, never the workspace folder; if both are empty, Grid is not set up for this folder: say so and write nothing). If the person does not say which task, take the only open one, else ask. Never search the disk for it. Read `task.md`, `intent.md`, `plan.md` (if it exists) and the end of `history.md` first.

## Before starting

- `status: closed`: say so and stop. A closed task is not run again.
- `status: pausing`: the pause has not finished. Say so and stop.
- `status: waiting` on a question in `needs` that is still open: say what it waits on and stop, unless the person has just answered it.
- `status: paused`, `stuck`, `queued`, `running` or `reworking` (a plan sent back), or `waiting` with nothing open in `needs`, is what a resume finds. Resume is how a stuck task goes on, so a person is never asked to restart a session; set `status: running` with `next` naming the step. Set `status`, `updated` (the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess) and `next`, and add a body line `- <D Mon> · <step> · resumed`. At the plan's approve step (below), a plan still waiting has `status: waiting` and `next` names who accepts it ("Sam reads plan rev 3 and accepts it in Grid"): the plan waits on that person, and Grid offers the approve only on a waiting task. A plan sent back stays `reworking` until its revision sets `status: waiting` (Sent back, below). At any other step, and once the plan is accepted, `status` is `running`. Leave `paused`, `runner`, `gate_raised`, `lane`, `lane_step`, `lane_round` and `step` as they are.
- Never use AskUserQuestion: use the questions skill and wait for the answer.

## Where it stood

Take `step` from task.md and its kind from `workflow`.

### The plan's approve step

This is the case when `step` is an `approve` step that comes before every `run` and `make-together` step in `workflow`, and `plan.md` exists. The planner plans in normal mode: the plan is `plan.md` in the task folder and the gate is history.md, never a tool call. Find the last `plan ready · rev <n> · <hash>` line in history.md and the gate lines after it. history.md is only appended to, so go by line order, not by the times (two lines can share a minute):

- **Sent back**: a `plan rejected · rev <n> · <hash>` or `plan sent back · round <n> · <who>: "<words>"` line after the last `plan ready`, or task.md says `status: reworking` with no such line yet (Grid sets the status before it writes the line, and the change is in the turn it sent). Check this case first. The person's words in that line, and in the turn Grid sent, are the changes asked.
  1. Revise `plan.md` through the plan skill from those changes, with `--bump` and its `plan ready` line. Change only what they asked.
  2. Rewrite `views/plan.html` through the plan-page skill, have `grid-reviewer` read the new rev, and set `status: waiting`, all as the plan skill's "After writing" says.
  3. End the turn. Grid holds the new rev at the gate.
- **Waiting**: no `plan accepted`, `plan rejected` or `plan sent back` line after the last `plan ready`. The plan waits on the person.
  1. Check that task.md says `status: waiting` (Before starting).
  2. Read `plan.md` as it is. Do not reword, tick or bump it: the person approves the rev and hash they saw, and a bump voids that. Only if the person asked in this turn for a change do you revise it through the plan skill, with `--bump` and its `plan ready` line, and say that the earlier approval no longer counts.
  3. If `views/plan.html` is missing, write it through the plan-page skill.
  4. End the turn. Never write `plan accepted`: the person accepts the plan in Grid, and Grid starts a new turn on this session with the result.
- **Accepted**: a `plan accepted` line after the last `plan ready`. Carry on in this same turn: Grid moves `step` off the approve step when the person approves, so read task.md again and run the new current step with the execute skill, from the plan item the person named (`from <n>`), else from the first unticked one, and stop only at a place its opening list names (stopping here once cost T-04 97 minutes, brief 103 run 5).

If `plan.md` does not exist yet, the plan was never written: use the plan skill to write it, then end the turn as in Waiting.

### Any other step

1. Read the step's briefs in `subagents/`. A brief with status `running` has no agent any more: the session that started it ended. Set it to `blocked` (Edit that line only) and note in task.md's body line that it was cut off; a new brief goes through `grid-brief-writer`. Never edit its Outcome.
2. If the step is the last step in `workflow`, `status` is `running`, and history.md's latest gate line is `ship approved` or `preview approved` with no `delivered` line after it, the person approved the result: run the deliver skill. It keeps the Outcome written before the gate unless something changed, writes the `delivered` line and lands the work. Do not set `status: waiting`.
3. If the step is an `approve` step after the plan (a review, a preview, a sign-off), there is nothing to run: on a ship approve step whose task.md has no `## Outcome`, or one older than a brief's Outcome, first run the deliver skill's `## Before the gate`. Compare by history.md order: the Outcome is older when a `subagent finished · brief <NN-name>` line (or a `file edited · subagents/<NN-name>.md` line) comes after the last `outcome written` line, which is where a Before-the-gate run ends, or when there is no `outcome written` line; the run then rewrites the Outcome whole. Then set `status: waiting`, name the person in `next`, report what waits and stop.
4. Otherwise run the step through the execute skill, from the plan item the person named (`from <n>`), else from the first unticked plan item. Tick nothing without evidence.

Tell the person what the task was doing, which case applied, and what happens next.
