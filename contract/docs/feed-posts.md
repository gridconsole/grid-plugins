# Feed post types

A post type is one kind of line in a task's Feed: a name, a pinned version (`type_v`), a class (`state` changes what the fold decides, `chatter` is read but changes no Job or question) and the roles that may write it.
Posts are written with `grid feed post` and, over MCP, with the `feed_post` tool; the schema of each type, which this page follows, says what a valid record is.
A body that is only `{"ref": "bodies/<ulid>.json"}` is the same body stored outside the line; read it as that file's content.

## ack
Version 1. Class state. Written by worker, agent, cli.

An ack is the first post of a worker's tenure on a Job. It names the control that assigned the Job in `after`, carries the lease, fence, `job` and `attempt` of that grant, and may name the `session` and `model` that took it. The fold lets a tenure start only with an ack that names its assign exactly once; the Job then moves from assigned to running, and any other side post of that tenure before the ack is ignored.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "ack",
  "type_v": 1,
  "author": {
    "id": "worker",
    "role": "worker"
  },
  "tier": "agent",
  "worker": "worker@01K9J000000000000000000002",
  "lease": "J-1",
  "gen": 0,
  "fence": 2,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [
    "01K9J000000000000000000001"
  ],
  "job": "J-1",
  "attempt": 1,
  "wake": null,
  "body": {}
}
```

Read: contract/schemas/ack.json:85; internal/contract/fold.go:211-216; contract/README.md:157.

## answer
Version 1. Class state. Written by owner, member, lead, agent, worker, reviewer, curator, host, cli, guest, external.

An answer replies to a question: `re` names the question post and the body holds the `answer` text. It does not close the question by itself; a control that selects it does (see control). A guest's answer counts only when the question is addressed to that guest.

An agent may answer for a person with `for` (the person), `by` (its own author id, which grid mcp writes from the session) and `grant` (a link to the person's grant note, `grid/people/<person>/grants/<name>`). The answer counts as the person's only when that note names the agent by id or role, covers `answer`, the task and the person Job's kind (`approve` only when named), and its `until` is after the answer's `at` (contract.PostedAs; run-jobs-launcher.md Q8). Otherwise it counts as the agent's own. It is never refused for this, and its stored author stays the agent's.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "answer",
  "type_v": 1,
  "author": {
    "id": "owner",
    "role": "owner"
  },
  "tier": "person-team",
  "worker": "lead@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": "01K9J000000000000000000001",
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "answer": "Use the first option."
  }
}
```

Read: contract/schemas/answer.json:95; internal/contract/fold.go:166, :254-259, :293-296; contract/README.md:163.

## approval
Version 1. Class state. Written by owner, member, lead, agent, cli.

An approval post answers a `blocked` post. `re` names the blocked post, `job` and `attempt` name the attempt, and the body holds the exact `rules` and the `decision`. An assignment's `grant` names it (run-jobs-launcher.md Addendum 7 A2 and C point 11, which write it as `approval {re, job, attempt, rules, decision}`; `re`, `job` and `attempt` are the envelope's). The fold has no case for it yet: it counts as a post and changes no Job.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "approval",
  "type_v": 1,
  "author": {
    "id": "owner",
    "role": "owner"
  },
  "tier": "person-team",
  "worker": "owner@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": "01K9J0000000000000000000B1",
  "after": [],
  "job": "J-7",
  "attempt": 3,
  "wake": null,
  "body": {
    "rules": [
      "Bash(make deploy-prod)"
    ],
    "decision": "approve"
  }
}
```

Read: contract/schemas/approval.json:82; internal/contract/fold.go:101-103 (no approval case); contract/README.md:177.

## blocked
Version 1. Class state. Written by worker, agent, reviewer.

A blocked post says that the agent's attempt needs a permission it does not have: the `rule` it needs, the exact `command` and `why`. The agent ends after it (run-jobs-launcher.md Addendum 7 A5 and C point 13, `blocked {need: permission, rule, command, why}`). The fold has no case for it yet: it counts as a post and changes no Job.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "blocked",
  "type_v": 1,
  "author": {
    "id": "worker",
    "role": "worker"
  },
  "tier": "agent",
  "worker": "worker@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": "J-7",
  "attempt": 3,
  "wake": null,
  "body": {
    "need": "permission",
    "rule": "Bash(make deploy-prod)",
    "command": "make deploy-prod",
    "why": "The Job deploys the site"
  }
}
```

