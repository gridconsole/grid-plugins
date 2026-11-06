---
name: research
description: Research a question for a task, or map where something is in the code, and write research.md in the task folder. Use when a task needs facts before a plan or a build ("research how X works", "find out what we need for Y", "where is Z handled", "look into it first").
---

<!-- Decision source: plugin component table row 2 (research starts researcher subagents, writes research.md); Q23 (Sonnet researches, Haiku maps); Q27 item 1 (a subagent writes briefs); Q22 (subagents may start subagents). -->

# Research

Facts first, then the plan. The work goes to an agent through a brief; this skill picks the agent, gets the brief written and reads the result.

The task folder is `<projects folder>/<project>/tasks/T-NN-slug/`. The projects folder is `GRID_PROJECTS`, else `$GRID_HOME/projects` (`GRID_HOME` is the Grid data folder `~/.grid/<name>`, never the workspace folder; if both are empty, Grid is not set up for this folder: say so and write nothing). If the person does not say which task, take the only open one, else ask. Never search the disk for it. Read `intent.md` and `task.md` first.

## Steps

1. **The question.** One sentence the research answers, taken from `intent.md`, the plan step or the person's words. If it is wider than one question, split it into briefs, one question each.
2. **The agent.**
   - Facts, comparisons, how something works, anything from docs or the web: `grid-researcher`. It writes `research.md`.
   - "Where is it, which lines": `grid-scout`. It only reads; it cannot write files, so its reply is the map.
   - Both, when the files are not obvious: scout first, then the researcher with the map in the brief's Context.
   - Research before a plan always runs `grid-researcher`, after a scout when the files are not obvious: the plan cites `research.md`, and a scout's map alone is not research.
   If `routing.md` has a rule for this project or for the researcher/scout role, the routing skill's lookup decides; otherwise the above.
3. **The brief.** Ask `grid-brief-writer` for it: the task folder path, the step, the role (`researcher` or `scout`) and the question in one sentence. It writes `subagents/NN-name.md` with status `briefed`. You do not write briefs, and an agent may run in the background: wait for its reply before you read the brief. If the brief is not there yet, wait; never write the brief, and never do the agent's job yourself.
4. **Run it.** Set the brief's `status` to `running` (Edit, that line only), start the agent and tell it the brief's path. Independent questions go in parallel.
5. **Read the result.**
   - Researcher: `research.md` must exist in the task folder and the brief's Outcome names it. A second question appends a section; it never replaces the first. Set the brief `done`.
   - Scout: write its reply into the brief's `## Outcome` (the scout cannot), and when `research.md` is wanted, add it there under `## Map` with the source paths. Set the brief `done`.
   - No result, or the agent says it is blocked: set the brief `blocked`, say what is missing, stop.
6. **Task files.** In `task.md` set `updated` (the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess) and `next` (usually "Plan the task" or "Alex reads the research"), and add a body line `- <D Mon> · <step> · research.md written`. Leave `step` where it is; moving it is the execute skill's job at a step boundary. Do not touch `intent.md`.

Findings stay in `research.md`; do not copy them into `task.md`. Research changes no code. If a finding makes the person's ask unclear, put it to them through the questions skill.
