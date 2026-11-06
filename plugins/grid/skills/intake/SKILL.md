---
name: intake
description: Turn what a person typed, and any files they dropped into the task's inputs folder, into the task's intent.md and task.md. Use when a task starts from an email, a note, a screenshot or a pasted brief ("here is what the customer sent", "start a task from inputs/", "I dropped the files in the task folder"), or when a task folder holds inputs/ but no task.md yet.
---

<!-- Decision source: plugin component table row 11 (intake turns intent.md and inputs/ into task.md); Q13 (workflow steps have kinds, no fixed phases); the task skill owns the task.md format. -->

# Intake

Intake is how a task begins when there is more than one sentence to go on. The result is the same as the task skill's: `intent.md` and `task.md` (plus `history.md`) in a task folder. The format of `task.md`, the id rule and the choice of workflow steps are the task skill's; load it and follow it. This skill adds the reading of `inputs/`.

The projects folder is `GRID_PROJECTS` (`printf %s "$GRID_PROJECTS"`); if it is empty it is `$GRID_HOME/projects`, where `GRID_HOME` is the Grid data folder `~/.grid/<name>`, never the workspace folder (`printf %s "$GRID_HOME"`); if both are empty, Grid is not set up for this folder: say so and write nothing, never a task under a repo. Never search the disk for a task folder and never write outside the projects folder.

## Where the folder is

- The person points at a folder, or names a task, that already holds `inputs/` and no `task.md`: use it. Its name is the id and slug (`T-NN-slug`); its project is the folder under the projects folder it sits in.
- Otherwise make the folder as the task skill does (project, next id from its script, slug) and put nothing in `inputs/` yourself: the person's files go there, or are named by path and you copy them in with Bash `cp`.
- A folder that already has a `task.md` is not new: say so and stop. Intake never rewrites a task.

## Steps

1. **List `inputs/`.** Read every file in it (text, markdown, csv, json, images and PDFs through Read). Never move, rename, edit or delete one; `inputs/` is the person's.
2. **intent.md.** The person's typed text, word for word, written with the Write tool. If they typed nothing and `intent.md` already exists, leave it as it is. If they typed nothing and there is no `intent.md`, ask what they want done; do not write one from the files. The inputs are evidence, not the intent.
3. **task.md.** Frontmatter as the task skill defines it. From the typed text and the inputs decide:
   - `title`: a short phrase in the person's words;
   - `project`: the person names it, else the folder it sits in, else the one matching the working folder, else ask;
   - `size`: `small`, `normal` or `large`; more and messier inputs push it up, one clear change keeps it small;
   - `workflow` and `step`: 2 to 6 steps with kinds, chosen from what the task delivers (the task skill's "Choosing the steps"); `step` starts on the first;
   - `status`: `running` if the first step is `run`, `check` or `make-together`; `waiting` if it is `ask` or `approve` (set `next` to who answers); `source`: `idea` unless the inputs are a signal or a note;
   - `summary` and `next`: one line each, never empty. The summary says what the inputs show in your words; the next line says who does what next.
   Add a body line `- <D Mon> · intake · read inputs/<file>, ...` naming the files read.
4. **history.md.** First line `- <time> · created · by <owner>` (Write tool; `<time>` is the output of `date +%Y-%m-%dT%H:%M`, run just before this write, never a guess). Owner is the person who asked, lowercase first name.
5. Tell the person the folder path, the id, the size, the workflow with each step's kind, and which input files you read. If a file could not be read, say which and why.

Intake ends at step 5. Do not plan, research or build here, and do not run the first step, even if the person says "start the task": the task is started when its folder exists and `step` is on the first workflow step. Leave `step` there. The person asks for the work to go on in a later turn, and then the plan, research or execute skill takes it.
