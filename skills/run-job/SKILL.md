---
name: run-job
description: How a builder reads one Grid Job, stays in its scope, and posts a result with evidence. Use when running an assigned Job.
---

Read with `grid job show <task> <job>`. Post with `grid job result --task <task> --job <job> --body <json>`.

Before work: note the files you may write and the out-of-scope list. After work, the result lists:

- what changed, in a few lines;
- each acceptance row with its evidence (a command and its exit code, or `path:line`);
- each verify command with its exit code;
- what is NOT VERIFIED, and why.

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

If blocked, post the result with the blocker named in `outcome` and the rows you could not finish as `not met`; follow the Job's `on_blocked`. Never edit an existing test to make it pass. If `grid job result` exits 3, the lease or record was rejected: do not retry.

## Brain

- Read the notes the Job's Context names: `grid brain show <note> --task <task>`.
- To leave a durable fact for later tasks, propose it: `grid brain propose --task <task> --name <id> --topic <topic> --description <when> --summary <what> --body <text> --source job:<task>/<job>`.
- If a note turns out wrong, do not edit it. Propose the fixed fact the same way, with a `supersedes` link to the wrong note: `--link '{"to":"<wrong-note>","rel":"supersedes","summary":"<what it said>","why":"<what was wrong>"}'`.
