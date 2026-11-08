# Lane L30: browser Job ports and environments

Implemented `internal/ports`, `internal/env`, a read-only Job lease reader in
`internal/lease`, and CLI registration for:

```text
grid ports claim J-1 --json
grid ports release J-1
grid ports list --json
grid ports gc
grid env up J-1
grid env down J-1
```

Run Job commands inside the task folder. Outside a task, a saved `jobs/J-1`
directory must identify exactly one task in the Grid home. Task discovery uses
`gridhome.TasksDirs`, the existing resolver, and `internal/task.Read`. Job ids
are scoped to tasks, so two tasks may each claim `J-1` without sharing ports.

## Configuration and behavior

Add this entry to an existing project's `project.md` front matter:

```yaml
environments:
  command: "exec npm run dev -- --port {port}"
  health_url: "http://127.0.0.1:{port}/health"
  timeout: 30s
  port_range: [41000, 41999]
```

The range is inclusive and defaults to 41000–41999. Allocation starts at its
lower bound in blocks of ten; an incomplete final block is unused. The project
must name exactly one checkout folder for the current machine in its existing
`folders` entry. The command runs there through `/bin/sh -c`. Keep the server
in the foreground; `exec` avoids an unnecessary shell parent. Both command and
health URL expand `{port}` to the block's first port. The child also receives
`PORT`, `GRID_PORT`, and `GRID_JOB`. The timeout defaults to 30 seconds and uses
Go duration syntax. Health requires HTTP 2xx; redirects do not count as health.

Each port has an O_EXCL-created `ports/<port>.json` reservation under the Grid
home, containing the full block, pid, Job, relative task path, lease id, fence,
and UTC claim time. `internal/fslock` serializes claims, reads, release, GC,
and environment lifecycle transactions. The CLI's parent process owns a plain
claim because the claim CLI itself immediately exits. Environment startup hands
the block to its new process group and records that group's pid before waiting
for health. Server output goes to `ports/env-<first-port>.log`.

Repeated claim is idempotent. Repeated up checks the existing server's health.
Startup errors, occupied ports, and health timeouts release the block after
stopping any server started for it. Down sends SIGINT, waits two seconds, sends
SIGTERM, waits two seconds, then sends SIGKILL if needed. It allows another two
seconds for reaping after SIGKILL. Down is idempotent. Plain release refuses a
live environment group and directs the caller to down.

GC reclaims a dead owner or group. For a live owner it requires a released or
superseded lease, or an expired matching heartbeat, and no TCP listener anywhere
in the block. It checks IPv4 and IPv6. Unsupported address families are ignored;
other bind errors retain the reservation. It reads `lease/<J>.json` with a shared
fslock and validates the matching `.live` heartbeat using the contract schemas.
Missing grants or heartbeats do not prove expiry. No lease, heartbeat, Feed, task,
or project file is written by these commands.

## Verification

Run from the repository root, with the normal system TMPDIR:

```sh
make test
make lint
go test -race ./internal/ports ./internal/lease ./internal/env ./internal/cli
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

The tests cover 20 concurrent goroutine claims and 20 separate claimant
processes, unique ten-port blocks, range exhaustion, task isolation, idempotent
release, GC after killing a process, matching lease expiry, listener protection
including IPv6, malformed and mismatched leases, and refusal of storage symlinks.
A small Go server is built by the lifecycle and CLI tests. It exercises startup,
health checks, a real descendant process, SIGINT/SIGTERM/SIGKILL escalation,
repeated up/down, health timeout cleanup, and redirect rejection. CLI tests also
cover JSON output, custom project ranges, ambiguous Job ids, and usage errors.

All commands above passed on Linux. The Darwin build passed; macOS runtime tests
could not be executed because this lane has no macOS runner. Tests use the default
temporary directory and resolve Grid home paths rather than rejecting symlinks
in system temporary-directory ancestors.

## Intended changes from the old plugin

References below name the supplied reference documents and their line numbers.

- Add local ten-port allocation, O_EXCL reservations, listing, release, and GC.
  The design explicitly identifies missing parallel-lane isolation as wb-F2:
  `docs/go-architecture.md:28` and `docs/go-architecture.md:245`.
- Start a dev server and prove health before a browser Job, then stop its process
  group and release its ports. These implement wb-F3 rather than assuming the
  checker already has a server: `docs/plugin-design.md:31`,
  `docs/plugin-design.md:191`, and `docs/go-architecture.md:246`. Signal escalation
  follows the supervision sequence in `docs/go-architecture.md:9`.
- The new `env` commands manage dev processes; the old `grid-env.mjs` hook only
  discovered Grid directories and exported them to the session, as described in
  `old-plugin-map.md:31`. Configuration comes from this project's environment
  entry, consistent with `docs/go-architecture.md:17` and
  `docs/go-architecture.md:245`. This lane does not replace that directory hook.
- Read Job leases and local supervision heartbeats instead of posting old Feed
  heartbeat events. The old hook behavior is recorded in `old-plugin-map.md:43`;
  shared-lock lease reads and local `.live` files are specified by
  `docs/feed-contract.md:94`, `docs/feed-contract.md:203`, and
  `docs/feed-contract.md:211`. These commands never renew a lease or declare a
  Feed Job stale.
- Keep a block whose lease has expired while a listener remains, matching
  `docs/go-architecture.md:245`. Checking both TCP families and refusing startup
  on an occupied block make this rule apply to servers outside Grid as well.
- Native Windows operation refuses explicitly instead of silently starting a
  server that cannot be stopped as a group. This is the documented-refusal option
  in this lane's brief; the process-group requirement comes from
  `docs/go-architecture.md:246`. Windows compilation remains supported.

## Boundaries and limitations

- The protected `internal/project.Record` does not expose environment fields.
  `env.Load` therefore validates identity through `project.Parse` and reads the
  extension through the existing `internal/job/frontmatter` parser. It introduces
  no new YAML splitter and does not change `internal/project`. The supported
  entry is the single mapping shown above, not a registry of named environments.
- Native Windows job object supervision is not implemented. Both environment
  operations refuse there; the existing fslock implementation also refuses port
  operations. Use Linux, macOS, or WSL2 for execution.
- Startup and shutdown hold the shared allocator transaction to prevent GC,
  release, or another up from racing the pid handoff. This serializes local
  allocation operations during health waits and shutdown grace periods.
- Reservations coordinate Grid processes; they are not kernel socket leases.
  An unrelated process can bind after the availability probe. Process supervision
  uses a pid or process group, not a native process start-time token, so pid reuse
  remains a limitation; GC additionally checks the recorded lease fence.
- Malformed or inconsistent port files fail closed. A crash in the middle of a
  multi-file update may require inspecting and repairing the local reservations;
  GC does not guess ownership of corrupt records. Child output logs are retained
  locally and are not rotated by this lane.
- Worker scheduling, automatic calls around browser Jobs, browser installation,
  and Job-render integration are outside this lane. These commands provide the
  lifecycle operations described in `docs/go-architecture.md:245` and
  `docs/go-architecture.md:246`; worker ownership is described in
  `docs/plugin-design.md:121`.
