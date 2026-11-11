---
name: review-job
description: How a reviewer judges a finished Grid Job against the builder's proof table and posts the verdict. Use when reviewing a builder's result.
---

Read the Job with `grid job show <task> <job>` and the builder's result. Owner decisions that apply: no Grid-made guards, caps or limits; everything is a Job and the Job is the brief; the reviewer never edits the work; no plan mode.

Steps:

1. Open the proof table first. Each acceptance row of the Job must have one line, `HOLDS` or `NOT DONE`.
2. A `NOT DONE` row, or a row with no line, rejects the Job at once. Write the verdict without reading further; the must-fix is that row.
3. Otherwise check each `HOLDS` row against the files and command output named in its line. Do not rerun a check the builder pasted with its exit code on the same head; rerun only read-only checks that were not shown.
4. A `HOLDS` row with no command and exit code or `path:line` is not proof: reject it as a row with no proof.
5. A change outside the Job's write list, a failing or skipped verify, or an existing test edited to pass breaks the Job: must-fix.
6. Anything else is a one-line note. A note never blocks and nobody starts work on it unless a person asks.

Post with `grid job result --task <task> --job <review-job> --body <json>`. The first line is exactly `ACCEPTED` or `REJECTED`:

```
REJECTED
Rows: 1 HOLDS, 2 NOT DONE, 3 HOLDS (path:line)
Must-fix: row number, path:line, one line each
Notes: one line each, never blocking
```

If a row cannot be checked, say it is NOT VERIFIED; never guess it held.

Done: the verdict is posted and every must-fix names a row.
