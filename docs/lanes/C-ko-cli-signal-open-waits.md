# C-ko-cli-signal-open-waits

Plugin repo, branch lane-C-ko-cli-signal-open-waits. Base: main 21f02736 (the brief quoted dc301fce; `git merge main` says "Already up to date" on this base). The four sleeps were on the lines the brief names: open_start_test.go:138, :140 and signal_act_test.go:482, :485. No line moved.

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1 | `git grep -n 'time\.Sleep' -- internal/cli/signal_act_test.go internal/cli/open_start_test.go` prints nothing | HOLDS. It printed nothing after the commit (git grep exits 1 on no match; the harness refused my `|| echo` wrapper, so I did not capture the code itself). On base it printed :138, :140, :482, :485. |
| 2 | `go test -count=1 -v -run 'TestSignalConcurrentTurnProcesses\|TestOpen' ./internal/cli/` | HOLDS, exit 0. Lines: `--- PASS: TestSignalConcurrentTurnProcesses (0.19s)`, `--- PASS: TestOpenParallelStartsEachProgramOnce (0.37s)`, `--- PASS: TestOpenReplacesAStaleAppURL (0.36s)`. No FAIL or SKIP line for any TestOpen name. |
| 3 | `go test -count=3 ./internal/cli/` | HOLDS, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/cli  260.190s` |
| 4 | `git diff main --stat` names only the two files | HOLDS: `internal/cli/open_start_test.go` and `internal/cli/signal_act_test.go`, 2 files changed. This report is under docs/lanes/ and is the only other file in the branch (see "What changed"). |
| 5 | Walk tests after `git merge main` | HOLDS. `git merge main`: Already up to date. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `ok ... 0.322s`. `go test -count=1 ./internal/job/lint/`: `ok ... 0.615s`. Both exit 0. |
| 6 | `make lint` and `make cross` | HOLDS. `make lint` (go vet ./...) exit 0, no output. `make cross` exit 0: `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`, no FAIL line. |

## Rules

| Rule | Result |
|------|--------|
| Packages that import the changed one | HOLDS. The only changes are two `_test.go` files of `internal/cli`, so no importer sees a different package. `go list` shows `cmd/grid` and `lib` import `internal/cli`. `go test -count=1 ./cmd/grid ./lib`: `cmd/grid [no test files]`, `lib ok 44.885s`. The package itself passed row 3. |
| `gofmt -l internal/cli/` | HOLDS: prints nothing. `go vet ./internal/cli/` exit 0. |
| `go test -race` on the new stderr reader | HOLDS: `go test -race -count=1 -run 'TestSignalConcurrentTurnProcesses\|TestOpenParallelStartsEachProgramOnce' ./internal/cli/` exit 0. |
| New tests fail on base and pass on the lane | Not applicable as written: these are two existing tests rewritten, not new tests, and the base versions pass too (they slept). No existing test was edited to make it pass; no test was deleted. The proof that the new waits depend on the events is the mutation run below. |
| Mismatch proof (my own check, not a row) | With the child hook suffix changed to `.signals.lockX` and the open prefix changed to `grid open: startedX `, the run fails: `TestOpenStartsWorkerAndServerWhenNotRunning: starts: worker 0, server 0, want 1 and 1`, `TestOpenParallelStartsEachProgramOnce: two opens at once: worker started 0 times, server 0, want 1 and 1`, `TestOpenReplacesAStaleAppURL: starts: server 0, worker 0, want 1 and 0`, `TestSignalConcurrentTurnProcesses: child 0 did not wait for the signal lock: ""` (10.04s, the failure bound). Both edits were reverted with `git checkout --`; `git status --short` printed nothing before the commit of this report. |
| No guard, gate, cap or hook on agents; no production file | HOLDS. Only test files changed. |
| No `RealPlugin` test touched | Not applicable. |
| No personal path, host or email in the diff | HOLDS. |

## What changed

internal/cli/signal_act_test.go:
- TestSignalTurnProcess (the child) sets `fslock.OnBlockForTest` before Run. For a path ending `.signals.lock` the hook writes `koSo blocked` to the child's stderr and never blocks. The hook is not restored: the child ends with `os.Exit`.
- The ready file and the `GRID_SIGNAL_TEST_READY` env var are gone.
- TestSignalConcurrentTurnProcesses reads each child's stderr through `cmd.StderrPipe()` (helper `koSoReadStderr`, type `koSoStderr`). The wait for the line is a select with three cases: the event line, the child's pipe ending (the child exited, which fails the test) and a 10 s failure bound.
- After the parent unlocks, the test waits for each pipe to end before `cmd.Wait()`, as the os/exec docs require. The event line is taken out of the stderr text as it is read, so the exact loser check (`grid: this note is already decided (done)\n`) sees the text without it.
- New package-level names: `koSoBlocked`, `koSoFailBound`, `koSoStderr`, `koSoReadStderr`.

internal/cli/open_start_test.go:
- `starts` and `counts` read the `grid open: started ...` lines of the run's stderr. `koSoStarted(stderr, name)` counts one per line that names the program ("started grid-worker and grid-server." names both).
- `counts()` takes no wanted numbers; with no argument it reads `f.errb`. TestOpenParallelStartsEachProgramOnce passes `errs[0].String(), errs[1].String()`. Direct caller TestOpenMissingProgramExitsThreeAndStartsNothing calls `f.starts("grid-worker")`. The five call sites (four through counts, one direct) were adjusted for the new signature only; the expected numbers are the same as before.
- No wait: `grid open` has returned when the test reads the line.

docs/lanes/C-ko-cli-signal-open-waits.md (this file) is added; it is the only file outside Touches.

## Left as it is (named in the brief)

- The 5 ms marker poll of the fake worker goroutine (open_start_test.go, `time.After(5 * time.Millisecond)` in newStartFixture) and the `sleep 0.3` inside the fake grid-server shell script. Both sit in detached shell programs with no pipe back. INDEX.md of backlog-11nov-ko lists them for a later folder. The brief's row 1 greps `time\.Sleep` only, and `time.After` is not matched.
- `startDetached` programs still write their marker file; the fake worker goroutine still needs the grid-worker marker.

## Notes for the reviewer

- The open start count now comes from open's own stderr line, not from the fake program having run. A program that open said it started but that never ran would now count. That is what the brief asks for; the fake program's own marker is still written and the fake worker goroutine waits on it.
- `awaitEnd` after the unlock is bounded by the same 10 s as the first wait. The base had no bound there (a bare `c.Wait()`); the select's other case is the child's exit.

## NOT VERIFIED on a live screen

Nothing here is a UI row; all proof is `go test` and `make`.

## Verdict

DONE
