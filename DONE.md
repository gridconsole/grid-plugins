# Lane L24: intent files

Implemented `internal/intent` and CLI adapters for session start, stop, send,
pause, resume, handback and switch, plus task move. Every action writes an
immutable ULID-named JSON request beneath the selected task's `intents/` folder.
No action launches an agent or edits a task, session or Feed record. Existing
session list/show commands remain available.

The store includes five-second send coalescing, atomic file publication,
exclusive claim markers, five-minute claim expiry, stale-token rejection,
completion/error records, and pending/all listing. The request, claim,
completion and CLI JSON contracts are documented in `contract/README.md` under
"Task intent files". No changes were made to `go.mod`, `internal/contract`,
`internal/where`, `internal/project` or `internal/fslock`; changes to
`internal/cli/cli.go` only register commands.

## Usage

Run inside an existing task folder, or add `--task <path-or-id>` and optionally
`--project <name-or-id>`:

```sh
grid session start --agent planner
grid session send 'Continue the task' --idempotency-key request-example
grid session pause
grid session resume
grid session stop
grid session handback
grid session switch builder
grid task move worker-example
grid intent list --json --pending
grid intent claim <id> --who worker-example --json
grid intent done <id> --claim <returned-token>
grid intent done <id> --claim <returned-token> --error='agent unavailable'
```

`GRID_INTENT_CLAIM` can supply the completion token instead of `--claim`.
`--error` alone records a generic worker failure. All writes accept `--json`.
Session actions accept `--target <session>`; without it the selected task is the
target. `--who` defaults to `cli` for requests and `worker` for claims. These
are caller labels, not authenticated identities. Move accepts the alternate
`grid task move <task> --to <machine>` syntax too. It checks every configured
local repository for a remote before queuing anything.

## Validation

Passed from this checkout, with the default TMPDIR:

```sh
make test
make lint
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

`make test` includes table-driven temporary-folder cases for every verb and
invalid payloads, CLI usage and lifecycle tests, send deduplication and expiry,
explicit-key conflicts, multi-repository moves, symlink refusal, malformed
claim recovery, and JSON goldens in `testdata/golden/intent-list.golden`.
`TestTwentyProcessesAndCrashRecovery` starts 20 separate processes behind a
release gate; exactly one claims the same intent. The winner exits without
done. Advancing the test clock past expiry makes the request claimable again,
and the old token cannot finish the replacement claim. The timeout is tested
without waiting five real minutes. `make golden` also checks the JSON fixture.

## Intended differences from the old plugin

Reference names and line numbers below refer to the read-only source material.

- Agent actions are durable requests, not detached process launches or direct
  desktop operations. The worker interprets them, including pause, resume,
  handback and switch. Sources: old-plugin/feed/worker-start.mjs:20-35;
  go-architecture.md:106,354,372,596; prototype/task.html:12195,12222-12228.
- Requests survive an offline worker and do not call the old HTTP bridge.
  Sources: old-plugin/hooks/lib.mjs:1196-1214;
  go-architecture.md:178-199,273. A successful CLI write means queued, not that
  an agent has started or stopped.
- `intents/<ULID>.json` is an action queue, separate from the old `intent.md`
  prompt document, session records and Feed posts. No task is created from
  send text, and no task status is changed by enqueueing. Sources:
  old-plugin-map.md:35,70,74; plugin.md:249,252-258;
  sessions.md:118; go-architecture.md:372. Sending a Feed message, if needed,
  belongs to the worker's handling of the request, not this queue writer.
- Move only records a destination. It does not use SSH, clone, push, copy a
  login or start a process. A missing remote refuses the request before any
  queue write. Sources: old-plugin-map.md:61-62,114;
  go-architecture.md:376-384. Both the lane's `<machine>` spelling and the
  architecture's `<task> --to <worker>` spelling are supported.
- The prototype's terminal attach/resume and direct UI state changes are not
  runtime implementations here. Session list/show remain readers; interactive
  terminal keystrokes remain outside the intent protocol. Sources:
  prototype/terminal.html:658-670; prototype/task.html:12217-12228;
  go-architecture.md:224,372.
- The JSON shape, string-valued argument allowlist, caller defaults, optional
  explicit send key, five-second coalescing window, five-minute claim timeout,
  completion token, terminal error state and pending filter are newly specified
  queue details. The architecture requires persistent intents but does not
  prescribe these fields or timings: go-architecture.md:178-199,372,596.
  They do not reuse the old Feed claim/lease semantics documented at
  old-plugin-map.md:55,72,138. Coalescing also covers completed sends within
  the window; conflicting text under one explicit key fails. Pending means
  available to claim now, excluding unexpired claims and completed failures.
- Worker completion requires its claim token, by flag or environment. This
  extends the short done spelling to fence a worker that returns after expiry.
  Requests remain immutable; claim and completion files carry lifecycle state.
  Recovery uses exclusive creation and fslock, with at-least-once delivery after
  a crash. This concretizes the persistence requirement at
  go-architecture.md:178-199; external effects still need intent-ID idempotency.

## Limits and work not performed

- No worker runtime or desktop is part of this checkout. Actual launches,
  terminal control, orchestration, moves through the server and online-worker
  checks were not exercised or implemented; those belong to the worker per
  go-architecture.md:372-384. Session/task/Feed updates after processing remain
  that worker's responsibility.
- The 20-process race ran on a local Linux filesystem, not two physical
  machines sharing storage. Cross-machine guarantees require a shared
  filesystem honoring distributed flock, atomic O_EXCL and hard links, plus
  synchronized worker clocks. Replicated independent folders are not one lock
  domain. A live shared-filesystem deployment was not available to test.
- Darwin and Windows compilation passed, but native execution on those systems
  was not available. Windows intent operations retain the existing fslock
  unsupported-platform behavior; this lane was forbidden to change fslock.
  Temporary-folder handling resolves symlink ancestors for macOS.
- Claims expire after five minutes with no renewal command. A worker must finish
  handling the request within that lease and stop acting after expiry. A crash
  after an external effect but before done can cause redelivery; exactly-once
  external execution cannot be supplied by intent files alone.
- Move validates actual local Git remotes, not destination reachability or
  credentials. A project without a repository folder configured for this
  machine cannot pass that check. Remote data transfer remains outside this lane.
