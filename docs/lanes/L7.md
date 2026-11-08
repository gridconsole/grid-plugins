# Lane L7 completion

Implemented sessions, supervised liveness, and the project merge lease using only the Go standard library. The existing module and Makefile are unchanged. The only changes to `internal/cli/cli.go` register the two command handlers; `internal/where` and `internal/project` are unchanged.

## What is built

- `internal/session`: one Claude session record per task, machine, and runtime session id, at `<task>/sessions/<machine>/<session>.json`. Records retain transcript paths, origin, activity times, process identity, and the persisted state. `Start` explicitly creates or resumes; `Touch` records activity; `Sleep` and `End` record lifecycle decisions.
- `Reconcile` accepts a supervisor's process observation and writes the Job tenure heartbeat at `<task>/.live/<holder>.<fence>.json`. The heartbeat interval is 60 seconds and its validity is 10 minutes. PID and start time must both match the record. Callers provide process evidence and the cadence; the package starts no timer or daemon.
- States include exactly `Stale`, `Sleeping`, and `Liveness unknown`, alongside `Running`, `Finished`, `Errored`, and `Stopped by you`. Reads return the persisted state without aging a heartbeat into a new decision. A failed process check stays unknown until the Job's 20-minute offline rule applies. Plain terminal sessions require process evidence. Wake protection persists until a successful identity check.
- `Cleanup` serializes its checks with writers, removes dead sessions' heartbeat files, and preserves ended records and transcripts. Sleeping sessions are retained without probing. Failed checks are unknown, not proof of death.
- `internal/lease`: a 15-minute merge lease per project at `<project>/lease/merge.json`, with take, renew, release, status, and takeover at expiry. Every grant gets a monotonically increasing fence from `lease/seq`; renew and release require both holder and fence. The lease inode stays in place under `flock`, readers use a shared lock, and mutations and counter writes are fsynced. Release retains the record with a null holder. Corrupt nonempty state fails closed.
- `grid session list/show`: task scope when inside a task or passed `--task`; otherwise the resolved project's tasks. `--machine` disambiguates a session resumed on another machine. Both commands support `--json` and do not write.
- `grid lease take/renew/release/status`: merge leases only, with optional literal `merge`, `--project` or `--task`, and `--json`. The grant and held refusal wording follows the contract. A stable `--holder` is required for mutations; renew and release also require the returned `--fence`.

For an existing project and task:

```sh
make build
./dist/grid session list --project example --json
./dist/grid session show session-1 --project example --task T-01 --machine machine-1
./dist/grid lease take merge --project example --holder session-1
./dist/grid lease status merge --project example --json
./dist/grid lease renew merge --project example --holder session-1 --fence 1
./dist/grid lease release merge --project example --holder session-1 --fence 1
```

Use the fence actually returned by take in place of `1`.

## Tests and validation

```sh
make test
make lint
go test -race ./internal/session ./internal/lease ./internal/cli
go test ./internal/lease -run '^TestMultiProcessRace$' -count=1
```

Fake-clock tests cover exact expiry boundaries, renewal, stale-holder rejection, unknown liveness, wake handling, sleep/resume, machine identity, PID reuse, read-only inspection, and cleanup. The process race test starts 12 independent processes against a free lease, then another 12 at expiry, and requires exactly one winner in each round. CLI tests cover plain and JSON output, project/task resolution, ambiguity, refusal messages, and usage errors. Storage tests cover corruption and symlinks.

Validation completed: `make test` (which runs `go test ./...`), `make lint`, and race checks on the new session, lease, and CLI tests. A Darwin arm64 build also validates the standard-library locking API on the other supported OS target; it is not a runtime test on macOS.

This managed environment exposes a synthetic `.git` under its default temporary directory, which breaks the existing resolver tests' assumption that their temporary folders are outside a checkout. Its default Go cache is also read-only. The full suite passed with a mount namespace backed entirely by this checkout's ignored `dist/` folder, without changing those tests or the resolver:

```sh
mkdir -p dist/tmp
bwrap --ro-bind / / --dev /dev --proc /proc \
  --bind "$PWD" "$PWD" --bind "$PWD/dist/tmp" /tmp --chdir "$PWD" \
  env GOCACHE="$PWD/dist/go-cache" TMPDIR=/tmp make test
GOCACHE="$PWD/dist/go-cache" TMPDIR="$PWD/dist/tmp" make lint
```

The environment required sandbox review for that namespace and for the requested commits. Normal checkouts can use the ordinary commands above.

## Intended differences from the old plugin

References below name files in the read-only reference bundle; design documents are under `docs/` and old implementation files are under `old-plugin/`.

