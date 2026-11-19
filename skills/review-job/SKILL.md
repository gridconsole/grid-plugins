---
name: review-job
description: How a reviewer judges a finished Grid Job against the builder's proof table and posts the verdict. Use when reviewing a builder's result.
topics: reviewing
---

Read the Job with `grid job show <task> <job>` and the builder's result. Owner decisions that apply: no Grid-made guards, caps or limits; everything is a Job and the Job is the brief; the reviewer never edits the work; no plan mode.

Steps:

1. Open the proof table first. Each acceptance row of the Job must have one line, `HOLDS`, `WAITING on <kind> <ref>` or `NOT DONE`.
2. A `NOT DONE` row, or a row with no line, is a must-fix on that row. The exception is a row marked `NOT DONE (environment: <cause>)`: do not reject it; check it yourself by the environment rule below.
3. A `WAITING` row is not a `NOT DONE` row. Check that its wait is real: `waits_on` names a `kind` (`job`, `landed`, `question` or `permission`) and a `ref` that exists, the row truly cannot hold without it, and the thing it names has not already landed (a `job` ref already accepted, a `landed` ref already landed, a `question` already answered, a `permission` already granted). A wait that fails any of these hides a defect: reject the row as `NOT DONE`, with a must-fix on it.
4. When every row is `HOLDS`, `dropped` with its decision, or a real `WAITING`, at least one is `WAITING`, and nothing else is must-fix, the verdict is `HELD`, not `REJECTED`, with `"must_fix": []`. A hold gets no rework Job and no "Decide how" person Job; the Job carries on by itself once the wait lands.
5. Otherwise check each `HOLDS` row against the files and command output named in its line. Do not rerun a check the builder pasted with its exit code on the same head; rerun only read-only checks that were not shown.
6. A `HOLDS` row with no command and exit code or `path:line` is not proof: reject it as a row with no proof.
7. A change outside the Job's write list, a failing or skipped verify, or an existing test edited to pass breaks the Job: must-fix, also when the other open rows are `WAITING`. A verify that failed only for an environment cause is not that: check it by the environment rule below.
8. Anything else is a one-line note. A note never blocks and nobody starts work on it unless a person asks.

Environment rule: you judge whether the work is done, not the environment.

- Sort every failing or unrunnable check, yours or the builder's, into a code cause or an environment cause. An environment cause is a port in use, a shared service down, a missing dependency, a folder or credential missing in the worktree, or a sandbox refusal to bind or write. Name the error lines as the proof of the cause.
- For an environment cause, fix it or work around it without touching the Job's files: a free port, your own service instance, a run in a checkout that has the dependencies. Rerun the check and judge the row on that output.
- A row you still cannot run after trying a workaround is judged from the code and the builder's evidence. The environment cause is a note, plus a fix Job in `proposed_jobs` when it will recur. It is never a must-fix, never a rework and never a question to a person.
- A real wait on a Job, a landing, a question or a permission stays `WAITING` (step 3); an environment failure is never turned into a hold.

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

Post with `grid job result --task <task> --job <review-job> --body <json>`. The first line is exactly `ACCEPTED`, `REJECTED` or `HELD`:

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

A hold body looks like this. The second line lists the waits; the waiting row keeps its state `waiting` with its `waits_on`:

```json
{
  "outcome": "HELD\nWaits: A3 on landed J-6\nRows: A1 met, A2 met, A3 WAITING (J-6 not landed)\nNotes: none",
  "changed": [],
  "evidence": ["go test ./internal/sync/ exit 0", "internal/sync/client.go:42"],
  "acceptance": [
    {"id": "A1", "state": "met"},
    {"id": "A2", "state": "met"},
    {"id": "A3", "state": "waiting", "waits_on": {"kind": "landed", "ref": "J-6"}}
  ],
  "verified": null,
  "must_fix": [],
  "found": [],
  "proposed_jobs": [],
  "decisions_needed": [],
  "knowledge": []
}
```

`found` holds the record ids that the `grid job post` calls above printed, one id per finding; it is `[]` when you posted none.

An `ACCEPTED` or `HELD` body has `"must_fix": []`. Each `must_fix` row is `A<n>` (or `write`, `verify`, `classify`) with the `path:line` in `at`.

Done: the verdict is posted and every must-fix names a row.
