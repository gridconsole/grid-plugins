---
name: execute
description: Run the task's current workflow step: pick the agent, have a brief written, start the agent and record the result. Use when the person says "run it", "do the next step", "go ahead with the build", "carry on with T-02", or a plan is accepted and the step is a run, check or make-together step.
---

<!-- Decision source: plugin component table row 5 (execute runs steps through runner and worker subagents); Q13 (steps have kinds, no fixed phases); Q22 wins over the old agent-gate row (workers may start workers, nothing here blocks it); Q27 item 1 (briefs are written by grid-brief-writer); Q7 as read through Q13 (a runner is switched only at a step boundary); step 19g C4 (a workflow item's agent runs the step, before routing.md and the kind table). -->

# Execute

One step at a time, in order. The task's `workflow` in task.md lists the steps with their kinds and `step` names the current one. This skill runs that step, records it, and when the next step needs no one, runs that one in the same turn, and so on.

A task keeps going on its own until it reaches something only a person can give. These are the only places to stop:

- a question or an approval waiting in Needs you (a blocking question, or an `approve` step the person has to answer);
- a `make-together`, `ask` or `schedule` step: the person steers, answers or the time has to come;
- an `ask` step waits only with an open question file in `questions/`: write it first (questions skill) with the choices `Done, carry on / Not yet / Answer in my own words` for a person's own action, and on Done record it and run the next step;
- a blocked brief;
- the last step finished: the task is done and delivery is next (on a task with a ship approve step, the Outcome was already written before that gate);
- a usage or session limit;
- the person pausing or stopping the task.

Everything else is carried on without a message from anyone: a step passed, a check failed and went back to build, a review returned must-fix, a new round. "The next step is ready" is never a reason to stop or to ask; start it.

The task folder is `<projects folder>/<project>/tasks/T-NN-slug/`. The projects folder is `GRID_PROJECTS`, else `$GRID_HOME/projects` (`GRID_HOME` is the Grid data folder `~/.grid/<name>`, never the workspace folder; if both are empty, Grid is not set up for this folder: say so and write nothing). If the person does not say which task, take the only open one, else ask. Never search the disk for it. Read `task.md`, `intent.md` and `plan.md` (if it exists) first.

## Before starting

- Grid opens a session with `/grid:execute T-NN` after a person gives something: it resumes the task's last session, or starts a new one when that cannot be continued. A line under it says what was given, for example `plan accepted by sam; the step is build now. Read task.md and continue.` or `Q-2 answered by sam. Read questions/ and continue.` That line is the go-ahead, not a reason to stop, ask or wait for a second message: read `task.md`, then run the current step. Grid sends it only once the task is `running` or `reworking` with nothing in `needs`, and a session that is still open is sent `Carry on with step <name>.` instead, so the same rules apply.
- If this is the first turn of the session (Grid just started or resumed it), no agent of yours is running. A brief of the current step that still reads `running` was cut off with the old session: set it to `blocked` (Edit that line only), note in task.md's body line that it was cut off, and start the step again with a new brief through `grid-brief-writer`. Never edit its Outcome.
- `status: closed`, `paused` or `pausing`: say so and stop. A closed task is not run again.
- `status: waiting`: say what it waits on (`needs`, `next`) and stop, unless the person has just answered it.
- A step already started in this session: if the current step's latest brief is `running` and you started it, wait for it instead of starting another; if it is `done` and its result is not yet in task.md, go to section 4.
- A blocking question in `needs` that is still open: stop.
- The planner start line: the task sits on its `plan` step and has no `plan.md` yet. Plan it before anything else: unless task.md says `size: small`, run the research skill first (`grid-scout` maps the files when they are not obvious, `grid-researcher` writes `research.md`), then the plan skill, which asks the open decisions in `questions/` and starts `grid-reviewer` on the plan before `status: waiting`.
- Never use AskUserQuestion: use the questions skill and wait for the answer.

## 1. Pick the agent

The step's own `agent` decides, then `routing.md`, then the step's kind.

1. If the step's item in `workflow` has an `agent` (`{step: build, kind: run, agent: grid-worker}`), the person chose it in the workflow, and that agent runs the step. It wins over `routing.md` and the table below:
   - `run`, `check`: that agent runs it. A `check` stays a check: it runs the check and never fixes.
   - `make-together`: that agent builds the draft with the person.
   - `ask`: that agent works out the question; put it with the questions skill and wait for the answer.
   - `approve` and `schedule` items carry no agent.

   If no agent of that name can be started, say so and ask; do not pick another one.
2. If the item has no `agent` (an older task.md, or an approve or schedule step) and `<projects folder>/<project>/routing.md` exists, use the routing skill's lookup for this project and kind. A rule wins over the table. A clash between rules: ask, do not pick.
3. Otherwise:

| Kind | Who |
|---|---|
| `check` | `grid-runner` runs the check; a failed check sends the work back a step, or asks the owner when a rework cannot fix it (section 5) |
| `ask` | no agent: use the questions skill, then wait for the answer |
| `approve` | a person, after `grid-reviewer` reviews the finished work; for UI or website work also `grid-checker` on its own `checker` brief, after the reviewer accepts. On a ship approve step (after the build) the deliver skill's `## Before the gate` writes task.md's `## Outcome` before the gate is raised. Raise the question and wait. Never write `plan accepted`, `preview approved` or `ship approved` |
| `make-together` | `grid-worker` builds the draft with the person; `grid-worker` builds UI when the brief names the design skill; `grid-designer` writes `spec.md` only, first for a large task |
| `run` | `grid-worker`, after `grid-scout` or `grid-researcher` when the files or facts are not known and `research.md` does not already hold them |
| `browse` | `grid-operator`: browser work that changes state, or a check marked `Needs grid-operator` |
| `schedule` | set `status: queued` and `next` to the set time; at that time `grid-runner` runs the one command |

On an `approve` step, first check the finished worker brief: if it has no Verdict yet, start `grid-reviewer` on it before anything else, then read the Verdict line:

- `ACCEPTED`: on a ship approve step, first run the deliver skill's `## Before the gate`, so task.md holds the `## Outcome` the person approves; this holds every time the gate is raised, also after a change asked at the gate led to more work. Then set `status: waiting` and name who in `next`. Then, as on an `ask` step, there is nothing to run: report what is waiting and stop.
- Anything else is `REJECTED`: a Verdict of `REJECTED`, `CHANGES`, or any first line that is not `ACCEPTED`, or one with a must-fix list. Do not wait and do not ask the owner whether to rework. While the round limit (section 5) allows another round, send the work back to the step that built it (section 5, "A review returned must-fix"): a fix brief, the worker, then `grid-reviewer` again on the new brief, all in this turn. Only past the limit raise a blocking question to the owner through the questions skill, naming the must-fix items still open.
- Waiting starts only on `ACCEPTED`. Once the review passes, the person's approve step waits for the person as before.

An end-of-phase review is a `grid-reviewer` brief whose Job says `end review` and names the phase's exit criteria and baseline.

A worker that reaches its turn limit has a step that is too big. Split it into briefs with smaller write sets, and never rerun the same brief.

## 2. Write the brief

Ask `grid-brief-writer`: the task folder path, the step name and kind, the role from step 1 and the job in a sentence. For a `check` step the job is the check to run, never the fix, even when intent.md says what to fix after a failure. For a run step with a plan, the job is the plan items for this step (a "plan item" is a numbered line in plan.md). It writes `subagents/NN-name.md` with status `briefed`. You do not write the brief yourself, and an agent may run in the background: wait for its reply before you read the brief. If the brief is not there yet, wait; do not write it, and do not do an agent's job yourself. Read it once: Intent must be the person's words, Acceptance must be checkable.

## 3. Start the agent

Set the brief's `status` to `running` (Edit that line only), start the agent and tell it the brief's path. Independent briefs go in parallel. Briefs that share a file run one after the other. Workers run in the session cwd, which is the task's worktree on `GRID_TASK_BRANCH` (the host makes one per task). Parallel worker branches, if used, are named `<GRID_TASK_BRANCH>-<slug>` (for example `grid/T-03-header`, never `grid/T-03/header`: git cannot hold both refs) and branch off `GRID_TASK_BRANCH`. Each gets its own worktree beside the task's, `$GRID_WORKTREE-<slug>`, never inside the repo folder (`git -C "$GRID_WORKTREE" worktree add -b <branch> "$GRID_WORKTREE-<slug>"`), and the worker runs there. Before the step is done, each is merged back into `GRID_TASK_BRANCH` in the task worktree, as the deliver skill's `## Merging` says, and its worktree removed (`git -C "$GRID_WORKTREE" worktree remove "$GRID_WORKTREE-<slug>"`), keeping the branch. A worker may start its own helpers; do not forbid it.

## 4. Record the result

Read the brief's Outcome (a reviewer writes Verdict only; a scout's reply is the map and you put it in Outcome for it). Set the brief `done` if the Outcome shows the job and its `verify` ran, else `blocked` and say what is missing. Never edit another agent's Outcome or Verdict.

