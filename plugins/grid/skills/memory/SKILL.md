---
name: memory
description: Keep what a task taught as short dated lines in its memory/ folder, one file per workflow step. Use when the person says "remember this", "note for next time", "add to the memory", or when a step or task ends and something surprised us, failed once, or was decided for a reason worth keeping. Leaves Claude's own memory alone.
---

<!-- Decision source: Q43 item 5 (a task keeps what it learned in memory/ only, no separate learnings file); plugin component table row 9. Promotion decided by Abdu, 2 Nov: task memory reaches the brain as proposals made at deliver and accepted in the end review; it never reaches Claude's own memory. -->

# Memory

A task keeps what it learned in its `memory/` folder (Q43 item 5), and only there: short dated lines, one file per workflow step. Nothing else. This skill does not write Claude's memory (`MEMORY.md`, memory folders, `CLAUDE.md`) or any file outside the task folder; a learning that would help another task reaches the brain only as a proposal at deliver (Promotion, below), never written there directly.

The task folder is `<projects folder>/<project>/tasks/T-NN-slug/`. The projects folder is `GRID_PROJECTS`, else `$GRID_HOME/projects` (`GRID_HOME` is the Grid data folder `~/.grid/<name>`, never the workspace folder; if both are empty, Grid is not set up for this folder: say so and write nothing). If the person does not say which task, take the only open one, else ask. Never search the disk for it. Every path below is under that absolute task folder; nothing is written relative to the working directory, which is the project's repo.

## What goes in

A learning is one thing a later task, or the next step of this one, would do better for knowing: a check that kept failing and why, a choice and the reason, a command that is not obvious, a thing the person wants done every time. Not progress ("step 2 done": history.md and task.md have that), not a summary of the work, not a guess. If you cannot name where it came from (a file, a brief, an answer by the person), do not write it.

## Layout

`memory/<step>.md`, where `<step>` is a name from the task's `workflow` in task.md: the step the learning came from, or the current `step` if it came from no step. One file per step, not one per learning, so a later task reads a few files, not many, and the file name already says where a learning came from.

## Format

```
# Memory: build

- 2026-03-04 · The tag chip in the design system already handles long names; reuse it. (Q-2, Sam)
- 2026-03-05 · `node --test` needs the store seeded first or the filter test is flaky. (subagents/03-tests.md)
```

- Line 1 is `# Memory: <step>`, the same step as the file name. One line per learning, newest last, `- <YYYY-MM-DD> · <what we learned>`. Get the date with `date +%Y-%m-%d`.
- One sentence, in the person's words where they gave it, plain words, no em dashes. Name the source in brackets at the end: a question id, a brief, a file.
- Never reword or delete an earlier line. If a learning turned out wrong, add a new line that says so.

## Write it

1. Read `memory/<step>.md` if it exists. A learning already there in other words is not added again.
2. If the file is missing, create it with the Write tool and the heading above (the `memory/` folder comes with it); otherwise add the line at the end with the Edit tool. Never a shell heredoc or redirect.
3. In task.md set `updated` (the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess), and add a body line `- <D Mon> · <step> · learning noted`. Change no other task.md field.
4. Tell the person the line you wrote and the path.

## Promotion

At deliver (the deliver skill), each memory line that would help another task becomes a brain proposal: `brain-proposals/<name>.md` in the task folder, one fact per proposal, with the brain entry's frontmatter (`name`, `type`, `description` of at most 200 characters, `updated`, `source` naming the memory file and line, `superseded_by:` empty, `links`), `level: project` (or `workspace` for the Grid home's brain), `status: proposed` and `reason:`, then a short body. A line that only matters to this task stays in `memory/`. A `grid-curator` reviews each proposal, rewrites it as a linked note and writes the accepted ones into the brain, with no person asked; this skill never writes the brain.
