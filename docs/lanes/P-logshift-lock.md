# P-logshift-lock

Rotating a log from two processes at once loses no copy. Base: `origin/main` (`git merge origin/main` said "Already up to date" on the last run). Code commit `e842380f` touches only `internal/logshift/logshift.go` and `internal/logshift/logshift_test.go`; this file is the runbook's proof table, in its own commit.

## Why

On grid3, `.hook-counts/events.jsonl.1` held 2 lines and `.2` held 3,010. `Shift` took no lock: two `grid hook` processes both passed the size check, and the second removed the full copy the first had just shifted (caller: `internal/hookevents/hookevents.go:77`).

## Base FAIL (new tests on the unmodified `logshift.go`, `go test -race -count=1 ./internal/logshift/`, exit 1)

```
--- FAIL: TestShiftEightWritersAtTheLimitLoseNoCopy (0.01s)
    logshift_test.go:144: events.jsonl.1 holds 448 bytes: a shifted copy holds at least the limit, 2048
    logshift_test.go:144: 64 lines are gone, first [old-000 old-001 old-002 old-003]
    logshift_test.go:146: round 5 of 60
--- FAIL: TestShiftEightWritersAppendingPastTheLimitLoseNoLine (0.02s)
    logshift_test.go:178: events.jsonl.1 holds 1152 bytes: a shifted copy holds at least the limit, 2048
    logshift_test.go:178: events.jsonl.2 holds 192 bytes: a shifted copy holds at least the limit, 2048
    ...
    logshift_test.go:178: 6 lines are gone, first [w4-000 w4-001 w4-002 w4-003]
--- FAIL: TestShiftSkipsWhileAnotherProcessHoldsTheLock (0.00s)
    logshift_test.go:207: Shift moved the log while another process held the lock
--- FAIL: TestShiftSkipsWhenTheLockFileCannotBeOpened (0.00s)
    logshift_test.go:229: Shift moved the log without holding the lock
FAIL	github.com/gridconsole/grid-plugins/internal/logshift	0.158s
```

`TestShiftLockedReadsTheSizeAgain` calls `shiftLocked`, which the base does not have, so on the base it does not compile; it was added after the base run. With the re-check line removed from `shiftLocked` (a short edit, put back with `git checkout -- internal/logshift/logshift.go`; `git status --short` clean) it fails:

```
--- FAIL: TestShiftLockedReadsTheSizeAgain (0.00s)
    logshift_test.go:195: events.jsonl.1 holds 64 bytes: a shifted copy holds at least the limit, 2048
    logshift_test.go:195: 32 lines are gone, first [old-000 old-001 old-002 old-003]
    logshift_test.go:199: events.jsonl.1 no longer starts with cur-000: <nil>
    logshift_test.go:199: events.jsonl.2 no longer starts with old-000: <nil>
```

## Proof table

