# C-kr2-hook-history-lock-wait

Lane branch `lane-C-kr2-hook-history-lock-wait`, repo plugin, from main 21f02736.
Origin: plugin check 2026-11-11T1858Z, issue 1 (PreToolUse hook timed out 7 times).

Verdict: DONE. Every row below HOLDS.

## What changed

- `internal/session/files.go:106` `lockedContext(ctx, fn)`: `locked` with the wait on `<task>/sessions/.lock` bounded by ctx, through `fslock.LockExclusiveContext` (files.go:115). `locked` itself is untouched.
- `internal/session/activity.go:81` `UpdateActivityContext(ctx, id, change)`: the old body, on `lockedContext`. `UpdateActivity` (activity.go:73) calls it with `context.Background()`.
- `internal/hook/history.go:111-112, 165`: the Bash history record (Pre, Post, PostFailure) waits at most `historyLockWait` = 2 s; on `context.DeadlineExceeded` it logs `history skipped: session lock busy after 2s` through `o.log` and returns. No activity and no snapshot are written. Guards run before and are untouched; a deny never reaches `history`.
- `internal/hook/hook.go:56, 68`: `Run` takes the start time and, when the call took 2 s or more, logs `hook <event> slow: <ms> ms` through `o.log` after the output is written. Nothing times out; the guards run to the end.
- Two new test files, helpers `kr2*`, tests `TestKr2*`.

A PostToolUse after a skipped PreToolUse finds no snapshot: `history.go:122` (`if !ok { return nil }`, unchanged from main :109-110) returns without an error.

## Proof table

