---
name: task
description: Start a Grid task from the person's own words, or show where tasks stand. Use when the person says "start a task", "I want to add", "show task T-02" or "list my tasks".
---

# Task

A task is a folder in the Grid home, made and read only by the `grid task` verbs. Never write `task.md`, `intent.md` or any file of a task by hand, and never put anything in the person's repo.

Owner decisions that apply here:

- "Everything is a Job, the Job is the brief." Starting a task only makes the task; the work is planned into Jobs later (skill plan-jobs).
- "Tasks carry on by themselves until a person point." Do not stop to ask the person to approve starting.
- No plan mode. Do not enter it and do not ask for it.
- No Grid-made guards, caps or limits: do not add a budget, a step limit or a hold to a task. Claude Code's own settings do that.

## Start a task

1. Take the person's words as they typed them. If they gave several messages, join them. If they typed nothing and only dropped files, ask what they want done; the files are evidence, never the intent.
2. Read any files they named or dropped (text, csv, json, md, images and pdf with the Read tool; docx with `pandoc <file> -t plain`; xlsx with a short read-only script). Never move, edit or delete them. Name each file you could not read, and why, in your reply.
3. Run `grid task new "<their words>"`. Add `--project <name-or-id>` when they named a project, `--size small|normal|large` only when it is obvious, and `--workflow <name>` only when they named one. Without `--project` the folder decides the project. The command keeps their words word for word, picks the id and the owner, and prints the id and the path.
4. Tell the person the id, the path and the project. If the command exits non-zero, say what it printed and stop; do not make the folder yourself.

When the person asks to start a task from a signal, run `grid signal act <signal> --act turn` instead (skill signals): it makes the task from the signal's title and names the signal's note in the task's words, so the link back is there.

## Show or list

- `grid task show <task>` (or no operand inside a task folder): id, title, project, size, workflow, state, path, the person's words, and the plan hash when a plan exists. `<task>` is an id such as T-02, a folder name or a path.
- `grid task list [--project <name-or-id>] [--state <state>]`: one line per task, id, state, title.
- `grid task history <task>`: the dated lines of what happened.

Read only. Report what the command printed; change nothing.

## Done

The task exists, `grid task show <task>` prints the person's words unchanged, and the person has the id and path. Nothing else was written.
