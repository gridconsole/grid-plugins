---
name: views
description: Write or refresh a task's view pages in views/: progress.html, result.html and questions.html, from task.md, plan.md and history.md. Use when the person says "show me the progress", "update the views", "make the result page", "show the open questions", or after a step moves and the pages are out of date. plan.html belongs to the planner.
---

<!-- Decision source: plugin component table row 8 (views: views/*.html from task.md, plan.md, history.md); Q25 item 2 (a good skill and component library; pages written by subagents per role in parallel, no strong model). Default pending F1: the schema of views/*.html beyond "four derived pages" is open. -->

# Views

**Default pending F1.** A task folder has four derived pages in `views/`, and no other file there (the contract checker fails one): `progress.html`, `plan.html`, `result.html`, `questions.html`. A view is derived, never the source: it shows what task.md, plan.md, history.md and `questions/` say. Change the source first, then the page. Never write `plan.html` here: the planner writes it, through the plan-page skill.

The task folder is `<projects folder>/<project>/tasks/T-NN-slug/`. The projects folder is `GRID_PROJECTS`, else `$GRID_HOME/projects` (`GRID_HOME` is the Grid data folder `~/.grid/<name>`, never the workspace folder; if both are empty, Grid is not set up for this folder: say so and write nothing). If the person does not say which task, take the only open one, else ask. Never search the disk for it.

## Which pages

| Page | Role it answers | Reads | Shape |
|---|---|---|---|
| `progress.html` | where the task stands and whose turn it is | task.md, plan.md, history.md | `templates/progress.html` |
| `result.html` | what was made, and the checks that prove it | task.md (`outcome:`, and the `## Outcome` blocks the deliver skill writes, before a ship approve gate and again after `ship approved` when something changed: Summary first, then Before and after, Not in it and After the deploy when written), artifacts/, evidence/, briefs' Outcome | `templates/result.html` |
| `questions.html` | what waits on a person | `questions/*.md`, task.md `needs` | `templates/questions.html` |

Write only the pages the person asked for, or the ones whose source changed. `result.html` appears once there is a result to show; before that, do not write it. An empty `questions.html` is not written either: say there is nothing open.

## Style and parts

Pages use the Grid Console design tokens (`tokens.css`) and the small page frame in `templates/views.css`: classes prefixed `v-` (`v-head`, `v-section`, `v-track`, `v-step`, `v-list`, `v-row`, `v-meta`) that lay a page out over the tokens. The templates are page frames, not the plan-page component library (`plan.html` alone is built from that); copy their markup and keep to their `v-*` classes. `components.css` is still linked because it holds the shared base rules (body reset, link colour) the frames sit on.

1. Make `<task folder>/ds/` (absolute, as above) if it is missing and copy into it `tokens.css` and `components.css` from `${CLAUDE_SKILL_DIR}/../plan-page/reference/`, and `views.css` from `${CLAUDE_SKILL_DIR}/templates/`: `cp` each, one command per file (`mkdir` and `cp` are the only shell use here). Never edit them. Nothing is written relative to the working directory, which is the project's repo.
2. Start from the page's file in `templates/`: copy it with the Write tool (not `cp`) to the absolute path `<task folder>/views/<page>.html`, and replace every `{{...}}` with real content. A page that still holds `{{` is not finished.
3. Link `../ds/tokens.css`, `../ds/components.css` and `../ds/views.css`, in that order. No literal colours, sizes or radii, no inline `style` with them, no other stylesheet. States are a glyph and a word (`✓ done`, `● running`, `! waits on you`, `■ stuck`, `○ planned`).
4. The headline states the conclusion ("Step 2 of 3 is running; nothing waits on you"), not the topic. The `workflow` is read from task.md; never draw a fixed phase bar.
5. Times are absolute. Numbers carry a sentence. Evidence (command output, errors) is copied, not reworded. Absence is one line, never an empty box.

## Who writes them

The planner writes the pages, through this skill, with the Write tool, one page at a time. `grid-builder` never writes them and never edits task.md; there are no briefs for views. Before you say it is done, check parts 2 and 3 above on each page.

Afterwards task.md gets `updated` (the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess) and a body line `- <D Mon> · <step> · views refreshed`. history.md is not touched by this skill.

Tell the person which pages were written and where they are.