Read: contract/schemas/blocked.json:80; internal/contract/fold.go:101-103 (no blocked case); contract/README.md:178.

## cancel
Version 1. Class state. Written by owner, member, lead, cli.

A cancel post asks to stop the attempt that `job` and `attempt` name, with an optional `why`. A person writes it as a request, and the orchestrator writes it to drop an attempt it no longer wants (run-jobs-launcher.md Q2 and Q4). The fold has no case for it yet: it counts as a post and changes no Job.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "cancel",
  "type_v": 1,
  "author": {
    "id": "owner",
    "role": "owner"
  },
  "tier": "person-team",
  "worker": "owner@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": "J-7",
  "attempt": 3,
  "wake": null,
  "body": {}
}
```

Read: contract/schemas/cancel.json:62; internal/contract/fold.go:101-103 (no cancel case); contract/README.md:179.

## checkpoint
Version 1. Class state. Written by worker, agent, cli.

A checkpoint is a worker's progress mark inside a Job tenure: the `job`, a short `step` and the `commit` reached. The fold keeps the latest one on the Job, so a later attempt can continue from it. Like every Job-side post it counts only inside a tenure that began with an ack.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "checkpoint",
  "type_v": 1,
  "author": {
    "id": "worker",
    "role": "worker"
  },
  "tier": "agent",
  "worker": "worker@01K9J000000000000000000002",
  "lease": "J-1",
  "gen": 0,
  "fence": 2,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": "J-1",
  "attempt": 1,
  "wake": null,
  "body": {
    "job": "J-1",
    "step": "Checks complete",
    "commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  }
}
```

A checkpoint can instead mark one acceptance row of the Job: the body is the `job` and an `acceptance` object with the row `id`, its `state` (`met`, `not met`, `not-met` or `pending`) and an optional `note`, for example `{"job": "J-1", "acceptance": {"id": "A1", "state": "met", "note": "go test ./... exit 0"}}`. A worker writes it with `grid job post --type checkpoint` when it proves a row. The fold keeps the latest state and note of each row in the Job's `acceptance` list (record order; a row nothing marked reads `pending`, a row the record drops reads `dropped`, `not-met` reads `not met`), and a result's `acceptance` states overwrite the marks. A row mark leaves the step checkpoint as it was; it still counts as progress for the crash-loop count, as a step checkpoint does. A mark of a row the Job record does not have is ignored with its reason.

Read: contract/schemas/checkpoint.json:92; internal/contract/fold.go:362-365; contract/README.md:160.

## control
Version 1. Class state. Written by lead, owner, member, cli, guest.

A control is a decision about a Job or a question, and the only post that moves a Job. Its body is one of several shapes: `assign` grants a Job attempt to a worker with a fence and an `until` time; `target` with `status`, `reason` and `close_reason` changes a Job's status (cancelling or rejecting needs a `close_reason`); `order` changes the queue position of a Job that has not started; `target` with `answer` selects the answer that closes a question. Controls apply in (gen, fence, seq) order, never by timestamp.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "control",
  "type_v": 1,
  "author": {
    "id": "lead",
    "role": "lead"
  },
  "tier": "person-team",
  "worker": "lead@01K9J000000000000000000002",
  "lease": "task",
  "gen": 0,
  "fence": 1,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "assign": {
      "job": "J-1",
      "attempt": 1,
      "to": "worker@01K9J000000000000000000002",
      "fence": 2,
      "until": "2026-11-08T10:10:00.000Z"
    }
  }
}
```

Read: contract/schemas/control.json:484; internal/contract/fold.go:11-20, :33-34, :158-249; contract/README.md:156.

## deploy-failed
Version 1. Class state. Written by lead, owner, member, cli.

A deploy-failed post records that a delivery to an environment (`env`) stopped at a `stage` for a stated `reason`. It puts the failure on the task's Feed for the next reader. The fold has no case for it: it counts as a post and changes no Job.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "deploy-failed",
  "type_v": 1,
  "author": {
    "id": "lead",
    "role": "lead"
  },
  "tier": "person-team",
  "worker": "lead@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "env": "example",
    "stage": "test",
    "reason": "Example check failed"
  }
}
```

