---
name: run-job
description: How a builder reads one Grid Job, stays in its scope, and posts a result with evidence. Use when running an assigned Job.
topics: delivering
---

Read with `grid job show <task> <job>`. Post with `grid job result --task <task> --job <job> --body <json>`.

Before work: note the files you may write and the out-of-scope list. After work, the result lists:

- what changed, in a few lines;
- each acceptance row with its evidence (a command and its exit code, or `path:line`);
- each verify command with its exit code;
- what is NOT VERIFIED, and why.

If blocked, post the result with the blocker named; follow the Job's `on_blocked`. Never edit an existing test to make it pass. If `grid job result` exits 3, the lease or record was rejected: do not retry.
