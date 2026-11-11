# C-gu-replan-ticks-at-once: two ticks at once post one re-plan Job

Repo plugin, branch lane-C-gu-replan-ticks-at-once, from main d1c76719. Test commit 0739cd17.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. TestGuCtTwoTicksOneReplan: gkRig(t, []any{"J-1"}), two Ticks released by the barrier, both nil, exactly one re-plan Job J-3 from J-1 | HOLDS | internal/orchestrator/gu_replan_ticks_test.go:104; `go test -race ./internal/orchestrator/ -run 'GuCt\|GkReplan' -count=1 -v` exit 0, `--- PASS: TestGuCtTwoTicksOneReplan (1.85s)` |
| 2. TestGuCtEightTicksOneReplan: eight goroutines, one re-plan Job, Feed Job ids J-1, J-2, J-3 only | HOLDS | gu_replan_ticks_test.go:113; same run, `--- PASS: TestGuCtEightTicksOneReplan (5.03s)` |
| 3. TestGuCtTwoDeadJobs: J-1, J-2 with waiters J-3, J-4; one tick assigns both (`out.Assigned == 2` is asserted); guCtAck/guCtExit act on a given Job id with Lease = that id; four Ticks at once give exactly two re-plan Jobs, discovered_from J-1 and J-2, Feed ids J-1..J-6 with no repeat | HOLDS | gu_replan_ticks_test.go:128 (rig and asserts), helpers :66 and :78; same run, `--- PASS: TestGuCtTwoDeadJobs (8.07s)` |
| 4. `go test -race ./internal/orchestrator/ -run 'GuCt\|GkReplan' -count=1` passes | HOLDS | exit 0: `ok github.com/gridconsole/grid-plugins/internal/orchestrator 26.760s`; all 7 TestGkReplan* and the 3 TestGuCt* PASS. Also `-run GuCt -count=3 -race` exit 0 (52.5s) |
| Start barrier, goroutines on a copy of the fixture Store, sync.WaitGroup, no sleep, no timeout | HOLDS | gu_replan_ticks_test.go:18-35 (`s := f.s` per goroutine; `waiting` WaitGroup closes `start` once all n goroutines reported; `done.Go`/`done.Wait`). No `time.` import, no clock read |
| New tests fail on base and pass on the lane | NOT APPLICABLE, see notes | The file is new, so base has no GuCt test (`-run GuCt` on main runs none). The gk pass on main already posts once under two Ticks at once, so no row failed on main and no production cause needed a fix. No failing lines to paste |
| Do not edit an existing test; no test deleted | HOLDS | `git diff --stat origin/main..HEAD` lists the new test file and this report only |
| Only the new file named in Touches; prefix guCt | HOLDS | package-level names in the file: guCtTicks, guCtNoErrors, guCtJob, guCtAck, guCtExit, guCtDiscovered, TestGuCt*. It reuses setup, f.job, f.tick, gkRig, gkReplans from the existing tests unchanged |
| No guard, gate, cap, hold or blocking hook added | HOLDS | tests only; no production file changed |
| Tests of the packages changed and of every package that imports them | HOLDS | `go list` importers of internal/orchestrator: internal/cli, internal/migrate, internal/orchestrator. `go test -race ./internal/orchestrator/ ./internal/migrate/ ./internal/cli/ -count=1` exit 0: orchestrator 249.5s, migrate 23.9s, cli 189.2s |
| Lint | HOLDS | `make lint` (go vet ./..., gofmt -l .) exit 0, no output after `go vet ./...` |
| Windows and macOS builds | HOLDS | `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package ok |
| Proof by go test and make only; no claude, curl, server | HOLDS | only go and make were run |
| *RealPlugin* tests | NOT APPLICABLE | the brief names none |
| UI rows | NOT APPLICABLE | no UI |
| Small commits, one sentence, no attribution lines | HOLDS | `git log origin/main..HEAD` |

## What changed

- New `internal/orchestrator/gu_replan_ticks_test.go`: `guCtTicks` runs n Ticks at once, each on `s := f.s`, behind a start channel that is closed once all n goroutines have reported; errors are collected per goroutine and checked after `done.Wait()`. Three tests (rows 1 to 3). `guCtAck` and `guCtExit` are `f.ack` and `f.exit` for a Job id: they write with a copy of `f.worker` whose Lease, Fence and Gen are that Job's, and do not change `f.worker`.
- This report.
- No production code changed.

## Notes

- Base has no GuCt tests, and the behaviour holds on main, so there is no failing run to paste. I did not run a mutation proof: this brief names no mismatch-proof row, and my attempt to make a scratch copy of the repo for one was refused. A reviewer who wants the tests shown to catch a regression can remove `fslock.LockExclusive(f)` at internal/orchestrator/store.go:141 in a scratch copy and run `-run GuCt`; I have not run it, so I claim no result for it.
- The barrier is closed when every goroutine has called `waiting.Done()`, a moment before it blocks on `<-start`; Go gives no way to see a goroutine parked on a channel without a sleep. A goroutine that has not yet parked receives at once on a closed channel, so all n are released together within scheduling.

## NOT VERIFIED on a live screen

Nothing here is UI; there is nothing to look at on a screen.
