---
type: job
version: 1
who: orchestrator
required: [title]
fields:
  - title: one line saying what the Job is
  - brief: path of the brief file in the task folder, or the instructions in a few lines
  - role: the Worker role that should run it (scout, researcher, builder, runner, reviewer, designer, checker, operator)
  - repo: the repo the Job works in, when it works in one
  - after: ids of Jobs that must be done first
  - kind: `wait` for a Job that watches a pipeline and runs no model; leave out for a Job a Worker does
  - check: for a wait Job, `{command: [argv...]}` or `{url}`, with `interval_s` (15), `timeout_s` (3600) and `probe_timeout_s` (30); a command exits 0 green, 1 red, 3 asks for a manual approval, anything else is pending; never run through a shell
  - item: the number of the plan item the Job serves, a whole number; the Orchestrator sets it from the plan
  - priority: high, normal (the default) or low; a later control post's priority wins
  - rerole: set by the Orchestrator when it posts a planned Job under another role than the plan gave, and why (a checker Job whose brief runs a command goes to a runner)
  - host: the Host the Job must run on, from the plan step's `host:` line: `local`, this machine's name, an SSH Host added for the project, or a Host on the Feed; the Job runs there or waits, never elsewhere
---
One piece of work that one Worker picks up. The envelope `job` is its id (J-1, J-2, ...); a Job post without one gets the next number.

Only a Job posted by the Orchestrator counts: role `orchestrator`, and a Worker id that is missing, or is not a Job attempt's Worker id (the Orchestrator's own session id, say). A Job from anyone else is kept, shown and ignored with a reason, and never goes ready; the same holds for a `handover`.

Orchestrator: posts it, then moves it with control posts: ready when nothing blocks it, assigned to a Worker and Host, and on from there. A Job nobody moved stays posted.

Wait Job: the Orchestrator loop reads the check itself (feed/wait.mjs), with no Worker and no model call. Green posts a result and the Job goes done; red posts a result with the failure and the Job goes rejected; past `timeout_s` the lease runs out and the Job is stale, as for any Job. A check that asks for a manual approval makes the Orchestrator post one question to the Owner (`who: owner`) and go on watching; it never approves. The question is closed when the Job ends, goes stale or is sent again; a new watch that needs approval asks again.

Limits of a wait Job, to know before you post one:
- Posts are unsigned in this version. Whoever can write a Job post that the fold counts (a line in the Feed that claims to be the Orchestrator, from a synced remote host too) makes the Orchestrator host run its command, with the loop's environment. Only the job post the fold counted is read, never a later line that reuses its id.
- A URL check reaches any http or https address the host can, internal ones too, and keeps the last 400 characters of the answer as evidence in the result.
- A command and the processes it starts run in their own process group, which is killed when the probe ends or at `probe_timeout_s`; no probe runs past the lease.
- A plan item of kind `wait` with a `check` is posted as a wait Job and checked the same way; one that fails the check is posted too and rejected with the reason.