Read: contract/schemas/deploy-failed.json:87; internal/contract/fold.go:292-319 (no deploy-failed case); contract/README.md:174.

## exit
Version 1. Class state. Written by cli, host, worker.

An exit post is the launcher's record that an attempt ended: the `reason`, the `code` (an exit code, `no_result`, or the usage reset in unix seconds), the `cost` line, the pushed `sha` and the agent's `permission_denials`. On `secret_found`, `secret` names the commit, file, line and pattern, never the secret text; a command Job's exit also carries its `log` and `link` (run-jobs-launcher.md Q3, Addendum 7 B and C points 15-16). The YAML in that note writes `job` and `attempt` beside the body; on the Feed they are the envelope's. The fold has no case for it yet: it counts as a post and changes no Job.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "exit",
  "type_v": 1,
  "author": {
    "id": "cli",
    "role": "cli"
  },
  "tier": "agent",
  "worker": "cli@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": "J-7",
  "attempt": 3,
  "wake": null,
  "body": {
    "reason": "done",
    "code": 0,
    "cost": {
      "wall_s": 2140,
      "cpu_s": 3810,
      "cpu_avg_cores": 1.8,
      "rss_peak_mb": 2900,
      "io_read_mb": 410,
      "io_write_mb": 95,
      "tokens_in": 81234,
      "tokens_out": 21011,
      "cache_read": 1904400,
      "usd": 6.12,
      "model": "opus",
      "account": "claude-team-2"
    },
    "sha": "9ab01cd"
  }
}
```

Read: contract/schemas/exit.json:197; internal/contract/fold.go:101-103 (no exit case); contract/README.md:180.

## found
Version 1. Class chatter. Written by reviewer, lead, worker, agent, cli.

A found post is a finding a reviewer raises on a Job (a Job agent with role worker or agent may also author one): the `job`, the post it is about (`re`), where (`at`, as file:line), the `text`, a `kind` such as bug, and an optional `class`. It changes no status. The one exception is a finding of kind `security`: the fold marks that Job as needing the lead and lists the post among the wakes.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "found",
  "type_v": 1,
  "author": {
    "id": "reviewer",
    "role": "reviewer"
  },
  "tier": "agent",
  "worker": "lead@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "job": "J-1",
    "re": "01K9J000000000000000000001",
    "at": "main.go:10",
    "text": "Check the example boundary.",
    "kind": "bug",
    "class": null
  }
}
```

Read: contract/schemas/found.json:104; internal/contract/fold.go:261-267, :311-316; contract/README.md:169.

## handover
Version 1. Class state. Written by lead, owner, member, cli.

A handover announces that the writer is passing the task on. Its body is empty. The fold reads it only as an announcement: neither it nor a transfer changes who holds authority, which comes from the controls and their fences.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "handover",
  "type_v": 1,
  "author": {
    "id": "lead",
    "role": "lead"
  },
  "tier": "person-team",
  "worker": "lead@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {}
}
```

Read: contract/schemas/handover.json:84; internal/contract/fold.go:153-155; contract/README.md:164.

## head
Version 1. Class state. Written by lead, owner, member, cli.

A head post names the `sha` the task's work has reached. Only a head written under the task lease is ordered: the fold keeps the newest by (gen, fence, seq) as the task's head, and heads without that lease stay visible but do not become it.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "head",
  "type_v": 1,
  "author": {
    "id": "lead",
    "role": "lead"
  },
  "tier": "person-team",
  "worker": "lead@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "sha": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  }
}
```