| Row | Result |
|---|---|
| A1 `Shift` takes an exclusive flock on `<path>.lock` and checks the size again after taking it; a process that finds the file already shifted does nothing | HOLDS: `internal/logshift/logshift.go:29` opens `<path>.lock` (`fslock.OpenNoFollow`, create, 0600), `:34` `fslock.TryLockExclusive`, `:37` unlock, `:49-52` `shiftLocked` reads the size again before it moves anything. `TestShiftLockedReadsTheSizeAgain` (`logshift_test.go:185`) puts a small log over a full `.1` and an old `.2`, calls `shiftLocked`, and both copies stay (fails with the re-check removed, pasted above). The cheap unlocked size check stays in front (`:26`), so a hook run with a small log opens no lock file |
| A1 choice, said once | `Shift` tries the lock and does not wait for it. A process that finds it held skips: the holder is shifting, and an append during a shift lands in the old file or the new one and stays. A stalled holder cannot hang a hook or `grid open`; a skipped shift is found again on the next call if the log is still full. This adds no guard, gate, cap or hold on an agent: no call ever waits |
| A2 8 goroutines append past Limit at once; no shifted copy lost, none smaller than Limit; fails on main, passes on the lane with `-race -count=20` | HOLDS: `TestShiftEightWritersAtTheLimitLoseNoCopy` (`logshift_test.go:123`, 60 rounds of 8 writers finding a full log at one start gate, `Files = 3` as in `hookevents`) and `TestShiftEightWritersAppendingPastTheLimitLoseNoLine` (`:154`, 8 writers x 40 lines, ten limits, `files` large enough that no copy is dropped on purpose). Both check every line is in exactly one copy, every shifted copy is at least `limit` bytes, and a shifted copy exists. FAIL on base pasted above; lane: `go test -race -count=20 ./internal/logshift/...` -> `ok  github.com/gridconsole/grid-plugins/internal/logshift  3.256s`, exit 0 |
| A3 `Shift` never fails its caller when it cannot take the lock (it skips) | HOLDS: `TestShiftSkipsWhileAnotherProcessHoldsTheLock` (`:207`: a second descriptor holds the flock, `Shift` returns inside 10 s and moves nothing, then shifts once the lock is free) and `TestShiftSkipsWhenTheLockFileCannotBeOpened` (`:243`: `path.lock` is a directory, so the open fails; `Shift` returns and moves nothing). Both FAIL on base (pasted). Every error path of the lock in `Shift` is a bare `return` (`logshift.go:31,35`); `Shift` has no result |
| A4 grid open's app log (`internal/cli/open_start.go:227`) keeps working; its tests pass | HOLDS: `go test -count=1 -v ./internal/cli -run 'TestSg\|TestOpen'` exit 0: `--- PASS` for `TestSgRotatesAtStart`, `TestSgKeepsSeven`, `TestSgSmallLogAppends`, `TestSgRunningProgramNotRotated`, `TestSgFailedShiftStillStarts` and every `TestOpen*`. `open_start.go` is unchanged: its call `logshift.Shift(path, appLogLimit, appLogFiles)` keeps its signature |
| `go test -race ./internal/logshift/... ./internal/hookevents/... ./internal/cli/...` | HOLDS, exit 0: `ok .../internal/logshift 1.499s`; `? .../internal/hookevents [no test files]`; `ok .../internal/cli 342.772s` |
| Packages that import logshift (`go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... \| grep internal/logshift`) | `cmd/grid`, `internal/cli`, `internal/hook`, `internal/hookevents`, `internal/packaging`, `internal/packaging/cmd`, `lib`. `go test -count=1 ./internal/hook/... ./internal/fslock/... ./internal/packaging/... ./lib/... ./cmd/grid/...` exit 0: hook ok 9.172s, fslock ok 0.450s, packaging ok 7.936s, packaging/cmd ok 0.338s, lib ok 20.280s, cmd/grid no test files. `internal/cli` is in the row above |
| `make lint` | HOLDS: `go vet ./...` exit 0, no output |
| Windows build | HOLDS: `GOOS=windows go build ./...` exit 0; `GOOS=windows go vet ./internal/logshift/` exit 0 (the tests compile for Windows) |
| macOS build | HOLDS: `GOOS=darwin go build ./...` exit 0 |
| Walk tests | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` -> `ok .../internal/cli 0.130s`; `go test -count=1 ./internal/job/lint/` -> `ok .../internal/job/lint 0.390s` |
| `git fetch origin && git merge origin/main` before the last proof run | HOLDS: `Already up to date.`; the proof runs above are on `e842380f` plus this file |
| App-repo checks (`make webui-test`, `TestFvArgvCollects`, `TestEveryWiredMount`, guest walk tables, `make realplugin`) | not run, not asked: this is the plugin repo |
| Whole-module `go test`, cross-repo set | not run, not asked: the orchestrator runs them at landing |
| `*RealPlugin*` tests | not run, not asked: no row names one |
| New guard, gate, cap, hold or blocking hook on agents | none: no call waits on the lock, and a failure to lock skips the shift |
| Touches | only `internal/logshift/logshift.go`, `internal/logshift/logshift_test.go` and this file; the other callers (`hookevents.go:77`, `open_start.go:227`) keep their calls as they are |

## What changed

`internal/logshift/logshift.go`: `Shift` keeps its signature. When the log is full it opens `<path>.lock`, tries an exclusive flock (`internal/fslock`, which has Unix and Windows implementations), and calls `shiftLocked`, the old shift body with a second size check in front. The lock file is never removed; it stays beside the log (`events.jsonl.lock` in `.hook-counts`, `grid-server.log.lock` and the worker's in `logs`) and `Names` does not list it, so `hookevents.Read` is unaffected. `open_start_test.go`, `hook_events_test.go` and `ruleruns_test.go` hold no `ReadDir` of those folders (grep), and the A4 run above is green with the lock file in place.

`internal/logshift/logshift_test.go`: five tests, none of them there before; the package had no test file.

## Fix round (P-logshift-lock-fix1)

After `origin/main` landed `internal/hookevents/hookevents_test.go`, `TestFilesStayWithinFilesTimesLimitPlusOneLine` failed on the merged head: `after line 51287 there are 4 files, more than Files=3`. The fourth entry is `events.jsonl.lock`, the lock file this lane adds beside the log; it holds no lines and is not one of the `Files` copies. The test now leaves that one name out of its file count (its bytes still count in the size bound). The row above that says `hookevents [no test files]` is out of date: `hookevents` has tests since that merge. See `docs/lanes/P-logshift-lock-fix1.md`.

## NOT VERIFIED on a live screen

There is no screen here. Not run: two real `grid hook` processes on a real Grid home; the goroutine tests stand in for them, since flock excludes separate opens inside one process.

## Open items

- A shift that is skipped because the lock is held leaves one more append in the full log; with `hookevents` that is a few lines past `Limit`, not a loss.
- On a platform without `fslock` (`fslock_other.go`) `Shift` always skips and the log grows; grid builds for macOS, Linux and Windows only.
