---
name: run-job
description: How a builder runs one Grid Job and hands off a proof table. Use when a Job is assigned to you.
---

Read the Job with `grid job show <task> <job>`. It is your whole brief. Owner decisions that apply: no Grid-made guards, caps or limits (Claude Code built-ins only); everything is a Job and the Job is the brief; carry on by yourself until a person point; no plan mode.

Steps:

1. Note the files you may write and the out-of-scope list. Touch nothing else. If the Job needs more, say so in the result; do not widen the scope.
2. Read each code fact again in this session before you rely on it, with a `path:line`. Do not trust the Job's own line numbers without reading them.
3. For a move or removal, write the list of items first and work through it one by one.
4. For UI, every field comes from the data source the Job names (served op answer, `frame/model.mjs`, decision doc). A field the read lacks is NOT DONE, not a guess. Check at widths 390, 1440 and 3840.
5. If the Job will not fit in its `budget.turns_estimate`, stop at the point it stops fitting and say which rows are left. Do not rush it.
6. A new test fails on the base and passes on your change; paste the failing lines. Never edit an existing test to make it pass.
7. Run `grid job verify <job> --task <task>` and the Job's verify command.
8. Post the result with `grid job result --task <task> --job <job> --body <json>`. If it exits 3 the lease or record was rejected: do not retry.

The result is a proof table: one line per acceptance row, each either

- `HOLDS` with the command and its exit code, or a `path:line`; or
- `NOT DONE` with why.

Then what changed, in a few lines, each verify command with its exit code, and what is NOT VERIFIED. A row you could not prove is NOT DONE; never write HOLDS for a row you did not check. Add evidence with `grid proof add <item> <file> --task <task>`. If blocked, post the result with the blocker named and follow the Job's `on_blocked`.

Done: every row has a line, the verify exit codes are pasted, and the result is posted.