Read: contract/schemas/head.json:76; internal/contract/fold.go:301-306, :321-323; contract/README.md:171.

## host
Version 1. Class state. Written by host, cli, lead.

A host post says whether a machine is `online`, whether it takes work (`grant`), and optionally which `repos` it has, its `slots` and the `usage_left`. The fold keeps the newest host post of each `worker` and lists it in the state's hosts, so a reader sees the current word of each machine.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "host",
  "type_v": 1,
  "author": {
    "id": "host",
    "role": "host"
  },
  "tier": "agent",
  "worker": "lead@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "online": true,
    "grant": true
  }
}
```

Read: contract/schemas/host.json:93; internal/contract/fold.go:307-310, :324-326; contract/README.md:172.

## integrated
Version 1. Class state. Written by cli, host, worker.

An integrated post records that `grid task integrate` squashed the accepted attempt onto the task head; `head_sha` is the new head (run-jobs-launcher.md C point 7, `integrated {head_sha}`). The fold has no case for it yet: it counts as a post and changes no Job.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "integrated",
  "type_v": 1,
  "author": {
    "id": "cli",
    "role": "cli"
  },
  "tier": "agent",
  "worker": "cli@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": "J-7",
  "attempt": 3,
  "wake": null,
  "body": {
    "head_sha": "9ab01cd"
  }
}
```

Read: contract/schemas/integrated.json:63; internal/contract/fold.go:101-103 (no integrated case); contract/README.md:181.

## job
Version 1. Class state. Written by lead, owner, member, cli.

A job post is the brief of one Job: its `id` (such as J-1), `title`, `role`, what to read and write, the acceptance rows, the verify command, the budget and what the worker posts back. The fold creates the Job from it as posted; a second job post with the same id is ignored, so a revision needs a new Job.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "job",
  "type_v": 1,
  "author": {
    "id": "lead",
    "role": "lead"
  },
  "tier": "person-team",
  "worker": "lead@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": "J-1",
  "attempt": null,
  "wake": null,
  "body": {
    "id": "J-1",
    "item": 1,
    "title": "Build example feature",
    "role": "builder",
    "skills": [],
    "review": "L1",
    "risk": [],
    "max_rounds": 2,
    "intent": {
      "owner": "intent.md#L1",
      "item": "plan.md#step-1",
      "goal": "plan.md#verify"
    },
    "links": [],
    "keep": [],
    "decisions": [],
    "read": [
      "repo:main.go:1-10"
    ],
    "write": [
      "main.go"
    ],
    "out_of_scope": [],
    "acceptance": [
      {
        "id": "A1",
        "text": "Example feature passes its checks",
        "traces": "item"
      }
    ],
    "verify": {
      "command": "go test ./...",
      "min_tests": 1
    },
    "budget": {
      "turns_estimate": 20,
      "reads_before_first_edit": 15
    },
    "on_blocked": "ask-and-continue",
    "after": [],
    "attempt": 1,
    "continues": null,
    "discovered_from": null,
    "close_reason": null,
    "checkpoint": null,
    "review_rounds": 0,
    "level_raised": null,
    "worktree": "example-worktree",
    "posts_back": [
      "result",
      "question",
      "knowledge",
      "checkpoint"
    ]
  }
}
```

Read: contract/schemas/job.json:58; internal/contract/fold.go:132-148; contract/README.md:154.

## knowledge
Version 1. Class chatter. Written by owner, member, lead, agent, worker, reviewer, curator, host, cli, guest, external.

A knowledge post holds one fact worth keeping: a `title` and the `text`. It is posted when someone learns something the next reader should not have to find again. It changes no Job or question; the fold does not read it.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "knowledge",
  "type_v": 1,
  "author": {
    "id": "owner",
    "role": "owner"
  },
  "tier": "person-team",
  "worker": "lead@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "title": "Example fact",
    "text": "An example observation."
  }
}
```