Read the brief with the Read tool right before you Edit it: a subagent wrote it since you last read it, and Edit refuses a file this session has not read.

Tick plan items through the plan skill, and only with evidence.

Only when the task's work is in a git repo: copy the Outcome's `repo: <name> branch: <branch> base: <base>` lines into task.md's `repos:` in the section 5 edit, for a `blocked` brief too, one `  - {name: <name>, branch: <branch>, base: <base>}` item per repo, without `base` when the line has none. A repo already listed keeps its one item: update its `branch`, and keep its `base` unless that was empty. This is the only place a task's branches are recorded, in task.md in Grid's data folder; never write a branch note, file or setting into the repo.

## 5. Task files, at the boundary only

Always: in task.md set `updated` (the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess) and `next`, and add a body line `- <D Mon> · <step> · <what happened>`.

Move `step` only when the step is finished:
- `run`, `make-together`, `check` passed, brief `done` and its Acceptance met: set `step` to the next one in `workflow`. "Check passed" means the check brief's Outcome says passed and its Acceptance lines are met; a brief `done` only means the command ran. A check brief whose Outcome says it edited project files does not count as passed: treat it as a failed check (the bullets below). If the next step is `approve`, start `grid-reviewer` on the worker's brief first; once it is `ACCEPTED` and the step is a ship approve, run the deliver skill's `## Before the gate`; then set `status: waiting` and name who in `next`. If it is `ask`, set `status: waiting` and name who in `next`.
- An `approve` step: you never move `step` forward off it, except back to the build step when a review returned must-fix (below). Grid moves `step` when the person approves; no skill does.
- The round limit is the `rounds` of the workflow item for the step the task is on; if that item has none, the `review rounds` line of the workflow template in `<projects folder>/workflows.md` or `<projects folder>/<project>/workflows.md` whose step names and kinds, in order, equal the task's (the later one wins); else 3. Round 1 is the first pass. Work goes back only while the new `round` is no greater than the limit; past it, raise a blocking question to the owner (the bullet after the next) instead of a new round.
- A review returned must-fix (an `approve` step's Verdict that is not `ACCEPTED`, section 1), and the new `round` is within the limit: send it back without a question. Set `step` to the step that built the reviewed brief (the template's "on reject, back to" step when it names one), `round` one higher, `status: reworking`. `next` says in the present tense what the rework changes ("Claude fixes the three must-fix items the review found, round 2"), never a question. Ask `grid-brief-writer` for a fix brief whose job is the Verdict's must-fix items. Add a body line to task.md and the history line `- <time> · sent back to <step> · round <n> of <limit> · review must-fix · by grid-planner` (`<time>` as below). Then carry on (section 6): the worker, then the reviewer again. The `approve` step is reached again only after the build and the checks pass.
- A review returned must-fix and the new `round` would be past the limit: leave `step` and `round`, and raise a blocking question to the task's `owner` through the questions skill. Name the must-fix items still open and a recommendation; options such as "One more round / Approve as it is / Stop the task". No amount of money in it.
- `check` failed, and redoing the step before it can fix the failures (within the round limit, otherwise as the next bullet): set `step` back to that step and `round` one higher (`2` the first time), `status: reworking`. `next` says in the present tense what the rework changes ("Claude fixes the date format the check flagged, round 2"), never a question. After a `make-together` step the rework is done with the person too, and `next` names them ("Claude and sam rework the team line the check flagged, round 2"). When the next agent for that step starts, set `status: running` again.
- `check` failed, and a rework cannot fix it: the failures sit in files the task must leave alone (intent.md says so, or another task owns them), or they need a choice only a person can make. Sending the step back would only loop. Leave `step` and `round` as they are and raise a blocking question to the task's `owner` through the questions skill, naming the failing items and your recommendation. That skill leaves the task `waiting` with the question in `needs` and `next` naming who decides.
- Brief `blocked`, or the step is not finished: leave `step`, and set `status: stuck` only if no agent and no Grid action can go on and no person decision is missing (a decision is a question, status `waiting`); `next` never tells a person to start, resume or restart a session or rerun a brief. If a fresh session or a rerun is wanted, set `status: running` and let `next` name the step.
- The last step finished: do not close the task. The deliver skill writes the outcome; on a task with a ship approve step it wrote it before the gate, and after `ship approved` it refreshes it only if something changed.

