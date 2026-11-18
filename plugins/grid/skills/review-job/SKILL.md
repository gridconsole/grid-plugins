---
name: review-job
description: How a reviewer judges a finished Grid Job against the builder's proof table and posts the verdict. Use when reviewing a builder's result.
topics: reviewing
---

Read the Job with `grid job show <task> <job>` and the builder's result. Owner decisions that apply: no Grid-made guards, caps or limits; everything is a Job and the Job is the brief; the reviewer never edits the work; no plan mode.

Steps:

1. Open the proof table first. Each acceptance row of the Job must have one line, `HOLDS` or `NOT DONE`.
2. A `NOT DONE` row, or a row with no line, rejects the Job at once. Write the verdict without reading further; the must-fix is that row.
3. Otherwise check each `HOLDS` row against the files and command output named in its line. Do not rerun a check the builder pasted with its exit code on the same head; rerun only read-only checks that were not shown.
4. A `HOLDS` row with no command and exit code or `path:line` is not proof: reject it as a row with no proof.
5. A change outside the Job's write list, a failing or skipped verify, or an existing test edited to pass breaks the Job: must-fix.
6. Anything else is a one-line note. A note never blocks and nobody starts work on it unless a person asks.

Reading code, as the run-job skill's Helpers section says:

- To check a row in code you do not know, send `grid:scout` for the `path:line` ranges and read only those, because a map costs far less than whole files.
- Read a large file or log by line range, never whole, because whole-file reads are most of what a session reads.
- Hand a read-only check longer than a short one to `grid:runner`, which returns pass or fail and only the failing lines.

Bash:

- Send one command per Bash call: no `;`, no `&&`, no pipe, no `$(...)` and no `echo $?`. The exit code is in the tool result.
- Put no `<`, `>`, here-document or pipe on a `grid` command: such a call stays inside Claude Code's sandbox, which cannot write the task's files. Give a JSON body in single quotes, or write it with Write to a file outside the repo and pass `--body-file <file>`.
- `grid` and `git` are pre-approved. The Job's verify command is still run, as the Job writes it, in a call of its own (step 3).
- Put no `VAR=value` in front of a command; use the command's own flags.
- Use Read, Glob and Grep instead of ls, cat, find and grep.
- Never set `dangerouslyDisableSandbox`.

Post each finding before the verdict. For each security problem and each must-fix bug, make one call, in a Bash call of its own: `grid job post --task <task> --job <review-job> --type found --body <json>`. The body is `{"job": "<reviewed job>", "re": "<the builder's result record id>", "at": "<path:line>", "text": "<one line>", "kind": "security|bug|test|evolvability", "class": null}`. Pass `--job` as the review Job, because `grid job post` takes its lease from `--job`; the reviewed Job goes in `body.job`. The command prints the record id. A note stays a note: it makes no `found` record.

Post with `grid job result --task <task> --job <review-job> --body <json>`. The first line is exactly `ACCEPTED` or `REJECTED`:

```
REJECTED
Rows: 1 HOLDS, 2 NOT DONE, 3 HOLDS (path:line)
Must-fix: row number, path:line, one line each
Notes: one line each, never blocking
```

If a row cannot be checked, say it is NOT VERIFIED; never guess it held.

The verdict text goes in `outcome`. A rejecting body looks like this; keep every field, with `[]` where you have nothing to list:

```json
{
  "outcome": "REJECTED\nRows: A1 met, A2 NOT MET (internal/sync/client.go:42)\nMust-fix: A2 the retry is never reached\nNotes: the helper name is unclear",
  "changed": [],
  "evidence": ["go test ./internal/sync/ exit 1", "internal/sync/client.go:42"],
  "acceptance": [
    {"id": "A1", "state": "met"},
    {"id": "A2", "state": "not met"}
  ],
  "verified": null,
  "must_fix": [{"row": "A2", "at": "internal/sync/client.go:42"}],
  "found": [],
  "proposed_jobs": [],
  "decisions_needed": [],
  "knowledge": []
}
```

`found` holds the record ids that the `grid job post` calls above printed, one id per finding; it is `[]` when you posted none.

An `ACCEPTED` body has `"must_fix": []`. Each `must_fix` row is `A<n>` (or `write`, `verify`, `classify`) with the `path:line` in `at`.

Done: the verdict is posted and every must-fix names a row.