Read: contract/schemas/knowledge.json:141; internal/contract/fold.go:292-319 (no knowledge case); contract/README.md:168.

## landed
Version 1. Class state. Written by cli, host, worker.

A landed post records that a land Job put the change on main; `main_sha` is main after the land. A Job record's `after` edge of kind `landed` waits for it (run-jobs-launcher.md Q2 and Q9). The fold has no case for it yet: it counts as a post and changes no Job.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "landed",
  "type_v": 1,
  "author": {
    "id": "cli",
    "role": "cli"
  },
  "tier": "agent",
  "worker": "cli@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": "J-7",
  "attempt": 1,
  "wake": null,
  "body": {
    "main_sha": "9ab01cd"
  }
}
```

Read: contract/schemas/landed.json:63; internal/contract/fold.go:101-103 (no landed case); contract/README.md:182.

## loop-detected
Version 1. Class state. Written by cli, worker, lead.

A loop-detected post reports that a repeated pattern was seen: the `signal` that fired, the `threshold` it crossed and the `calls` that repeated (or an `error`). It is posted when the pattern is noticed, and the fold lists it among the state's wakes so the lead is woken to look at it.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "loop-detected",
  "type_v": 1,
  "author": {
    "id": "cli",
    "role": "cli"
  },
  "tier": "agent",
  "worker": "lead@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "signal": "repeated-call",
    "threshold": 5,
    "calls": [
      "example --check"
    ]
  }
}
```

Read: contract/schemas/loop-detected.json:79; internal/contract/fold.go:317-318; contract/README.md:166.

## post
Version 1. Class chatter. Written by owner, member, lead, agent, worker, reviewer, curator, host, cli, guest, external.

A post is a free message about something: `ref` names what it is about (a Job, `task`, and so on), `kind` is message, pickup, ask-pickup, ship, close or reopen, and `text` is required for a message. The fold groups posts by their `ref` so a reader can list what was said about one thing; it changes no state.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "post",
  "type_v": 1,
  "author": {
    "id": "owner",
    "role": "owner"
  },
  "tier": "person-team",
  "worker": "lead@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "ref": "task",
    "kind": "message",
    "text": "Example feedback."
  }
}
```

Read: contract/schemas/post.json:46-54, :141; internal/contract/fold.go:297-300; contract/README.md:167.

## question
Version 1. Class state. Written by lead, owner, member, worker, agent, cli.

A question asks a person (`who`) for a decision: the `question` text, and optionally `options`, a `recommended` one, a `why`, a `gate` and whether it is `blocking`. It stays open and appears in needs-you until a control selects an answer to it; it closes by itself when the Job it belongs to is cancelled.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "question",
  "type_v": 1,
  "author": {
    "id": "lead",
    "role": "lead"
  },
  "tier": "person-team",
  "worker": "lead@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "question": "Which option should run?",
    "who": "owner"
  }
}
```

Read: contract/schemas/question.json:36-65, :193; internal/contract/fold.go:149-152, :164-171, :272-285; contract/README.md:161.

## result
Version 2. Class state. Written by worker, agent, cli.

