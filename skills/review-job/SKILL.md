---
name: review-job
description: How a reviewer checks a finished Grid Job against its acceptance rows and writes the verdict. Use when reviewing a builder's result.
---

Read with `grid job show <task> <job>`. Post with `grid job result --task <task> --job <job> --body <json>`.

Check each acceptance row against files and command output. Rerun only read-only checks; the verify command's recorded totals can be trusted when they ran on the head you review.

Verdict text:

```
ACCEPTED | REJECTED
Rows: 1 met, 2 met, 3 NOT MET (path:line)
Must-fix: only a gap that breaks a row, one line each, with the row number
Notes: one line each, never blocking
```

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

An `ACCEPTED` body has `"must_fix": []`. Each `must_fix` row is `A<n>` (or `write`, `verify`, `classify`) with the `path:line` in `at`.

A change outside the files the Job allows, or a failing or skipped verify, breaks the Job itself and is a must-fix. Anything else you notice is a note.
