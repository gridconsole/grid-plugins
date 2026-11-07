---
name: routing
description: Read or change a project's routing.md rules for which agent runs which kind of step. Use when the person asks who handles a step ("which agent runs checks?"), wants a rule ("always use the designer for make-together steps in notes-app"), or the execute skill needs the agent for a step.
---

<!-- Decision source: plugin component table row 10 (routing reads and writes routing.md rules for which agent gets a step); Q22 (workers may start workers, so routing never limits nesting); routing.md shape from the project's own routing.md (Topic | Goes to | Rule | Why, then Members). -->

# Routing

`routing.md` sits beside `tasks/`: `<projects folder>/<project>/routing.md`. It is optional. Without it, the execute skill uses the planner's table of step kinds. The projects folder is `GRID_PROJECTS` (`printf %s "$GRID_PROJECTS"`); if it is empty it is `$GRID_HOME/projects` (`printf %s "$GRID_HOME"`; `GRID_HOME` is the Grid data folder `~/.grid/<name>`, never the workspace folder; if both are empty, Grid is not set up for this folder: say so and write nothing); never search the disk for the file.

## The file

The file already has a title, a table of topics and who answers them, and `## Members`. Leave all of that as it is. Agent rules live in their own section, added at the end if it is missing:

```
## Agents

| Applies to | Agent | Why |
|---|---|---|
| kind: check | grid-runner | Checks are commands |
| kind: make-together, project: notes-app | grid-designer | The person steers a mock |
| project: notes-app | grid-builder | Default builder here |
| kind: check, project: notes-app | grid-checker | UI change in `app/`: `grid-builder` with the design skill, then `grid-checker` |
```

- **Applies to**: `kind: <kind>` (check, ask, approve, make-together, run, schedule), `project: <name>`, or both separated by a comma.
- **Agent**: one of `grid-scout`, `grid-researcher`, `grid-runner`, `grid-builder`, `grid-reviewer`, `grid-designer`, `grid-checker`, `grid-operator`. Not `grid-planner` (one per task, it runs the task) not `grid-brief-writer` (it writes briefs, nothing else) and not `grid-curator` (the planner starts it for brain proposals). Nothing here limits whether an agent may start others.
- **Why**: a few words. Never empty.

`grid-worker` is the old name of `grid-builder`: a `grid-worker` row, or an `agent: grid-worker` in a workflow, means `grid-builder`.

## Look up the agent for a step

Given the task's project and a step's kind, take the rules whose every condition matches. The most specific wins: both `kind` and `project`, then `kind` alone, then `project` alone. Two rules of the same rank naming different agents: do not pick; tell the person which rows clash and ask. No match: say so, and the caller falls back to the step-kind table. A kind of `ask` or `approve` is never given to a worker; if a rule says so, report it as a conflict with the kind table and ask.

## Show

Print the rules as a short list, most specific first, and say which rows the topic table and members hold. Change nothing.

## Add, change or remove a rule

Only when the person asks, and only the rule they state. Never invent a rule, fill a gap, "tidy up" or reorder rows they did not mention.

1. If the request is unclear (which kind, which project, which agent), ask. If the agent is not one of the eight, say so and ask.
2. If a rule with the same `Applies to` exists, show it and ask whether to replace it; replace only on yes.
3. Edit with the Edit tool, never a shell redirect. A new rule goes in as one row at the end of the table (lookup ranks rows by specificity, so order does not matter). Every other line stays as it is, including the spacing of the other rows. No `routing.md` yet: create it with a title `# Routing · project <name>` and the `## Agents` section only, after telling the person.
4. Read the changed section back and report the new rule in one line.

A rule changes which agent starts next time. It does not reroute a brief that is already `running`.
