# F-leases-map: leases, locks and claims, for verdict v-8

Docs only. No code, no test, no guard added. Base: main (842f006e). Paths are under `internal/`.

## Proof table

| Row / rule | Result |
|---|---|
| 1 every lease, lock, claim with file:line, taker, reader, second machine | HOLDS: sections A to C below |
| 2 one line each: crosses (keep) or same machine (built-in named) | HOLDS: the Verdict column |
| 3 plugin call sites the app reaches | HOLDS: section D (the app repo was not read) |
| 4 under 150 lines, ends with removals and covering tests | HOLDS: `wc -l` on this file; section E is last |
| 5 `make lint` 0 fail | HOLDS: `make lint` exit 0 (`go vet ./...`, no findings) |
| Windows and macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| Tests of changed packages and importers | NOT RUN: no package changed, so the `go list` set is empty; the orchestrator runs the module |
| New tests fail on base | NOT APPLICABLE: no new test |
| No guard, gate, cap, hold on agents | HOLDS: nothing added; section E only proposes deleting |
| Commits one sentence, no attribution | HOLDS: see `git log` on the lane |

## How to read "crosses machines"

The task folder is the shared unit. Its files are read by other machines: session records are
listed across every machine (`session/session.go:406`), writers are named `member@machine`
(`feed/store.go:122`), and the intent store says shared filesystems must support exclusive create
(`intent/intent.go:67`). An `flock` itself only excludes processes on one host. So an entry
**crosses** when a second machine reads its content and acts on it (a fence check, a takeover, a
claim). It is **local** when only the machine that wrote it reads it.

## A. Task-folder state (shared, so the content can reach a second machine)

| What | file:line | Taker | Reader | 2nd machine reads? | Verdict |
|---|---|---|---|---|---|
| Task lease `feed/lease/task.json` | `lease/task.go:113` (TakeTask), `:237` (AcquireTask), `:272` (Release) | `grid lease take task` `cli/task_lease.go:338,346`; worker stream `orchestrator/stream.go:181` | every Feed write `feed/store.go:109,122`; `orchestrator/write.go:37`, `tick.go:348,392`; `task/trash.go:216` | YES: takeover checks a holder on another host (`lease/task.go:157`, person plus 15 min server-observed offline) | CROSSES, keep |
| Job lease `feed/lease/<id>.json` | `lease/task.go:310` (GrantJob), called `orchestrator/tick.go:187` | task holder only | `lease/job.go:32`, `ports/ports.go:179,434`, `feed/store.go:125` | YES: the Job is granted `to` a `worker@machine` | CROSSES, keep |
| `feed/lease/takeovers.jsonl` (ended writer files) | `lease/task.go:406` (end) | takeover/adoption | `feed/store.go:140,132` | YES: stops a replaced writer on any machine | CROSSES, keep |
| Fence counter (per task) | `lease/task.go:359` | grant | grant | with the lease | CROSSES, keep (new fence is higher than any old one) |
| Writer file append lock `<worker>-<fence>.jsonl` | `feed/store.go:248-256` | each Feed post | same | NO for the lock; YES for the file content | flock = same machine; the fence covers the cross case. Keep |
| Gate lock `gate.lock` | `feed/gate.go:22` | question/gate post | same | NO (flock). Does not exclude a gate id raced from a second machine | LOCAL (OS flock). Keep; open point below |
| Wake lock `wake.lock` | `feed/store.go:553` | wake records | same | NO | LOCAL (OS flock). Keep |
| `.task.lock` (folder writes, moves, trash) | `task/write.go:458`, `task/history.go:117`, `task/trash.go:126`, `feed/identity.go:118`, `orchestrator/plan.go:37`, `lease/task.go:117` | any task writer | same | NO | LOCAL: OS flock plus the inode check at `task/trash.go:139`. Keep |
| `.status.lock`, `.brain-decide.lock`, `.land.lock` | `task/brief.go:32`, `brain/task_decide.go:74`, `land/land.go:201` | one writer each | same | NO | LOCAL (OS flock). Keep |
| Session records `sessions/<machine>/<id>.json` and `.lock` | `session/session.go:138`, lock `session/files.go:88` | `session` verbs, hooks `hook/events.go:264,300` | `session/session.go:406` lists every machine | YES (records); lock NO | CROSSES (record), flock local. Keep |
| `feed/.live` heartbeat | `session` Reconcile via `lease/activity.go:8` | supervision only | `lease/job.go:32` | NO: read beside the lease on the same host; a missing beat proves nothing (`lease/job.go:19`) | LOCAL. Keep (read-only) |
| Question and ReserveGate id lock `.lock` | `question/files.go:61`, `question/reserve.go:17` | `grid question` | same | NO (flock) | LOCAL (OS flock). Keep |
| Intent claim `<id>.claim` (O_EXCL), `.release.json`, `.lock` | `intent/intent.go:305` (Claim), `:344` (Renew), `:384` (Release); lock `intent/files.go:51`; marker read `files.go:96` | `grid intent claim` `cli/intent.go:250` | list/claim/done `intent/intent.go:261`, `cli/intent.go:234` | YES: a worker on another machine claims; claim expires by time, so clocks must agree (`intent.go:69`) | CROSSES, keep |

## B. Grid-home state (one per machine, never shared)

