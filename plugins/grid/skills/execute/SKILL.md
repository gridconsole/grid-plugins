---
name: execute
description: Run the task's current workflow step: pick the agent, have a brief written, start the agent and record the result. Use when the person says "run it", "do the next step", "go ahead with the build", "carry on with T-02", or a plan is accepted and the step is a run, check or make-together step.
---

<!-- Decision source: plugin component table row 5 (execute runs steps through runner and worker subagents); Q13 (steps have kinds, no fixed phases); Q22 wins over the old agent-gate row (workers may start workers, nothing here blocks it); Q27 item 1 (briefs are written by grid-brief-writer); Q7 as read through Q13 (a runner is switched only at a step boundary); step 19g C4 (a workflow item's agent runs the step, before routing.md and the kind table). -->

# Execute

The task's `workflow` in task.md lists the steps with their kinds and `step` names the current one. Workflow steps go in order; this skill runs the current step, records it, and when the next step needs no one, runs that one in the same turn, and so on. Inside a `run` step with a plan, the plan's numbered steps are not a queue: the Feed loop runs them as Jobs (the section "Run a `run` step through the Feed loop"); on the old path, every one whose `needs:` are ticked runs at the same time, up to the project's limit (the section "Run every ready step together").

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
- If this is the first turn of the session (Grid just started or resumed it), no agent of yours is running. Every brief of the current step that still reads `running` was cut off with the old session: set each to `blocked` (Edit that line only), note in task.md's body line that they were cut off, and start each of those steps again with a new brief through `grid-brief-writer`, all together as in "Run every ready step together". Never edit an Outcome.
- `status: closed`, `paused` or `pausing`: say so and stop. A closed task is not run again.
- `status: waiting`: say what it waits on (`needs`, `next`) and stop, unless the person has just answered it.
- A step already started in this session: a brief that is `running` and that you started is left to finish, and its step never gets a second brief; the other ready steps still start. If a brief is `done` and its result is not yet in task.md, go to section 4 for it.
- A blocking question in `needs` that is still open: stop.
- The planner start line: the task sits on its `plan` step and has no `plan.md` yet. Plan it before anything else: unless task.md says `size: small`, run the research skill first (`grid-scout` maps the files when they are not obvious, `grid-researcher` writes `research.md`), then the plan skill, which asks the open decisions in `questions/` and starts `grid-reviewer` on the plan before `status: waiting`.
- Never use AskUserQuestion: use the questions skill, then end the turn; the answer resumes the task.

## Run a `run` step through the Feed loop

On a `run` step whose task has a `plan.md`, the Feed loop does the work (the feed skill describes the Feed): the plan becomes Jobs, Workers on Hosts run them, their results come back on the Feed, and a review Job judges the whole. It replaces starting in-session subagents by hand. The logic is plain code in the plugin, so a plain terminal session runs it exactly as Grid does.

**Old path.** A task started before the Feed loop existed keeps the section "Run every ready step together" (briefs and in-session agents): that is a task whose folder has a `subagents/` folder with briefs and no `feed/`. Every other task, including every new one, uses the Feed loop. A task with a `feed/` always continues on it.

1. **Start it.** Run, on its own, with the project's repo clone from `project.md` after `--repo` when the plan changes code (leave `--repo` off for research and documents):

   ```
   node "${CLAUDE_PLUGIN_ROOT}/feed/start.mjs" T-NN --repo "<clone>"
   ```

   The first start saves its flags (`--repo`, `--base`, `--agent`, `--slots`, `--model`, `--interval`) in the task folder, `feed/.orchestrator.json`, never in the repo. A later start with no flags, such as a resume, runs the loop the same way; give a flag again only to change it, and the change is saved.

   It prints one JSON line. `started`: a loop now runs detached (its output in `feed/.orchestrator.log`) and outlives this session; it plans the Jobs from `plan.md` with one model call, then starts Workers as Jobs become ready, within the Hosts' free slots. A first plan that fails is asked again by the loop, at most three times, spaced one interval apart; after that the loop asks the Owner "Planning failed: <reason>. Retry it / Drop it" and stops, and a Retry answer plans it again. `running`: a loop already holds the task, change nothing. A lock left by a loop that no longer runs (its process gone, or no beat for 2 minutes) is taken over, and the line then says `took_over: true`. `done`: every Job is finished, go to step 4. `no-plan`: plan the task first. `error`: say the reason from `errors` and stop; with `retrying: true` a loop was started and keeps trying, without it no loop runs until the Owner answers.
2. **The review.** `start.mjs` plans the Jobs once, and when none of them is a `reviewer` it posts a review Job as the Orchestrator after every build Job, so the work always ends on a review. Write the review as the plan's last step anyway when the plan has specific things to check.
3. **Watch, do not hand-run.** Read `grid-feed state --task T-NN` (or `feed_state`): each Job with its status, the open questions, the Hosts. Never start a subagent for a Job the loop owns. A `question` on the Feed is a question for Needs you: the loop files it itself in `questions/<owner>.md`, with a `feed: <post id>` line under the heading, and sets the task waiting when it blocks; do not file it again. End the turn. Once the person answers in the app or in a terminal (the questions skill's Record an answer), the loop posts the answer on the Feed as an `answer` from the Owner on its next tick, and carries on. A parked or rejected Job the loop cannot move is a blocker: say which and why. A loop that died resumes by running step 1 again (the Feed is the state, the saved settings are reused). Between checks end the turn; Grid resumes the task when a Job posts.
4. **Record.** When every Job is `done`, read each Job's `result` and the review Job's result, tick the plan steps they cover with evidence from those results (plan skill), copy `repo:` lines from the `submit` posts' branches into `repos:` as in section 4, then go to section 5 and 6. A review that disagrees puts the work back: post a new Job for the fix and its re-review, and run step 1 again.

## Run every ready step together

On a `run` step whose task has a `plan.md`, the plan's steps are the work. A step is ready when it is unticked, every step in its `needs:` line is ticked, and no brief of it is running. Get the set with one command, on its own (no `cd`, `&&` or anything chained):

```
node "${CLAUDE_SKILL_DIR}/scripts/ready-steps.mjs" "<absolute task folder>"
```

It prints the limit, what runs, `start now:` (the ready steps that fit under the limit, lowest number first), and every other step's state: ready and waiting for a free slot, `problem:` lines (for example a `Needs:` or `NEEDS:` line that must be written `needs:` in lowercase), held (its latest brief is blocked: a brief cut off by an ended session is restarted with a new brief, any other waits for its question), in hand (a brief finished but the step is not ticked: a review, a fix or the next brief is yours to do), waiting (and on which steps) and done, and a `problem:` line for a `needs:` that names a missing step, itself or a loop (fix those in the plan before relying on the set; needs lines are outside the plan hash, so the fix needs no bump). A plan without `needs:` lines is read as each step needing the one before it, so it goes one step at a time until the planner adds the lines (plan skill, Revising).

- **Start them all in the same turn.** Every step you start from `start now:` gets its own brief (section 2) and its own agent (section 1, 3), and the agents are started together in the background, one message with several Agent calls. Never start one, wait, then start the next, and never wait for a step that does not gate the others.
- **The limit.** `parallel_steps` in the frontmatter of `<projects folder>/<project>/project.md` is how many steps run at once; the person sets it, and 6 is used when it is missing. The script applies it: steps ready beyond it wait and start as slots free. Do not edit project.md.
- **Same files, one after the other.** Before you start the set, read each step's plan text and the Job of any brief it already has for the files it writes. Steps that write the same file are not started together: start the lowest-numbered and let the others wait, still ready, until its branch is merged. Write no brief for a step that waits; its brief is written when it starts, so `ready-steps.mjs` keeps offering it.
- **Code steps each in their own worktree** (section 3), so no two agents share a checkout. A step that writes no code (a scout, a researcher, a check, a document in the task folder) needs no worktree and starts beside the rest.
- **Each step ends on its own.** When a brief finishes, record that step now (section 4): review it where it is reviewed, merge its branch, tick it. Then run the script again and start whatever became ready, before the other agents are done. Do not hold a finished step until the slowest of the set ends.
- **Heavy runs go through the slot lock.** Ready steps start together, but a full build, the whole test suite or any other heavy compile goes through the project's slot lock (a Makefile target that lets a few run at once, such as up to 3 Rust compiles, and queues the rest); the agents wait on that lock, not on each other. Never start a heavy run outside the lock, and never run a suite once per step: hand it to one `grid-runner` per machine. A project with no slot lock runs its heavy runs one at a time per machine.
- **When this machine is the bottleneck** (heavy runs queue several deep, or the machine has no room while ready steps wait), place steps on another Host the project has, an SSH machine added in Grid, rather than raising `parallel_steps`: say in `next` which steps go there and move them the way the person would. This skill adds no remote runtime. With no other Host, keep to the limit and say in `next` that a second machine would run more at once.
- **Keep the pages true.** When a set starts and when each step ends, update the steps' states on `views/plan.html` and `views/progress.html` (the plan-page and views skills): every running step shows `● running`.

## 1. Pick the agent

The step's own `agent` decides, then `routing.md`, then the step's kind.

`grid-worker` is the old name of `grid-builder`: an `agent: grid-worker` in a workflow or a `grid-worker` row in `routing.md` means `grid-builder`.

1. If the step's item in `workflow` has an `agent` (`{step: build, kind: run, agent: grid-builder}`), the person chose it in the workflow, and that agent runs the step. It wins over `routing.md` and the table below:
   - `run`, `check`: that agent runs it. A `check` stays a check: it runs the check and never fixes.
   - `make-together`: that agent builds the draft with the person.
   - `ask`: that agent works out the question; put it with the questions skill, then end the turn; the answer resumes the task.
   - `approve` and `schedule` items carry no agent.

   If no agent of that name can be started, say so and ask; do not pick another one.
2. If the item has no `agent` (an older task.md, or an approve or schedule step) and `<projects folder>/<project>/routing.md` exists, use the routing skill's lookup for this project and kind. A rule wins over the table. A clash between rules: ask, do not pick.
3. Otherwise:

| Kind | Who |
|---|---|
| `check` | `grid-runner` runs the check; a failed check sends the work back a step, or asks the owner when a rework cannot fix it (section 5) |
| `ask` | no agent: use the questions skill, then end the turn; the answer resumes the task |
| `approve` | a person, after `grid-reviewer` reviews the finished work; for UI or website work also `grid-checker` on its own `checker` brief, after the reviewer accepts. On a ship approve step (after the build) the deliver skill's `## Before the gate` writes task.md's `## Outcome` before the gate is raised. Raise the question and wait. Never write `plan accepted`, `preview approved` or `ship approved` |
| `make-together` | `grid-builder` builds the draft with the person; `grid-builder` builds UI when the brief names the design skill; `grid-designer` writes `spec.md` only, first for a large task |
| `run` | `grid-builder`, after `grid-scout` or `grid-researcher` when the files or facts are not known and `research.md` does not already hold them |
| `browse` | `grid-operator`: browser work that changes state, or a check marked `Needs grid-operator` |
| `schedule` | set `status: queued` and `next` to the set time; at that time `grid-runner` runs the one command |

On an `approve` step, first check the finished worker brief: if it has no Verdict yet, start `grid-reviewer` on it before anything else, then read the Verdict line:

- `ACCEPTED`: on a ship approve step, first run the deliver skill's `## Before the gate`, so task.md holds the `## Outcome` the person approves; this holds every time the gate is raised, also after a change asked at the gate led to more work. Then set `status: waiting` and name who in `next`. Then, as on an `ask` step, there is nothing to run: report what is waiting and stop.
- Anything else is `REJECTED`: a Verdict of `REJECTED`, `CHANGES`, or any first line that is not `ACCEPTED`, or one with a must-fix list. Do not wait and do not ask the owner whether to rework. While the round limit (section 5) allows another round, send the work back to the step that built it (section 5, "A review returned must-fix"): a fix brief, the worker, then `grid-reviewer` again on the new brief, all in this turn. Only past the limit raise a blocking question to the owner through the questions skill, naming the must-fix items still open.
- Waiting starts only on `ACCEPTED`. Once the review passes, the person's approve step waits for the person as before.

An end-of-phase review is a `grid-reviewer` brief whose Job says `end review` and names the phase's exit criteria and baseline.

A worker that reaches its turn limit has a step that is too big. Split it into briefs with smaller write sets, and never rerun the same brief.

## 2. Write the brief

Ask `grid-brief-writer`: the task folder path, the step name and kind, the role from step 1 and the job in a sentence. For a `check` step the job is the check to run, never the fix, even when intent.md says what to fix after a failure. For a run step with a plan, the job is one ready plan step (a "plan item" is a numbered line in plan.md), and its brief carries `item: N`, since the task page and the ready-set script both read it to know what runs. When you ask for the briefs of several steps at once, give each writer its own number (`NN-name`, consecutive from the next free one), so two writers never pick the same file name. It writes `subagents/NN-name.md` with status `briefed`. You do not write the brief yourself, and an agent may run in the background: wait for its reply before you read the brief. If the brief is not there yet, wait; do not write it, and do not do an agent's job yourself. Read it once: Intent must be the person's words, Acceptance must be checkable.

## 3. Start the agent

Set the brief's `status` to `running` (Edit that line only), start the agent and tell it the brief's path. Independent briefs go in parallel, all in the same turn. Briefs that share a file run one after the other. Workers run in the session cwd, which is the task's worktree on `GRID_TASK_BRANCH` (the host makes one per task); when several code steps run together, each of them instead gets its own branch and worktree. The branches are named `<GRID_TASK_BRANCH>-<slug>` (for example `grid/T-03-header`, never `grid/T-03/header`: git cannot hold both refs) and branch off `GRID_TASK_BRANCH`. Each gets its own worktree beside the task's, `$GRID_WORKTREE-<slug>`, never inside the repo folder (`git -C "$GRID_WORKTREE" worktree add -b <branch> "$GRID_WORKTREE-<slug>"`), and the worker runs there. As soon as its step is finished (its brief `done` and, where it is reviewed, `ACCEPTED`), and not when the whole set ends, each is merged back into `GRID_TASK_BRANCH` in the task worktree, one merge at a time, as the deliver skill's `## Merging` says, and its worktree removed (`git -C "$GRID_WORKTREE" worktree remove "$GRID_WORKTREE-<slug>"`), keeping the branch. A merge that conflicts goes back to that step's worker as a fix brief; the steps that wait on it stay waiting. A worker may start its own helpers; do not forbid it.

## 4. Record the result

Read the brief's Outcome (a reviewer writes Verdict only; a scout's reply is the map and you put it in Outcome for it). Set the brief `done` if the Outcome shows the job and its `verify` ran, else `blocked` and say what is missing. Never edit another agent's Outcome or Verdict.

Read the brief with the Read tool right before you Edit it: a subagent wrote it since you last read it, and Edit refuses a file this session has not read.

A `grid-checker` has no Bash and cannot file its own screenshots: whoever ran it (you, or the operator that started it) saves the screenshots its Verdict names through `skills/deliver/scripts/proof-add.mjs` with `--by grid-checker` and the checker brief's `subtask`, the one id, `_task` when it has none.

Tick plan items through the plan skill, and only with evidence. With several steps running, do this for each brief as it finishes, never for the whole set at the end.

Only when the task's work is in a git repo: copy the Outcome's `repo: <name> branch: <branch> base: <base>` lines into task.md's `repos:` in the section 5 edit, for a `blocked` brief too, one `  - {name: <name>, branch: <branch>, base: <base>}` item per repo, without `base` when the line has none. A repo already listed keeps its one item: update its `branch`, and keep its `base` unless that was empty. This is the only place a task's branches are recorded, in task.md in Grid's data folder; never write a branch note, file or setting into the repo.

## 5. Task files, at the boundary only

Always: in task.md set `updated` (the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess) and `next`, and add a body line `- <D Mon> · <step> · <what happened>`.

Move `step` only when the step is finished. A `run` step with a plan is finished when every plan step is ticked and no brief of it runs; until then `step` stays, and a finished brief only ticks its own plan steps:
- `run`, `make-together`, `check` passed, brief `done` and its Acceptance met: set `step` to the next one in `workflow`. "Check passed" means the check brief's Outcome says passed and its Acceptance lines are met; a brief `done` only means the command ran. A check brief whose Outcome says it edited project files does not count as passed: treat it as a failed check (the bullets below). If the next step is `approve`, start `grid-reviewer` on the worker's brief first; once it is `ACCEPTED` and the step is a ship approve, run the deliver skill's `## Before the gate`; then set `status: waiting` and name who in `next`. If it is `ask`, set `status: waiting` and name who in `next`.
- An `approve` step: you never move `step` forward off it, except back to the build step when a review returned must-fix (below). Grid moves `step` when the person approves; no skill does.
- The round limit is the `rounds` of the workflow item for the step the task is on; if that item has none, the `review rounds` line of the workflow template in `<projects folder>/workflows.md` or `<projects folder>/<project>/workflows.md` whose step names and kinds, in order, equal the task's (the later one wins); else 3. Round 1 is the first pass. Work goes back only while the new `round` is no greater than the limit; past it, raise a blocking question to the owner (the bullet after the next) instead of a new round.
- A review returned must-fix (an `approve` step's Verdict that is not `ACCEPTED`, section 1), and the new `round` is within the limit: send it back without a question. Set `step` to the step that built the reviewed brief (the template's "on reject, back to" step when it names one), `round` one higher, `status: reworking`. `next` says in the present tense what the rework changes ("Claude fixes the three must-fix items the review found, round 2"), never a question. Ask `grid-brief-writer` for a fix brief whose job is the Verdict's must-fix items. Add a body line to task.md and the history line `- <time> · sent back to <step> · round <n> of <limit> · review must-fix · by grid-planner` (`<time>` as below). Then carry on (section 6): the worker, then the reviewer again. The `approve` step is reached again only after the build and the checks pass.
- A review returned must-fix and the new `round` would be past the limit: leave `step` and `round`, and raise a blocking question to the task's `owner` through the questions skill. Name the must-fix items still open and a recommendation; options such as "One more round / Approve as it is / Stop the task". No amount of money in it.
- `check` failed, and redoing the step before it can fix the failures (within the round limit, otherwise as the next bullet): set `step` back to that step and `round` one higher (`2` the first time), `status: reworking`. `next` says in the present tense what the rework changes ("Claude fixes the date format the check flagged, round 2"), never a question. After a `make-together` step the rework is done with the person too, and `next` names them ("Claude and sam rework the team line the check flagged, round 2"). When the next agent for that step starts, set `status: running` again.
- `check` failed, and a rework cannot fix it: the failures sit in files the task must leave alone (intent.md says so, or another task owns them), or they need a choice only a person can make. Sending the step back would only loop. Leave `step` and `round` as they are and raise a blocking question to the task's `owner` through the questions skill, naming the failing items and your recommendation. That skill leaves the task `waiting` with the question in `needs` and `next` naming who decides.
- Brief `blocked` because a person must do something first (connect a browser, give a credential, start a machine, decide): a blocked brief with no open question is never a place to stop. Raise a blocking question to the task's `owner` through the questions skill now (status `waiting`), naming the action and what Grid does once it is done (it reruns the brief). The Stop hook blocks a running task that ends with a blocked brief and no open question, or whose `next` tells the person to resume or rerun.
- Brief `blocked` for any other reason, or the step is not finished: leave `step`, and set `status: stuck` only if no agent and no Grid action can go on and no person decision is missing (a decision is a question, status `waiting`); `next` never tells a person to start, resume or restart a session or rerun a brief. If a fresh session or a rerun is wanted, set `status: running` and let `next` name the step.
- The last step finished: do not close the task. The deliver skill writes the outcome; on a task with a ship approve step it wrote it before the gate, and after `ship approved` it refreshes it only if something changed.

Item records. When the brief has an `item:` line, record where its plan items stand with `node "$CLAUDE_PLUGIN_ROOT/skills/execute/scripts/item-record.mjs" <task folder> <state> <N[,N]> --by <role> [--env <id>]`, which appends the `- <time> · item N <state> ...` history line itself (never write these lines by hand):
- a worker or operator brief `done` whose items are built and now run on the local environment: `testing` (always `local`);
- a `check` brief (grid-checker or operator) whose Verdict or Outcome passed for those items: `testing` if nothing was recorded yet, and `deployed --env <id>` when the check ran on that environment of the project's environments (project.md, else `environments.json`), with `--by` the checker's role. The checker is read-only, so you run the script for it after reading its ACCEPTED Verdict;
- a check that failed on local: `failed`.
Skip it when the brief has no `item:`. Never invent an environment id: it must be one of the project's environments, else the script refuses and you leave the line out.

After that edit, run `grid-brain concept --task <id>` (the plugin's `bin/`) once, so the task's concept note in the project brain follows the new step and status. It is the one place this skill runs it; Workers never do. If it exits non-zero, carry on and add one history line saying it failed; it never stops the step.

Never write `runner`, `gate_raised`, `paused`, `lane`, `lane_step` or `lane_round`, never write an approval line, and never edit `intent.md`. history.md is appended by a hook as files change; if you add a line, its `<time>` is the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess, and you re-read the file after a "modified since read" error and try again.

## 6. Carry on

Once the task files are written, read `step`, `status` and `needs` again. If `status` is `running` or `reworking`, `needs` is empty, no brief is blocked and the new current step is a `run` or `check` step, go back to "Before starting" and from there to section 1 with that step now, in this turn. Do not write a message and wait for the person to say "carry on".

Inside a `run` step with a plan, every time one step ends (section 4), run the ready-set script again and start what became ready, before you wait for anything. Other briefs still running are never a reason to hold back what is ready; when nothing is ready and agents still run, the next one to finish starts the same loop, and the task stops only at the places in the opening list or when every plan step is ticked.

A review sent back to the build step (section 5) is carried on the same way: the current step is then a `run` step with `status: reworking`, so section 1 starts the worker on the fix brief now. Never write a message that asks the person whether to go back to build.

Do not go back when the current step is an `approve`, `ask`, `make-together` or `schedule` step, or when the last step's brief is done and its result is recorded in task.md: the task is done and delivery is next.

Stop at one of the places in the opening list, then tell the person which agents ran, the brief paths, the result of each step in a line, and what happens next or what waits on whom.
