---
name: task
description: Create a Grid task folder or show a task's state. Use when the person starts a task ("start a task", "new task", "I want to add ..."), or asks where a task stands ("show task T-02", "what's the state of ...", "list my tasks"). Writes intent.md from their own words and task.md from the fields below.
---

# Task

A task is a folder: `<projects folder>/<project>/tasks/T-NN-slug/`. The projects folder is `GRID_PROJECTS`; read it with `printf %s "$GRID_PROJECTS"`. If it is empty, it is `$GRID_HOME/projects`, where `GRID_HOME` is the Grid data folder `~/.grid/<name>`, never the workspace folder (read it with `printf %s "$GRID_HOME"`). If both are empty, Grid is not set up for this folder: say so, write nothing and create no task, never one under a repo (Grid writes nothing into a project's repo folder). Never guess a default and never write outside that folder.

## Start a task

1. **Project.** The folder name in the projects folder. Use the one the person names; else the one matching the working folder's name; else the only one. If still unclear, ask.
2. **Id.** Run `node "${CLAUDE_SKILL_DIR}/scripts/next-id.mjs" "<projects folder>"` as a single command on its own (absolute path, no `cd`, `&&`, `$PWD` or chaining), so a person's allow rule for `node` matches it. Ids are workspace-wide, so T-03 may sit in a different project than T-02. Slug: the title in 2 to 5 lowercase words joined by `-`, only `a-z`, `0-9` and `-`. Folder: `T-NN-slug`.
3. **Owner.** The person who asked. Take the name from the conversation ("I'm Alex"), lowercase, first name, only `a-z`, `0-9` and `-`. If you do not know it, ask. It is set once; you never change it later.
4. **intent.md.** Create it with the Write tool, never a shell heredoc or redirect. The person's request word for word, no heading, no rewording, no fixes to spelling. If they said it over several messages, join them with blank lines.
5. **task.md.** Create it with the Write tool, never a shell heredoc or redirect. Frontmatter, then a body log. Get times with `date +%Y-%m-%dT%H:%M`. The block below is one example, for a code change; pick your own steps (see "Choosing the steps").

```
---
id: T-02
title: Search notes by tag
project: notes-app
owner: alex
source: idea
size: normal
workflow:
  - {step: plan, kind: approve}
  - {step: build, kind: run}
  - {step: review, kind: approve}
step: plan
status: running
updated: 2026-03-04T10:32
summary: Let people filter the note list by tag.
next: Write the plan.
---

- 4 Mar · plan · task created
```

| Field | Rule |
|---|---|
| `id`, `title`, `project`, `owner` | as above; title is a short phrase in the person's words |
| `source` | `idea` (the person's own ask, the default), `signal`, `note` or `task` (made from another task) |
| `size` | `small` (one change in one place), `normal`, or `large` (needs a design or a preview). Your call; the plan can change it |
| `workflow` | the task's own steps, 2 to 6, one `  - {step: <name>, kind: <kind>}` line each. Names are `a-z`, `0-9` and `-`, each used once. There is no fixed phase list: see "Choosing the steps" below |
| `step` | the step now running; always one of the `workflow` names. Move it as the work moves |
| `round` | optional; a positive integer, written only when the work loops back to an earlier step: `2` the first time it goes back, then `3`, and so on |
| `status` | `running`, `waiting` (needs a person), `queued`, `stuck`, `reworking`, `pausing`, `paused` or `closed` |
| `updated` | the time of this write, the output of `date +%Y-%m-%dT%H:%M`, run just before this write, never a guess; set it on every change to task.md |
| `summary`, `next` | one line each: what the task is, and what happens next and who does it. Never leave either empty |
| `needs` | optional list of what waits on a person, such as `[Q-1]`; the questions skill keeps it |
| `outcome` | optional; one line, written by the deliver skill only: what was delivered, with the main path |
| `due` | optional; only when the person names a deadline: a local minute `YYYY-MM-DDTHH:MM`, no seconds and no zone |
| `repos` | optional; only when the task's work is in a git repo: the repos and branches its changes are on, one `  - {name: <repo>, branch: <branch>, base: <branch>}` line each. `name` is the repo's folder name (the folder project.md names, or a repo directly inside it), each used once; `branch` is the branch the changes are on and `base` the branch it started from. The execute skill writes it from the worker's Outcome; never write it at intake |

Never written: `branch`, `worktree` (the host makes one branch and worktree per task) and `phase` (no fixed phases) are gone, and a task.md with them fails the check; a task's branch goes under `repos`, never as a `branch:` field. `runner`, `gate_raised`, `paused`, `lane`, `lane_step` and `lane_round` are Grid's.

### Choosing the steps

Pick the steps from what the task delivers, not from the examples. A code change ends in a PR someone reviews; a site change ends in a deploy at a set time; a report ends in a PDF a person signs off; a newsletter ends in a send. Each needs different steps, and a small task may need only two. One line each, as a starting point:

- PR: `plan` approve, `build` run, `tests` check, `review` approve
- deploy: `fix` run, `staging` check, `sign-off` approve, `deploy` schedule
- PDF: `outline` approve, `draft` run, `sign-off` approve
- newsletter: `draft` make-together, `links` check, `approve` approve, `send` schedule

A `check` goes right before the step it guards: the tests before the review, staging before the deploy, the links before the send.

Each step has one kind, exactly one of these six:

| Kind | The step is |
|---|---|
| `check` | a quick test Claude runs before going on: an API key is there, there is disk space, staging answers |
| `ask` | a question to a person that the work waits on |
| `approve` | a person says yes to something: the idea, the plan, a customer's sign-off |
| `make-together` | a person steers in a canvas or a draft while Claude builds it |
| `run` | Claude works on its own; it may take minutes or days |
| `schedule` | something happens at a set time: a send, a deploy |

A second example: a fix to a live site, where staging failed once and the work went back to `fix`:

```
workflow:
  - {step: fix, kind: run}
  - {step: staging, kind: check}
  - {step: sign-off, kind: approve}
  - {step: deploy, kind: schedule}
step: fix
round: 2
```

6. **history.md.** Create it with the Write tool, never a shell heredoc or redirect, with the first line: `- <time> · created · by <owner>`. Every `<time>` in history.md is the output of `date +%Y-%m-%dT%H:%M`, run just before the write, never a guess. Append only. Write only these events: created, plan ready, question asked, question answered, question withdrawn, delivered (the deliver skill writes that one). Never write `plan accepted`, `preview approved`, `ship approved` or `phase set`; those are a person's approvals and Grid's.
7. Tell the person the folder path, the id and the workflow you chose, with each step's kind.

Do not plan, research or build in this skill. Starting a task only makes the folder.

## What else lives in the folder

Other skills add their own entries; none of them is a stray, so do not delete or tidy them: `research.md` (research), `plan.md` (plan), `spec.md` (preview, Large tasks only), `questions/<person>.md` (questions), `subagents/NN-name.md` (briefs), `artifacts/` and `evidence/` (the work and its proof), `views/` (views, the derived pages), `ds/` (views: copies of the design stylesheets the pages link) and `memory/<step>.md` (memory: short dated lines, one file per workflow step). Starting a task makes only `intent.md`, `task.md` and `history.md`.

## Show a task

Read `task.md` of the named task (or list `<projects folder>/*/tasks/` with each task's id, title, step and status). Report: id, title, owner, size, step and where it sits in the workflow, status, `needs`, `next`, and how many plan steps are ticked and how many questions are open. Read only; change nothing.

## Updating task.md later

Use the Edit tool, never a shell redirect: edit the field, set `updated` (the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess), and add a body line `- <D Mon> · <step> · <what happened>`. Never edit `intent.md`: it is the person's words. If they change their mind, add a dated paragraph to the end of it with their new words.
