---
name: run-job
description: How a builder runs one Grid Job and hands off a proof table. Use when a Job is assigned to you.
topics: delivering
---

Read the Job with `grid job show <task> <job>`. It is your whole brief. Owner decisions that apply: no Grid-made guards, caps or limits (Claude Code built-ins only); everything is a Job and the Job is the brief; carry on by yourself until a person point; no plan mode.

Steps:

1. Note the files you may write and the out-of-scope list. Touch nothing else. If the Job needs more, say so in the result; do not widen the scope.
2. Read each code fact again in this session before you rely on it, with a `path:line`. Do not trust the Job's own line numbers without reading them.
3. For a move or removal, write the list of items first and work through it one by one.
4. For UI, every field comes from the data source the Job names (served op answer, `frame/model.mjs`, decision doc). A field the read lacks is NOT DONE, not a guess. Check at widths 390, 1440 and 3840.
5. `budget.turns_estimate` is the planner's estimate, not a limit; finish the Job. If you end before every row holds (Claude Code's own limit, a blocker), post the result with the rows left as NOT DONE (`not met` in the result body).
6. A new test fails on the base and passes on your change; paste the failing lines. Never edit an existing test to make it pass.
7. Run `grid job verify <job> --task <task>` and the Job's verify command.
8. Post the result with `grid job result --task <task> --job <job> --body <json>`. If it exits 3 the lease or record was rejected: do not retry.

The result is a proof table: one line per acceptance row, each either

- `HOLDS` with the command and its exit code, or a `path:line`; or
- `NOT DONE` with why.

Then what changed, in a few lines, each verify command with its exit code, and what is NOT VERIFIED. A row you could not prove is NOT DONE; never write HOLDS for a row you did not check. Add evidence with `grid proof add <item> <file> --task <task>`. If blocked, post the result with the blocker named and follow the Job's `on_blocked`.

Post a result before you end, also when blocked or when nothing changed. A valid body looks like this; keep every field, with `[]` where you have nothing to list:

```json
{
  "outcome": "Added the retry to the sync client; both acceptance rows hold.",
  "changed": ["internal/sync/client.go", "internal/sync/client_test.go"],
  "evidence": ["go test ./internal/sync/ exit 0", "internal/sync/client.go:42"],
  "acceptance": [
    {"id": "A1", "state": "met"},
    {"id": "A2", "state": "dropped", "decision": "the Job's out-of-scope list rules it out"}
  ],
  "verified": null,
  "must_fix": [],
  "found": [],
  "proposed_jobs": [],
  "decisions_needed": [],
  "knowledge": []
}
```

`state` is `met`, `not met` or `dropped`; a `dropped` row needs a `decision`. Row ids are `A1`, `A2` and so on, as the Job numbers them. Put what is NOT VERIFIED in `outcome` or `evidence`.

If blocked, post the result with the blocker named in `outcome` and the rows you could not finish as `not met`; follow the Job's `on_blocked`.

To tell the task something during the Job (a question, a finding, a blocker), post it to the task's Feed with the feed_post tool; feed_types lists the record types. The result still goes with `grid job result`.

Done: every row has a line, the verify exit codes are pasted, and the result is posted.

## Bash

- Send one command per Bash call: no `;`, no `&&`, no pipe, no `$(...)` and no `echo $?`. The exit code is in the tool result.
- `grid` and `git` are pre-approved. The Job's verify command is still run, as the Job writes it, in a call of its own (step 7).
- Put no `VAR=value` in front of a command; use the command's own flags.
- Use Read, Glob and Grep instead of ls, cat, find and grep. Use Write and Edit instead of sed, cp and heredocs.
- Never set `dangerouslyDisableSandbox`.

## Brain

- Read the notes the Job's Context names: `grid brain show <note> --task <task>`.
- To leave a durable fact for later tasks, propose it: `grid brain propose --task <task> --name <id> --topic <topic> --description <when> --summary <what> --body <text> --source job:<task>/<job>`.
- If a note turns out wrong, do not edit it. Propose the fixed fact the same way, with a `supersedes` link to the wrong note: `--link '{"to":"<wrong-note>","rel":"supersedes","summary":"<what it said>","why":"<what was wrong>"}'`.
