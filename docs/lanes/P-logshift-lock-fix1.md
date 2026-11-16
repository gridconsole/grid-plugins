# P-logshift-lock-fix1

Fix round for P-logshift-lock, ejected from pint207 on `TestFilesStayWithinFilesTimesLimitPlusOneLine`.

## Failure (after `git merge origin/main`, before the fix)

```
go test ./internal/hookevents/ -run TestFilesStayWithinFilesTimesLimitPlusOneLine -count=1 -v   (exit 1)
=== RUN   TestFilesStayWithinFilesTimesLimitPlusOneLine
    hookevents_test.go:135: after line 51287 there are 4 files, more than Files=3
--- FAIL: TestFilesStayWithinFilesTimesLimitPlusOneLine (2.33s)
```

## Which side is right

The lane. `Shift` takes its flock on `<path>.lock` (brief row A1), so `.hook-counts/` holds `events.jsonl`, `.1`, `.2` and `events.jsonl.lock`. The test counts every directory entry as a log copy. The lock file holds no log lines and `logshift.Names` does not list it, so `hookevents.Read` is unaffected. The guard the test stands for (no more than `Files` log copies, none above `Limit` plus one line, all together under `Files x (Limit + one line)`) is still true. Only the count of names was wrong.

Fix, in `internal/hookevents/hookevents_test.go` (`fc4eae27`): the name count skips `events.jsonl.lock`. The size checks are unchanged and still add the lock file's bytes (0). The test is not weakened, skipped or deleted.

## Proof table

| Row | Result |
|---|---|
| 1 `go test ./internal/hookevents/ -run TestFilesStayWithinFilesTimesLimitPlusOneLine -count=1 -v` | HOLDS, exit 0: `--- PASS: TestFilesStayWithinFilesTimesLimitPlusOneLine (7.38s)`; with `-race` over the whole package (`-v`): `TestLimitKeepsThreeHoursAtBusyRate`, `TestReadReturnsShiftedCopiesInTimeOrder` and the target test all PASS, `ok .../internal/hookevents 27.698s` |
| 2 A1 `Shift` takes an exclusive flock on `<path>.lock`, re-checks the size under it | HOLDS: `internal/logshift/logshift.go:29` (open lock), `:34` (`TryLockExclusive`), `:49-52` (`shiftLocked` re-check); `TestShiftLockedReadsTheSizeAgain` PASS on the merged head |
| 2 A2 8 writers, no copy lost, none under Limit, `-race -count=20` | HOLDS: `go test -race -count=20 ./internal/logshift/...` -> `ok .../internal/logshift 4.110s`, exit 0; `TestShiftEightWritersAtTheLimitLoseNoCopy` and `TestShiftEightWritersAppendingPastTheLimitLoseNoLine` PASS. Base FAIL is pasted in `docs/lanes/P-logshift-lock.md` and is unchanged by this round (no code under `internal/logshift` changed) |
| 2 A3 `Shift` skips when it cannot take the lock | HOLDS: `TestShiftSkipsWhileAnotherProcessHoldsTheLock` and `TestShiftSkipsWhenTheLockFileCannotBeOpened` PASS (`logshift.go:31,35` bare returns) |
| 2 A4 grid open's app log keeps working | HOLDS: `go test -count=1 -v ./internal/cli -run 'TestSg\|TestOpen'` exit 0: `TestSgRotatesAtStart`, `TestSgKeepsSeven`, `TestSgSmallLogAppends`, `TestSgRunningProgramNotRotated`, `TestSgFailedShiftStillStarts` and all `TestOpen*` PASS |
| 2 `go test -race ./internal/logshift/... ./internal/hookevents/... ./internal/cli/...` | logshift ok 1.957s, hookevents ok 68.274s (exit 0 each). `internal/cli` under `-race` hit Go's default 10 minute test timeout in the first run (`panic: test timed out after 10m0s`, running `TestTfFoundShowRecord`, a feed/orchestrator test; the lane changed nothing there; the lane's own run took 342 s on a quieter machine). Rerun `go test -race -count=1 -timeout 40m ./internal/cli/...`: exit 0, `ok .../internal/cli 329.283s` |
| Packages that import logshift or hookevents (`go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...`: `logshift` is imported by `hookevents` and `cli`; `hookevents` by `hook` and `cli`; `cli` by `cmd/grid` and `lib`; `hook` by `packaging`; `packaging` by `packaging/cmd`) | HOLDS: `go test -race -count=1 ./internal/hook/... ./internal/packaging/... ./lib/... ./cmd/...` exit 0: hook ok 22.383s, packaging ok 73.868s, packaging/cmd ok 1.297s, lib ok 34.249s, cmd/grid no test files; `internal/cli` in the row above |
| 3 walk tests | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` -> `ok .../internal/cli 0.158s`; `go test -count=1 ./internal/job/lint/` -> `ok .../internal/job/lint 0.378s`. The app-repo walk tests (`make webui-test` and the rest) are not run, not asked: this is the plugin repo |
| 4 `make lint` | HOLDS, exit 0 (`go vet ./...`, no output) |
| 4 `make cross` | HOLDS, exit 0: darwin/arm64 and windows/amd64, vet and test compile, every package `ok` |
| `git fetch origin && git merge origin/main` before the last proof run | HOLDS: `Already up to date.` (the first merge, at the start, brought in `P-hookevents-keep-3h` and `-2`) |
| `jstest/*.test.mjs` | not run, not asked: the lane changed no page |
| `*RealPlugin*`, whole-module `go test`, cross-repo set | not run, not asked: no row names them; the orchestrator runs them at landing |
| New guard, gate, cap, hold or blocking hook on agents | none |
| Touches | `internal/hookevents/hookevents_test.go` (allowed by the brief), `docs/lanes/P-logshift-lock.md` (a note), this file |

## What changed

- `internal/hookevents/hookevents_test.go`: the file count in `TestFilesStayWithinFilesTimesLimitPlusOneLine` leaves out `events.jsonl.lock`.
- `docs/lanes/P-logshift-lock.md`: a "Fix round" section, so its stale `hookevents [no test files]` row has a pointer.

## NOT VERIFIED on a live screen

No screen here. Two real `grid hook` processes on a real Grid home were not run.