| # | Row | Result |
|---|-----|--------|
| base | `git grep -n 'fslock.LockExclusive(f)' -- internal/session/files.go` on main prints `files.go:94`; `locked` had no context or time bound | HOLDS, exit 0 |
| base | `git merge-tree --write-tree --name-only origin/main origin/lane-C-kr-planguard-off-hint` | HOLDS: prints one tree sha and no file name, no conflict |
| 1 | Red on base: `go test -count=1 -v -run TestKr2 ./internal/session/ ./internal/hook/` with both test files and main's code | HOLDS. Session package: compile failure, not a test failure: `kr2_lock_wait_test.go:107:12: s.UpdateActivityContext undefined (type Store has no field or method UpdateActivityContext)` (also :149, :169), `FAIL github.com/gridconsole/grid-plugins/internal/session [build failed]`. Hook package: `--- FAIL: TestKr2HistorySkipsWhenSessionLockBusy (5.02s)`, `--- FAIL: TestKr2PostAfterSkippedPreWritesNothing (5.00s)`, `--- FAIL: TestKr2SlowHookLogsOneLine (5.00s)`, each with `PreToolUse did not return while another descriptor held the session lock: the history record waits for it without a bound`. `TestKr2HistoryRecordsWhenSessionLockFree` and `TestKr2FastHookLogsNoSlowLine` pass on base, as they should (the free-lock behaviour is the same as on main). The tests are in commit 18880676, before the code in 81a4c1b3 |
| 2a | Lock held: PreToolUse returns in 2 s up to 4 s, exactly one `history skipped: session lock busy after 2s` line, activity file byte for byte, activity folder listing and history.md unchanged, same decision output as with the lock free | HOLDS: `--- PASS: TestKr2HistorySkipsWhenSessionLockBusy (2.05s)`, same command, exit 0 |
| 2b | Lock free: record written as on main, same JSON as UpdateActivity writes (compared byte for byte with the JSON of the expected Activity, snapshot equal to `snapshot(dir)`), Post writes the file line and removes the snapshot | HOLDS: `--- PASS: TestKr2HistoryRecordsWhenSessionLockFree (0.02s)` |
| 2c | PostToolUse after a skipped PreToolUse: no log, exit 0, no history line, no snapshot | HOLDS: `--- PASS: TestKr2PostAfterSkippedPreWritesNothing (2.00s)` |
| 3 | Lock released when the waiter gives up: hold, `UpdateActivityContext` with 100 ms ctx returns `context.DeadlineExceeded` and the change did not run, `fslock.OnBlockForTest` event, release holder, `fslock.OnAbandonedWaitForTest` event, then `UpdateActivity` succeeds within a bound derived from `t.Deadline()`. Channels and events only, no `time.Sleep` | HOLDS: `--- PASS: TestKr2UpdateActivityContextGivesUpAndReleasesTheLock (0.11s)`. Also passing: `...FreeLockWritesLikeUpdateActivity`, `...DoneBeforeTheCallWritesNothing`, `TestKr2UpdateActivityStillWaitsWithoutBound` (the unbounded form still waits) |
| 4 | Slow-hook line | HOLDS with a held lock: `--- PASS: TestKr2SlowHookLogsOneLine (2.00s)` (one `hook PreToolUse slow: <ms> ms` line, ms >= 2000 and not above the measured call); `--- PASS: TestKr2FastHookLogsNoSlowLine (0.00s)` (Pre and Post on a free lock, empty log). No injected clock |
| 5 | `go test -count=1 ./internal/session/ ./internal/hook/ ./internal/fslock/` | HOLDS, exit 0: session ok 7.158s, hook ok 12.891s, fslock ok 0.415s |
| 5 | `git grep -n 's.locked(' -- internal/session/` | HOLDS: lists `session.go:330` and `session.go:514`, both the unbounded form |
| 6 | `go test -count=1 -race -run 'TestKr2\|TestJwLc' ./internal/session/ ./internal/hook/ ./internal/fslock/` | HOLDS, exit 0 (session 1.298s, hook 7.295s, fslock 1.394s) |
| 6 | the same with `-count=3` | HOLDS, exit 0 (session 1.525s, hook 20.277s, fslock 1.568s) |
| 7 | `make lint` | HOLDS, exit 0 (`go vet ./...`) |
| 7 | `make cross` | HOLDS, exit 0: `cross: darwin/arm64 vet`, `darwin/arm64 test compile`, `windows/amd64 vet`, `windows/amd64 test compile`, all packages ok |
| 7 | plugin walk tests: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v` and `go test -count=1 ./internal/job/lint/` | HOLDS: `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, `ok internal/cli`; `ok internal/job/lint 0.754s`, exit 0. Run after the last Go change |
| 7 | `git diff main --stat` names only the files in Touches | HOLDS: six files, all in Touches: history.go, hook.go, kr2_history_wait_test.go, activity.go, files.go, kr2_lock_wait_test.go. This report is the seventh file, required by the runbook |
| rule | Packages that import session or hook, tested | HOLDS, exit 0. The `go list` output (package <- dependency): cmd/grid, internal/braincmd, internal/cli, internal/env, internal/guard, internal/job, internal/job/jobcmd, internal/job/jobtest, internal/job/lint, internal/jobgraph, internal/land, internal/lease, internal/mcp, internal/migrate, internal/orchestrator, internal/packaging, internal/packaging/cmd, internal/ports, internal/signal, internal/task, internal/updates and lib, each `<-` internal/session and/or internal/hook (cmd/grid, cli, packaging, packaging/cmd and lib import both). `go test -count=1` on all of them plus session, hook and fslock: every package `ok`; cmd/grid, jobtest and updates have no test files |
| rule | New tests fail on base and pass on the lane | HOLDS, see row 1 and rows 2 to 4 |
| rule | No test edited, none deleted | HOLDS: `git diff main --stat` shows no deletion of a test file; the 7 deleted lines are in history.go, hook.go and activity.go |
| rule | No guard, gate, cap, hold or blocking hook on agents | HOLDS: the lane bounds the hook's own bookkeeping wait and fails open; no tool call is blocked, no agent limited, the guards run to the end and a deny is decided before `history` |
| rule | Small commits, one-sentence messages, no attribution lines, no personal paths or emails | HOLDS: three commits (tests, code, this report); bodies checked with `git log --format=%B`; no path outside the repo, no host, no email in any file |
| rule | Remove nothing | HOLDS |
| rule | RealPlugin tests, UI changes | Not applicable: no `*RealPlugin*` test named and no UI row |

## Notes for the backlog

- `TestKr2FastHookLogsNoSlowLine` skips when a call on a free lock itself takes 2 s or more (a box at load 200 to 780). On such a run row 4 is proven by the slow test only; the skip is honest, not a pass.
- The slow line is new log output for any hook of 2 s or more. A test that asserts an empty hook log while it holds a lock (for example the 500 ms lock-order test) could go red on a heavily loaded box. It passed here in the full package run.
- The skipped record leaves a PreToolUse snapshot out of the activity file; a Post that is skipped leaves its snapshot in it. That is the same as a Post that never arrives on main.
- This lane does not settle whether the 20 to 35 s hooks were the lock queue or a stalled Claude Code process. The skip line and the slow line are what tell them apart next time: a slow line with no skip line means the time was spent outside the lock wait.

## NOT VERIFIED on a live screen

- The macOS and Windows lock paths are compile-checked by `make cross` (vet and test compile) only. `LockExclusiveContext` and the new tests ran on Linux.
- No real Claude Code session ran; the 10 s hook limit and a box at load 200 to 780 were not reproduced.