| What | file:line | Taker | Reader | 2nd machine? | Verdict |
|---|---|---|---|---|---|
| Merge slot `projects/<p>/lease` | `lease/lease.go:85` Take, `:103` Renew, `:106` Release; `cli/liveness.go:63` | `grid lease take [merge]` | `grid lease status` only; no skill, topic, agent or hook calls it (grep of `skills topics agents agent-templates hooks`) | NO: "all participants must use the same local filesystem and clock" (`lease/lease.go:57`) | SAME MACHINE. Built-in: git refuses a non-fast-forward push, and `land.lock` (`land/land.go:201`) serialises the deploy. Removal candidate |
| `task-take.lock` (take retry and handover) | `cli/task_lease.go:262` | `grid lease take task` | same | NO | LOCAL (OS flock). Keep |
| `--hold` holder (the shell pid) | `cli/task_lease.go:65,85` | `grid lease take task --hold` | adoption `lease/task.go:150` | NO: adoption demands a provably gone process on this host (`:152`) | LOCAL; the process table is the check. Keep |
| Same-host adoption `AdoptTask` | `lease/task.go:185` | worker stream, CLI | same | NO (dead-pid probe is local) | LOCAL. Keep; nothing built in frees our lease |
| `worker.lock`, `worker.json` (app-owned) | `worker/worker.go:101,97`; other machine's file is dropped `:120,125` | the app | `grid worker`, `cli/open_start.go:57` | NO | LOCAL. The plugin only reads. Keep |
| `open-start.lock` | `cli/open_start.go:25,130` | `grid open` | same | NO | LOCAL (OS flock). Keep |
| Port blocks `.lock`, claims | `ports/ports.go:109,127,163` | `grid ports claim`, `grid env up` | `ports/ports.go:248,384` | NO | LOCAL. OS bind is only a probe (`ports.go:463`), so the claim is not covered. Keep |
| Machine, signals, projects, routine, proof, usage, workspace, settings, account, plan, workflow, migrate, doctor locks | `localdata/localdata.go:128,177`; `signal/signal.go:214`, `signal/move.go:45`, `signal/act.go:104`; `projects/projects.go:209,409,554`; `projectmd/projectmd.go:81`; `routine/store.go:179`, `routine/schedule.go:227,322,413`; `proof/proof.go:95,303`; `usage/usage.go:128`, `usage/workspace.go:106`; `settings/store.go:265`; `account/registry.go:108`; `plan/store.go:88`; `workflow/write.go:159,181`; `migrate/recover.go:57`; `doctor/bundle.go:135`; `brain/store.go:157`; `task/create.go:43`, `task/trash.go:95` | each verb | same | NO | LOCAL: these are the OS flock itself, there is nothing to replace it with. Keep |
| Feed caches `reader.lock`, `fold.lock`, `.orchestrator.lock` | `feed/cache.go:47`, `feed/fold.go:66`, `orchestrator/store.go:133` | readers, tick | same | NO | LOCAL (OS flock). Keep |

## C. The primitive

`fslock` (`fslock/fslock_unix.go:12-28`, `fslock_windows.go:27-42`, `fslock_other.go`) is the OS
flock or LockFileEx behind every flock row above. It is the built-in; it is not a lease.

## D. Plugin call sites the app reaches

The app repo was not read. Go packages are under `internal/`, so the app cannot import them. The
only exported Go package is `contract/embed.go:2` (`contractdata.Schemas`, JSON schemas). Every
other path is the `grid` binary. Verbs that touch this map (`cli/cli.go`):

- `lease` `:112` (merge and task; `cli/liveness.go:20`, `cli/task_lease.go:42`), `job` `:111`, `feed` `:113`
- `intent` `:136` (`cli/intent.go:192`), `ports` `:126`, `worker` `:130` (`cli/worker.go:52`)
- `session` `:135` (`cli/liveness.go:108`), `land` `:122`, `open` `:105` (`cli/open_start.go:57,130`)
- `mcp` `:114`: Feed tools carrying the lease epoch (`cli/mcp.go`, `mcp/tools.go`)

Whether the app runs `grid lease take` without `task` (the merge slot) is NOT KNOWN from this repo.

## E. Removals this map implies

1. **Merge slot only** (`lease take|renew|release|status [merge]`): `lease/lease.go`, the merge
   branch of `cli/liveness.go:20-107`, and its `contract` lease schema use if unshared. Nothing in
   the plugin calls it and no second machine can read it. Delete only after the app owner confirms
   the app does not call it.
   - Tests to delete with it: `lease/lease_test.go` (TestLifecycle, TestCorruptionAndSymlinks,
     TestTornRecordRecovers, TestFenceSurvivesAMissingCounter, TestHolderRefusesControlCharacters,
     TestMultiProcessRace, TestWrittenMergeLeaseContract, TestFenceCannotOverflowContractInteger);
     `cli/liveness_test.go` TestLeaseCommands, TestLeaseCommandWritesContract,
     TestLeaseRejectsInvalidHolderBeforeWriting, TestLeaseCommandPreservesStoredGeneration.
   - Tests that keep covering the kept behaviour: `lease/task_test.go`, `lease/acquire_test.go`,
     `lease/job_test.go`, `lease/job_feed_test.go`, `feed/tenure_test.go` TestTakeoverRacesWriters,
     `feed/contract_test.go` TestFencingAndHolds, `cli/liveness_test.go` TestTaskLease*,
     `cli/orchestrator_test.go` TestTaskTakeoverCLI, `intent/intent_test.go` TestClaimLifecycle and
     TestTwentyProcessesAndCrashRecovery, `fslock/fslock_unix_test.go`.
   - Landing safety stays with git and `land.lock`: `land/runtime_test.go` TestConcurrentHistoryAppend.
2. **Nothing else.** The rest is either a cross-machine fence (keep) or the OS flock/process
   table already doing the job (the code is the built-in). Dropping them would remove atomicity,
   not duplication.

Open points, not removals: `gate.lock` (`feed/gate.go:22`) and `intent` claim expiry rely on one
host or synchronised clocks; `feed/store.go:233` checks a gate id across writers only on one host.
