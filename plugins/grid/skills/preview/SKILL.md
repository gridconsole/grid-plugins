---
name: preview
description: Preview a Large task before it is built: have a spec.md written and put it in front of the person to approve. Use when a task with size large reaches its preview step, or the person says "show me the design first", "write the spec", "preview it before you build". Not for small or normal tasks.
---

<!-- Decision source: plugin component table row 4 (preview: Large tasks only, spec.md before execute); Q4 (no separate size approval, the agent decides the size from the answers); Q13 (no fixed phases, so the preview is a workflow step of kind approve); Q27 item 1 (briefs are written by grid-brief-writer). Default pending Q4: whether a Large task always gets a preview step is open. -->

# Preview

**Default pending Q4.** A Large task gets a design before it is built: `spec.md`, written by `grid-designer`, then a person approves it. There is no separate approval of the size; you read the size from task.md, which the task skill set from the person's answers.

The task folder is `<projects folder>/<project>/tasks/T-NN-slug/`. The projects folder is `GRID_PROJECTS`, else `$GRID_HOME/projects` (`GRID_HOME` is the Grid data folder `~/.grid/<name>`, never the workspace folder; if both are empty, Grid is not set up for this folder: say so and write nothing). If the person does not say which task, take the only open one, else ask. Never search the disk for it. Read `task.md`, `intent.md`, and `research.md` and `plan.md` if they exist.

## Is this a preview?

- `size` is not `large`: say that only a Large task has a preview and stop. Write no `spec.md` (the contract checker fails a spec on a small or normal task). If the person thinks the task is large, they say so and the task skill changes `size`.
- `status` is `closed`, `paused` or `pausing`: say so and stop.
- `step` is an approve step other than `preview` and `status` is `waiting` (for example the plan's approve step): that gate waits on the person first. Say so and stop; do not move `step`, and write the spec only if the person asks for it now.
- history.md already has a `preview approved` line for the current plan or spec: the preview is done. Say so, and hand on to the execute skill. Do not write it again.

## 1. The spec

1. If `spec.md` exists, read it. Rewrite it only if the person asked for changes.
2. Otherwise ask `grid-brief-writer` for a brief: the task folder path, the step name and kind, role `designer`, and the job in a sentence ("write spec.md for this task from intent.md"). It writes `subagents/NN-name.md` with status `briefed`. You do not write the brief yourself.
3. Set the brief's `status` to `running` (Edit that line only), start `grid-designer` and tell it the brief's path.
4. Read the brief's Outcome and `spec.md`. The spec must start from the person's words in intent.md and say what is built, what the person sees and does, what is left out and what is still open. If it does not, send it back with a new brief; do not patch it yourself. Set the brief `done` only when the Outcome names `spec.md`.

## 2. The approve step

The preview is a workflow step of kind `approve`. The preview step is the approve step named `preview`, and it sits after the spec is written (the designer's run) and before the build. The plan's approve step before the first `run` step does not count as it, and neither does a review after the build.

- If an existing approve step is the one the person calls the spec sign-off, it counts: rename it `preview` and add nothing.
- If task.md `workflow` has no such step, add one in the same shape (`  - {step: preview, kind: approve}`, names `a-z`, `0-9` and `-`) just before the build's first `run` step, and keep `step` on one of the names. The preview never makes a workflow longer than 6 steps: if it is full, add nothing and raise a question with the questions skill instead (which step the preview replaces, or whether to skip it).
- First start `grid-reviewer` on the designer's brief from part 1 (the one whose Outcome names `spec.md`), as the planner and the execute skill do for every approve step; it writes a Verdict only into that brief.
- Then set `step` to the preview step, `status: waiting`, `updated` to now (`date +%Y-%m-%dT%H:%M`), and `next` to "The person reads spec.md and approves it, or asks for changes." Add a body line `- <D Mon> · preview · spec ready`.

## 3. Stop

You never pass the gate. A person approves the preview in Grid, and Grid writes the `preview approved` line in history.md. Never write `preview approved`, `plan accepted`, `ship approved` or `phase set`, in history.md or anywhere else. Do not start the execute skill, a worker or the plan item that follows.

Tell the person the path of `spec.md`, the one-line summary of what it proposes, and that the build waits for their approval. If they ask for changes, edit nothing yourself: write a new brief for the designer with their words, and repeat from step 1.
