---
name: questions
description: Ask a person a question, record their answer, or withdraw a question, in the task's questions folder. Use when the person says "ask Sam ...", "ask <name> which ...", gives an answer to an open question ("Sam says ..."), or a question is no longer needed.
---

# Questions

The task folder is `<projects folder>/<project>/tasks/T-NN-slug/`. The projects folder is `GRID_PROJECTS`, else `$GRID_HOME/projects` (`GRID_HOME` is the Grid data folder `~/.grid/<name>`, never the workspace folder; if both are empty, Grid is not set up for this folder: say so and write nothing). If the person does not say which task, take the only open one, else ask. One file per person asked: `<task folder>/questions/<person>.md` (the person's first name, lowercase, only `a-z`, `0-9` and `-`, such as `sam.md`; the same name goes after `to:`). Only the planner writes in this folder, one writer at a time. A subagent that needs an answer puts the question in its report; the planner records it. Every path below is under that absolute task folder; nothing is written relative to the working directory, which is the project's repo.

## Ask

Never ask a question about a refused command or offer to add an allow rule: Grid shows a refusal to the person itself and resumes the task with the decision.

0. Pick `<person>`, unless the person named one: (a) the role whose description fits the question (the project's `project-roles.json`), else (b) the person entry in the brain (`knowledge/people/`) with an `owns` or `knows` link, or a "go to" line, for the area, else (c) the owner. Write the reason in the block as `Routed: role <id>` or `Routed: brain <handle>` (nothing for the owner). The `Routed:` line is the last line of the block, after `Answer:` (never beside `asked:`, where it would read as question text).
1. Find the task folder. Read every file in `questions/` and take the next free id: `Q-n`, counted over the whole task, not per person.
2. Append a block to `questions/<person>.md` (create the file if it is new), separated from any earlier block by a blank line. Every `<time>` in this skill (here, in the answer, in the history lines) is the output of `date +%Y-%m-%dT%H:%M`, run just before the write, never a guess. Write with the Write or Edit tool, never a shell heredoc or redirect.

```
## Q-2 · to: sam · not blocking · open
asked: 2026-03-04T10:30 · by grid-planner

Should the tag box show recent tags?
Context: the mock lists the last five tags under the box.
Options: Yes, five / Yes, ten / No
Recommendation: yes, five, to save typing.
Answer: —
Routed: brain sam
```

- Heading: `## Q-<n> · to: <person> · blocking|not blocking · <state>`. State is `open`, `answered`, `withdrawn` or `disputed`. `blocking` means work cannot go on without the answer. Never write `later` yourself: Grid writes it for the questions of a setup task that wait their turn. In a setup task you add a follow-up as `open`, with a `group: followup` line under the heading.
- One question per block, in plain words. Context says why it matters; Options is `A / B / C` and may be left out for an open question; Recommendation is always there and starts with your pick and a reason. A question with no recommendation is not ready to ask.
- Options are concrete answers only; never add "Answer in my own words", "Other" or "Something else", because the card always offers its own free answer.
- The Answer line stays `Answer: —` until the person answers. `Routed: role <id>` or `Routed: brain <handle>` is the last line, only when the person was picked by role or brain.
- No money anywhere: a question, task.md, a plan or a history line never states a price, a cost or an estimate in a currency (no currency sign or currency word next to a number). Say it in tokens, minutes or a share of the usage limit, or leave it out.
- A step or task that waits on a person always has an open question; never set `status: waiting` on a person with none. For a person's own action (for example "ask the client to create a Mollie account") write what they must do, with Options `Done, carry on / Not yet / Answer in my own words`; on Done, record the answer and carry on.
- Never ask whether to rework. A reviewer's must-fix items go back to the step that built the work and the task carries on, with no question, until the workflow's review rounds are used up (see the execute skill). Only then ask, and name the must-fix items still open.
3. Append `- <time> · question asked · Q-<n> · by grid-planner` to `history.md` (Edit tool, not a shell append).
4. In task.md: for a blocking question add `Q-<n>` to `needs: [...]`, set `status: waiting`, set `next` (for example "Sam answers Q-2"), set `updated` (the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess), and add a body line.
   While the plan skill writes a plan, the planner leaves this step to the plan skill's last step, after the plan's review, so the task does not wait before the reviewer has read the plan.
5. Tell the person who was asked and what you recommended.

## Record an answer

Only when the person gives the answer in their own words (or says what another person said and names them). Never invent or guess one.

1. In the block, replace `Answer: —` with `Answer: <their words>. (<time>, <who>)` and change the state in the heading to `answered`.
2. Append `- <time> · question answered · Q-<n> · by <who>` to `history.md` (Edit tool, not a shell append).
3. In task.md remove `Q-<n>` from `needs`; if `needs` is now empty and `status` was `waiting`, set it to `running`. Set `next` to what happens now that `Q-<n>` is closed (it must not still name `Q-<n>`) and set `updated` (the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess).
4. When the question was raised because a brief is `blocked` on the person's action, the answer reruns that brief: the next step is the execute skill rerunning it (status `running`, `next` naming the step); never ask the person to resume or rerun.
5. Never edit another block, and never change an answer once recorded; a new answer is a new question.

## Withdraw

When a question is not needed any more (`<time>` as above): change the state in the heading to `withdrawn`, add a line `withdrawn: <time> · <reason>` under `asked:`, leave the Answer line as it is, append `- <time> · question withdrawn · Q-<n> · by grid-planner` to `history.md`, clear it from `needs` as above, and set `next` to what happens now (it must not still name `Q-<n>`).
