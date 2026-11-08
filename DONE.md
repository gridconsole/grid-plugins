# Lane L34

Implemented the mechanical Orchestrator in `internal/orchestrator`, with no model calls. The existing command table now routes `job next/post/ack/result/cancel/tree/blocked` and `feed tick` through the new adapters; existing Job and Feed commands retain their handlers.

## Behavior

- A checked task tenure and `internal/fslock` serialize each pass, including concurrent CLI processes using the same tenure. Assignment grants the Job lease before appending the task holder's control. Durable spawn requests are written to `spawned/<job>.json` and recovered on subsequent passes.
- `orchestrator.Load` and `orchestrator.Ready` provide the shared readiness interface. Set `Snapshot.Assignee` to the proposed worker when computing spawn demand, so host usage holds and review capacity match assignment. Dependency semantics and canonical sorting remain in `internal/jobgraph`.
- A pass handles dependencies, Feed gates, changed wait reasons, assignment, missing-ack expiry, heartbeat expiry, proven process death, exits, retries, `max_rounds`, checkpoints, proposals and judgement wakes. Failed process checks remain unknown and append no liveness events. Supervision can suppress clock expiry with `AfterWake`.
- A usage-limit exit parks its host's demand and preserves the attempt number. Confirmed capacity reoffers that attempt with a new fence. Cancelled exits do not count as crashes. Three consecutive failures without progress park with `needs_person`; the CLI files a Brain incidents proposal and asks the owner with a default. Task work products, Feed posts, checkpoints, owner answers and resolved worktree commits provide progress evidence.
- Answers with one eligible candidate are applied by an explicit task control. Multiple candidates wake the lead rather than choosing by timestamp. Person/default, timer, CI-answer and Job-state gates are supported.
- Writes use the existing Feed validation, role checks and fencing. Ack supplies its assignment in `after`. Old attempts and writes after exit or closure are refused. A duplicate wake write returns the first record's id and a nonzero exit.
- Results retain `proposed_jobs`. A proposal naming an existing acceptance row on the same planned item and role becomes a child with `discovered_from`, inherited scope and a cleared worktree field for fresh allocation by the worker. Broader proposals remain judgement events. Rendering and worktree creation remain their existing packages' responsibilities.
- Wakes are persisted, limited to 20 per task in a rolling hour, and excess events are delivered in one summary wake when capacity returns. A summary includes an `events` array. The cap never parks a Job.
- `next --wait` polls every second, including Feed, leases and heartbeat state; this also provides the required rescan within 30 seconds. `feed tick` performs the identical pass without waiting. `tree` includes attempts and discovery edges; `blocked` reports one reason per waiting Job.

## Added owning-package primitives

Main's `internal/lease` contained merge leases only. Added `lease.TaskStore` for first task grants, shared-lock reads and task-authorized Job grants, with in-place lease/counter writes and takeover snapshots. `feed.WithLease` holds an already checked tenure across a pass; `feed.Sync` shares the existing Linux/macOS durability implementation. `feed.FoldRead` folds one reader snapshot, and `feed.SaveStateLiveness` overlays holder observations for display.

`session.ProbeLocal` checks process identity conservatively on Linux/macOS. `task.FeedIdentity` delegates stable-id parsing to the existing Feed helper and reads optional owner metadata through the existing frontmatter reader. This accommodates main's legacy display-id task reader without adding another YAML parser. No protected contract, resolver, project or lock implementation was changed.

## Running

The caller must already hold the task lease. A supervising caller can bootstrap it with `lease.TaskStore.TakeTask`, then `feed.Store.Handover`. This lane does not add a second lease command table or silently take over a task.

Set `GRID_MEMBER`, `GRID_ROLE`, `GRID_LEASE=task` and `GRID_LEASE_EPOCH=<gen>.<fence>` to that tenure. The owner comes from `task.md`'s `owner`, `GRID_OWNER`, or `--owner`.

```sh
grid job next --task T-01 --wait --json
grid feed tick --task T-01 --json
grid job tree --task T-01 --json
grid job blocked --task T-01
```

Use `--to <member>@<machine-id>` for an explicit assignee. Worker writes use `GRID_LEASE=<job>`, the granted epoch and `GRID_ATTEMPT` (or the corresponding flags):

