# Leases, generation and fence

A Feed write names the tenure it was made under: a lease, a generation and a
fence. The Feed checks the writer's tenure against the lease file before it
appends. This page holds the lease file shape, how a writer names its tenure, and
every reason a write is refused as a stale lease. Line numbers are for
`internal/feed` and `internal/cli` on plugin main `dc301fce`.

## The lease file

A lease file is `feed/lease/<lease>.json`: `task.json` for the task lease and
`J-<n>.json` for a Job lease. `contract/schemas/lease.json` is a `oneOf` of two
variants. The Feed reads only `lease`, `holder`, `fence`, `gen` and `attempt`
(`feed.Lease`, internal/feed/store.go:106-112); the other properties are for the
lease commands.

| Field | Task variant | Job variant |
| --- | --- | --- |
| `lease` | the string `task` | a Job id such as `J-1` or `J-1.2` |
| `holder` | `<member>@<machine ULID>`, or null when free | the same |
| `fence` | integer, at least 1 | the same |
| `gen` | integer, at least 0 | the same |
| `attempt` | null | integer, at least 1 |
| `since` | RFC 3339 time, milliseconds, `Z` | the same |
| `pid` | integer while held, else null | null |
| `pid_start` | RFC 3339 time while held, else null | null |

Job liveness is in `.live`, not in the lease file (contract/README.md:381-382).

## How a writer names its tenure

A writer gets its tenure from the lease commands and passes it on every write.

- `GRID_LEASE`, or `--lease`: the lease name, `task` or a Job id
  (internal/cli/feed.go:60, internal/cli/orchestrator.go:67).
- `GRID_LEASE_EPOCH`, or `--lease-epoch`: `generation.fence`, for example `0.1`.
  The first number is the generation, the second the fence
  (internal/cli/feed.go:61 and :346-359, internal/cli/orchestrator.go:68 and
  :632-644).
- The worker is `<member>@<machine ULID>` (internal/cli/feed.go:341), the same
  string the lease file holds in `holder`.

The writer becomes a `feed.Writer` (internal/feed/record.go:107-111) and each
record it appends carries the envelope fields `worker`, `lease`, `gen` and
`fence` (contract/schemas/envelope.json; internal/feed/record.go:45-48). A Job
write also passes the Job's `attempt`.

A tenure's records go to one file, `<worker>.<fence>.jsonl`
(`writerFile`, internal/feed/record.go:149). When a takeover ends a tenure it
adds the line `{"file": "<worker>.<fence>.jsonl", "end_offset": <n>}` to
`feed/lease/takeovers.jsonl` (`tenureEnd`, internal/feed/store.go:114-117;
`readEnds`, :152).

## When a write is stale

`leaseLock` (internal/feed/store.go:119-150) checks the writer against the lease
file, and `WithLease` (internal/feed/guard.go:11-14) adds two refusals before it.
Each rule below returns `feed.ErrStaleLease`, which the CLI exits with code 3: do
not retry (contract/README.md:887).

| Rule | When | Code |
| --- | --- | --- |
| `no-lease` | the writer names no lease; `WithLease` refuses at internal/feed/guard.go:12 | stale lease, exit 3 |
| `fence-zero` | the writer's fence is below 1; `WithLease` refuses at internal/feed/guard.go:12 | stale lease, exit 3 |
| `unreadable` | `feed/lease/<lease>.json` cannot be opened, for example it is missing; internal/feed/store.go:123-126 | stale lease, exit 3 |
| `undecodable` | the lease file is not valid JSON; internal/feed/store.go:133-135 | stale lease, exit 3 |
| `holder` | the lease file's `holder` is not the writer's worker (null counts as empty); internal/feed/store.go:136-138 | stale lease, exit 3 |
| `lease-name` | the lease file's `lease` is not the writer's lease name; internal/feed/store.go:136-138 | stale lease, exit 3 |
| `fence` | the lease file's `fence` is not the writer's fence; internal/feed/store.go:136-138 | stale lease, exit 3 |
| `gen` | the lease file's `gen` is not the writer's generation; internal/feed/store.go:136-138 | stale lease, exit 3 |
| `attempt` | the lease is a Job lease and the call's attempt is missing or differs from the file's `attempt`; internal/feed/store.go:139-141 | stale lease, exit 3 |
| `ended-tenure` | `feed/lease/takeovers.jsonl` has a line for `<worker>.<fence>.jsonl`; internal/feed/store.go:146-148 | stale lease, exit 3 |

Notes:

- `holder`, `lease-name`, `fence` and `gen` are one comparison in the code
  (internal/feed/store.go:136), so the refusal does not say which of the four
  differed. They are four rows here because each one is a different way for the
  writer to be wrong.
- The shared lock wait (internal/feed/store.go:128-130) honours `Store.Context`.
  A cancelled wait returns the context's error, not a stale lease, so it has no
  row and does not exit 3 as a stale lease.
- A takeover log line that does not parse returns a plain error, not a stale
  lease (internal/feed/store.go:163-166).
- The task lease needs no attempt; the `attempt` rule applies when the lease
  name is not `task`.