A result is the worker's report that an attempt is done: the `outcome`, what `changed`, the `evidence`, the state of each acceptance row, and lists of `must_fix`, `found`, `proposed_jobs`, `decisions_needed` and `knowledge`. The fold records it on the Job; proposed jobs and decisions needed are copied to the Job, and any decision needed marks the Job as needing the lead.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "result",
  "type_v": 2,
  "author": {
    "id": "worker",
    "role": "worker"
  },
  "tier": "agent",
  "worker": "worker@01K9J000000000000000000002",
  "lease": "J-1",
  "gen": 0,
  "fence": 2,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": "J-1",
  "attempt": 1,
  "wake": null,
  "body": {
    "outcome": "Example completed",
    "changed": [
      "main.go"
    ],
    "evidence": [
      "go test ./... passed"
    ],
    "acceptance": [
      {
        "id": "A1",
        "state": "met"
      }
    ],
    "verified": null,
    "must_fix": [],
    "found": [],
    "proposed_jobs": [],
    "decisions_needed": [],
    "knowledge": []
  }
}
```

Read: contract/schemas/result.json:303; internal/contract/fold.go:350-361; contract/README.md:159.

## started
Version 1. Class state. Written by cli, host, worker.

A started post is the launcher's claim that it started the attempt that `job` and `attempt` name. `base_sha` is the commit the worktree starts at, `allow_dropped` lists the `allow` rules that no grant covered, and `firing` is the scheduled minute of a recurring Job (run-jobs-launcher.md Decision note point 3, Q2, Addendum 7 A2 and B). The fold has no case for it yet: it counts as a post and changes no Job.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "started",
  "type_v": 1,
  "author": {
    "id": "cli",
    "role": "cli"
  },
  "tier": "agent",
  "worker": "cli@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": "J-7",
  "attempt": 3,
  "wake": null,
  "body": {
    "base_sha": "3f2c9e1"
  }
}
```

Read: contract/schemas/started.json:75; internal/contract/fold.go:101-103 (no started case); contract/README.md:183.

## submit
Version 1. Class state. Written by worker, agent, cli.

A submit post hands in the work of a Job attempt: the body holds the `sha` of the commit to review. It is posted by the worker once the work is committed, and the fold lists it among the Job's submits. Like the other Job-side posts it counts only inside a tenure that began with an ack.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "submit",
  "type_v": 1,
  "author": {
    "id": "worker",
    "role": "worker"
  },
  "tier": "agent",
  "worker": "worker@01K9J000000000000000000002",
  "lease": "J-1",
  "gen": 0,
  "fence": 2,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": "J-1",
  "attempt": 1,
  "wake": null,
  "body": {
    "sha": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  }
}
```

Read: contract/schemas/submit.json:94; internal/contract/fold.go:348-349; contract/README.md:158.

## task-move
Version 1. Class state. Written by owner, member, lead, cli.

A task-move post moves the task's home from the machine `from` to the machine `to`. `to` is required; `from` is null only on the post that migration writes (brain-board.md decision point 4 and migration step 5; run-jobs-launcher.md Q4 "Task move"). The fold has no case for it yet: it counts as a post and changes no Job.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "task-move",
  "type_v": 1,
  "author": {
    "id": "owner",
    "role": "owner"
  },
  "tier": "person-team",
  "worker": "owner@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "from": "01K9J000000000000000000002",
    "to": "01K9J000000000000000000003"
  }
}
```

Read: contract/schemas/task-move.json:76; internal/contract/fold.go:101-103 (no task-move case); contract/README.md:184.

## transfer
Version 1. Class state. Written by lead, owner, member, cli.

A transfer names the worker (`to`) that the task is handed to. The fold treats it as an announcement, like a handover: it does not change authority, which stays with the controls and their fences.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "transfer",
  "type_v": 1,
  "author": {
    "id": "lead",
    "role": "lead"
  },
  "tier": "person-team",
  "worker": "lead@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "to": "lead@01K9J000000000000000000002"
  }
}
```

Read: contract/schemas/transfer.json:68; internal/contract/fold.go:153-155; contract/README.md:173.

## usage-notice
Version 1. Class chatter. Written by cli.

A usage-notice tells the task that its spend crossed a `threshold` with the `dollars` used so far. Only the cli writes it. It is chatter: the fold does not read it and no Job changes.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "usage-notice",
  "type_v": 1,
  "author": {
    "id": "cli",
    "role": "cli"
  },
  "tier": "agent",
  "worker": "lead@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "task": "01K9J000000000000000000001",
    "threshold": 70,
    "dollars": 12.5
  }
}
```

Read: contract/schemas/usage-notice.json:96; internal/contract/fold.go:292-319 (no usage-notice case); contract/README.md:170.

## waited
Version 1. Class state. Written by cli, host, worker.