```sh
grid job ack J-1 --attempt 1
grid job post J-1 --type checkpoint --body -
grid job result J-1 --body -
```

Bodies are contract JSON; `-` reads stdin. Task holders can create Jobs with `job post --type job --lease task --body -`. Cancellation requires the task tenure and `--reason`. After positively confirming restored capacity, a holder can use `job next --usage-ready --to <member>@<machine-id>`; it is never inferred from elapsed time.

## Tests and validation

Passed on Linux, with the default TMPDIR:

```sh
make test
make lint
go test -race ./internal/orchestrator ./internal/lease ./internal/feed ./internal/session ./internal/task ./internal/cli
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

The executable contract scenarios reuse the repository's schema fixture bodies. Coverage includes move tables, ready/spawn agreement and order, all gate kinds, late result refusal with an unchanged record count, two real processes running `job next`, crash-loop parking and recovery, progress resets, usage-limit reoffers and host holds, unknown liveness with no appended records, expiry and wake suppression, checkpoint retention, safe proposals, wake idempotency/rate summaries, and review capacity. These are behavioral scenarios, not a second copy of the Feed core's golden corpus.

## Intended differences from the old plugin

References below name the read-only reference documentation.

| Difference | Basis |
| --- | --- |
| The product entry point is `job next --wait`, with `feed tick` for one pass; no separate JavaScript run loop or model calls. | `docs/feed-contract.md:223-225`; old entry points: `old-plugin-map.md:56`, `old-plugin/feed/orchestrator.mjs:1-9,894` |
| A task lease authorizes controls; Job grants, fences and assignment-dependent ack replace worker claiming. | `docs/feed-contract.md:126-138,183-186` |
| Ready selection and spawn demand use one function, with no v1 priority. | `docs/feed-contract.md:226-227`; `docs/job-contract.md:74` |
| Heartbeats stay outside the Feed; an unavailable process check is unknown, and supervision controls sleep/wake handling. | `docs/feed-contract.md:202-218` |
| Usage-limit and cancelled exits do not burn the crash count; three failures ask the owner, while `max_rounds` wakes the lead. | `docs/feed-contract.md:215-216,229`; `docs/job-contract.md:137` |
| Questions are opened from the counted Feed gate and answers are selected with task controls, rather than importing the old loop's question-directory bridge. | `docs/feed-contract.md:176,188-198`; old bridge: `old-plugin/feed/orchestrator.mjs:8` |
| Structured proposals retain discovery edges and cannot silently widen the plan. Children are immutable new Jobs. | `docs/job-contract.md:9,76,117-118` |
| Wakes have durable event/verb/target deduplication and an hourly summary queue, not a per-Job stop cap. | `docs/feed-contract.md:229,251,255` |
| Text output uses `ID  ROLE  STATE  WHY` and `Nothing ready.`; JSON carries those same row fields or a wake object. | `docs/feed-contract.md:307` |

## Integration limits

- Main has no provider/account usage service. Host holds are enforced here, but automatic 70/80/90-percent account decisions and provider recovery must call `UsageReady` or explicitly use `--usage-ready` after verification. No recovery time is invented.
- Native local process checks are implemented; SSH checks, server-observed offline periods, task-lease adoption/takeover and supervision's heartbeat renewal are not. Unknown checks remain held, following this lane's explicit “never on liveness unknown” rule. The reference's 20-minute remote-host escalation needs that external authority (`docs/feed-contract.md:112,214,218-219`).
- Spawn execution, host discovery, `machines.max_jobs`/spawn-delay policy and the three-spawn-failures host circuit breaker belong to the worker integration. This lane emits requests and enforces the available usage/review waits; it does not launch agent processes (`docs/brain-feed.md:138-142`).
- The protected `proposed_jobs` schema has no explicit item or write-scope fields. Automatic promotion is therefore deliberately restricted to inherited scope and a named existing acceptance row. Other proposals wake the lead instead of guessing whether a scope change fits.
- Existing question-file commands and Feed questions do not yet share an identity bridge. Crash-loop questions are in the Feed and `needs-you.json`; this lane does not duplicate them into independently numbered question files.
- macOS was cross-built, not executed on a macOS host. Windows compiles; runtime Feed locking retains main's explicit unsupported-platform behavior.