| Difference | Reference and reason |
| --- | --- |
| Session identity includes the machine, rather than only the sanitized session id. The selected layout nests records by machine inside each task. Invalid path components are refused instead of silently rewritten. | `go-architecture.md:267` requires runtime session id plus machine id; old layout and sanitization are at `hooks/lib.mjs:450` and `hooks/lib.mjs:459`. The exact nested layout and strict validation are implementation choices. |
| Session updates are serialized with a persistent OS lock and use unique temporary files and rename. Errors are returned, and `Touch` is not suppressed by the old 10-second throttle. | Old `hooks/lib.mjs:455` describes the throttle and silent failure; `hooks/lib.mjs:480` uses a fixed temporary name. Serialization, explicit errors, and unthrottled metadata updates are implementation choices for safe concurrent callers. |
| Hook activity never renews liveness. The worker's successful process observation writes `.live`, with a 60-second cadence and 10-minute validity. No heartbeat is appended to the Feed. | `plugin-design.md:90`; `feed-contract.md:202`, `feed-contract.md:203`, `feed-contract.md:210`, `feed-contract.md:211`; old behavior is mapped at `old-plugin-map.md:43` and `old-plugin-map.md:138`. |
| Reader commands never infer stale from file age or silence; the task lease holder supplies observations and persists decisions. The record keeps the display state rather than deriving a “stuck” label. | `feed-contract.md:212`; `go-architecture.md:275`; exact display labels at `prototype/sessions-tab.js:129`, `prototype/sessions-tab.js:130`, and `prototype/sessions-tab.js:132`. |
| Failed checks become `Liveness unknown`; Job sessions become stale/offline only after the 20-minute grace and expiry, unless a successful check proves the process gone sooner. Plain terminal sessions do not expire on heartbeat age. | `feed-contract.md:211` and `feed-contract.md:213`. |
| A wake cannot consume a Job's timeout without a process recheck. Observation identity prevents delayed results for a previous process from ending a resumed session. | `feed-contract.md:101` and `feed-contract.md:218`. The persisted `wake_pending` flag is an implementation choice. |
| Intentional sleep is preserved separately from death, and explicit resume keeps the first start and transcript. Cleanup removes liveness files but retains session history. | `plugin-design.md:193` requires sleep and resume by session id; `job-contract.md:144` requires earlier-attempt session history to remain available. The cleanup retention policy implements those requirements. |
| Merge coordination is a new per-project 15-minute advisory lease, independent of the old 10-minute Job lease. Expired holders cannot renew or release a subsequent grant, even if the holder string is reused. | `plugin-design.md:113`; `feed-contract.md:115`; the old 10-minute constant is `feed/lib.mjs:23`; `old-plugin-map.md:118` identifies merge take as a new command. |
| Merge coordination uses persistent `flock` locks, in-place lease writes, fsync, a fence counter, and null-holder release. It does not reuse the old heartbeat lock's age-based unlink scheme. | `feed-contract.md:94`, `feed-contract.md:101`, `feed-contract.md:103`; old heartbeat lock behavior is `hooks/heartbeat.mjs:83`. |
| Session inspection and merge coordination are typed Go CLI commands with JSON support. Holder/fence flags and project-wide session listing are explicit interface choices where the brief did not prescribe syntax. | `plugin-design.md:113`, `plugin-design.md:118`; `go-architecture.md:152`; exact merge take/refusal text is `feed-contract.md:308`. |

## Integration boundaries and remaining work

All requested packages, read commands, merge commands, fake-clock tests, and the process race test are implemented. This lane does not implement the closed worker, a process launcher, OS-specific process probes, or a periodic loop. Hook and worker callers still need to invoke `Start`, `Touch`, `End`, and `Reconcile`; worker supervision must compare PID plus start time, detect monotonic/wall-clock wake gaps, and supply `AfterWake`. That separation follows `go-architecture.md:152` and `go-architecture.md:221`; hooks must not generate liveness (`plugin-design.md:90`).

These APIs store session state; they do not post Job `stale` records, modify the Feed fold's `state.json`, assign another attempt, grant task/Job leases, or implement task takeover logs and server fencing. Those require the Job/Feed lease-holder integration described in `feed-contract.md:118`, `feed-contract.md:212`, and `feed-contract.md:213`. Callers must enforce that only the current task lease holder supplies liveness decisions. The merge lease remains advisory and is safe between processes sharing the same local filesystem, not between independently synced machines; Git still decides landing races (`feed-contract.md:115`).

There is no migration of old pointer, usage, recall, or flat session files in this lane; new records use the machine-aware layout, and migration remains separate. Native Windows locking is not implemented: this slice targets Linux and macOS, consistent with WSL2 being the supported Windows worker path (`go-architecture.md:22`). The worker and macOS runtime behavior could not be exercised because neither runtime is present in this checkout.