Item records. When the brief has an `item:` line, record where its plan items stand with `node "$CLAUDE_PLUGIN_ROOT/skills/execute/scripts/item-record.mjs" <task folder> <state> <N[,N]> --by <role> [--env <id>]`, which appends the `- <time> · item N <state> ...` history line itself (never write these lines by hand):
- a worker or operator brief `done` whose items are built and now run on the local environment: `testing` (always `local`);
- a `check` brief (grid-checker or operator) whose Verdict or Outcome passed for those items: `testing` if nothing was recorded yet, and `deployed --env <id>` when the check ran on that environment of the project's `environments.json`, with `--by` the checker's role. The checker is read-only, so you run the script for it after reading its ACCEPTED Verdict;
- a check that failed on local: `failed`.
Skip it when the brief has no `item:`. Never invent an environment id: it must be one in `environments.json`, else the script refuses and you leave the line out.

Never write `runner`, `gate_raised`, `paused`, `lane`, `lane_step` or `lane_round`, never write an approval line, and never edit `intent.md`. history.md is appended by a hook as files change; if you add a line, its `<time>` is the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess, and you re-read the file after a "modified since read" error and try again.

## 6. Carry on

Once the task files are written, read `step`, `status` and `needs` again. If `status` is `running` or `reworking`, `needs` is empty, no brief is blocked and the new current step is a `run` or `check` step, go back to "Before starting" and from there to section 1 with that step now, in this turn. Do not write a message and wait for the person to say "carry on".

A review sent back to the build step (section 5) is carried on the same way: the current step is then a `run` step with `status: reworking`, so section 1 starts the worker on the fix brief now. Never write a message that asks the person whether to go back to build.

Do not go back when the current step is an `approve`, `ask`, `make-together` or `schedule` step, or when the last step's brief is done and its result is recorded in task.md: the task is done and delivery is next.

Stop at one of the places in the opening list, then tell the person which agents ran, the brief paths, the result of each step in a line, and what happens next or what waits on whom.