A waited post is the outcome of `grid wait`: `status` is `green`, `red` or `timeout`, with the pipeline's `link` and the `log` (run-jobs-launcher.md C point 16, `waited {status: green|red|timeout, link, log}`). The fold has no case for it yet: it counts as a post and changes no Job.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "waited",
  "type_v": 1,
  "author": {
    "id": "cli",
    "role": "cli"
  },
  "tier": "agent",
  "worker": "cli@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": "J-9",
  "attempt": 1,
  "wake": null,
  "body": {
    "status": "green",
    "link": "https://ci.example.com/runs/42",
    "log": "logs/J-9-a1.log"
  }
}
```

Read: contract/schemas/waited.json:73; internal/contract/fold.go:101-103 (no waited case); contract/README.md:185.

## waiting
Version 1. Class state. Written by cli, host, worker.

A waiting post says that an assignment's `start` condition has not held for its `wait_max`. `condition` is the start condition, and `unmet` names each atom that does not hold, with its `measured` value and the value it wants (run-jobs-launcher.md Q2, "Timeout"). The fold has no case for it yet: it counts as a post and changes no Job.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "waiting",
  "type_v": 1,
  "author": {
    "id": "cli",
    "role": "cli"
  },
  "tier": "agent",
  "worker": "cli@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": "J-7",
  "attempt": 3,
  "wake": null,
  "body": {
    "condition": {
      "all": [
        {
          "cores_free": 4
        },
        {
          "no_test_run": true
        }
      ],
      "wait_max": "30m"
    },
    "unmet": [
      {
        "atom": "cores_free",
        "measured": 1.5,
        "want": 4
      }
    ]
  }
}
```

Read: contract/schemas/waiting.json:111; internal/contract/fold.go:101-103 (no waiting case); contract/README.md:186.

## wake
Version 1. Class state. Written by cli, host.

A wake post records that the task's home launcher woke the orchestrator: `events` are the ids of the event posts it handed over, and `pid` is the session's process (run-jobs-launcher.md Q4). The fold has no case for it yet: it counts as a post and changes no Job.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "wake",
  "type_v": 1,
  "author": {
    "id": "cli",
    "role": "cli"
  },
  "tier": "agent",
  "worker": "cli@01K9J000000000000000000002",
  "lease": null,
  "gen": 0,
  "fence": 0,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": null,
  "attempt": null,
  "wake": null,
  "body": {
    "events": [
      "01K9J0000000000000000000E1"
    ]
  }
}
```

Read: contract/schemas/wake.json:71; internal/contract/fold.go:101-103 (no wake case); contract/README.md:187.

## worker-exit
Version 1. Class state. Written by worker, agent, cli.

A worker-exit post says an attempt ended and why: `reason` is done, crash, usage_limit, cancelled or loop, with an optional `code`. A missing reason reads as crash. A cancelled exit with code `worker_stopped` means the worker stopped or restarted, not that anyone cancelled the Job: the Job goes back to Ready and is offered again, and no re-plan Job is posted; every other cancelled exit ends the Job. A usage_limit exit lets the same attempt be granted again with a higher fence; its `code` is the reset time in unix seconds when the worker knows it, and the Job is offered again once that time has passed or an account has room. A loop exit counts a review round and marks the Job as needing the lead.

```json
{
  "id": "01K9J000000000000000000001",
  "task": "01K9J000000000000000000001",
  "type": "worker-exit",
  "type_v": 1,
  "author": {
    "id": "worker",
    "role": "worker"
  },
  "tier": "agent",
  "worker": "worker@01K9J000000000000000000002",
  "lease": "J-1",
  "gen": 0,
  "fence": 2,
  "seq": 1,
  "at": "2026-11-08T10:00:00.000Z",
  "re": null,
  "after": [],
  "job": "J-1",
  "attempt": 1,
  "wake": null,
  "body": {}
}
```

Read: contract/schemas/worker-exit.json:33-41, :94; internal/contract/fold.go:187, :366-385; contract/README.md:165.
