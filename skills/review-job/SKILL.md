---
name: review-job
description: How a reviewer checks a finished Grid Job against its acceptance rows and writes the verdict. Use when reviewing a builder's result.
topics: reviewing
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

A change outside the files the Job allows, or a failing or skipped verify, breaks the Job itself and is a must-fix. Anything else you notice is a note.
