# C-ko-task-tick-flake-events

Repo: plugin. Branch lane-C-ko-task-tick-flake-events. Base: main 21f02736 (the brief names dc301fce; main has moved, and every line the brief quotes is the same on 21f02736: approval_read_test.go:112-114 `tick`, :91 and :99 the calls, :291 the post with no id, flake_close_reopen_test.go:41-44 the gap sleep, :139-141 the 2s call, write_test.go:176 the gap-0 call).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. no `time.Sleep` in the two files | HOLDS | `git grep -q 'time\.Sleep' -- internal/task/approval_read_test.go internal/task/flake_close_reopen_test.go` exits 1, prints nothing. On main the same grep prints `approval_read_test.go:114` and `flake_close_reopen_test.go:43`. |
| 2. TestApprovalRead*, TestFlakeCR* pass | HOLDS | `go test -count=1 -v -run 'TestApprovalRead\|TestFlakeCR\|TestConcurrentCloseReopen' ./internal/task/` exit 0: `--- PASS: TestApprovalReadLatestResultWins (0.03s)`, `--- PASS: TestApprovalReadReopenVoidsOlderResults (0.14s)`, `--- PASS: TestFlakeCRLateClose (0.20s)`; no FAIL, no SKIP. |
| 3. TestConcurrentCloseReopen passes | HOLDS | same run: `--- PASS: TestConcurrentCloseReopen (0.41s)`. Its call is `flakeCRRun(t, []string{"close","reopen"}, false)` (write_test.go:176): both children start at once, as before. |
| 4. `go test -count=3 ./internal/task/`, diff names only Touches | HOLDS | exit 0, `ok .../internal/task 22.241s`. `git diff main --stat` (before this report): approval_read_test.go, flake_close_reopen_test.go, write_test.go (the one call). This report file is the only other file, as the runbook requires. |
| 5. walk tests | HOLDS | after `git merge main` ("Already up to date"): `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0, `ok .../internal/cli 0.193s`; `go test -count=1 ./internal/job/lint/` exit 0, `ok .../internal/job/lint 1.415s` (run in the importer set below, same flags). |
| 6. `make lint`, `make cross` | HOLDS | `make lint` (go vet ./...) exit 0. `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package ok. |
| Importers of internal/task | HOLDS | Direct importers by `go list` imports: braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, land, signal, updates. `go test -count=1 ./internal/braincmd/ ./internal/guard/ ./internal/hook/ ./internal/job/... ./internal/land/ ./internal/signal/ ./internal/updates/` exit 0 (jobtest and updates have no test files); cli ran its walk tests (row 5). Only `_test.go` files changed, so no importer sees a different package. |
| gofmt | HOLDS | `gofmt -l internal/task` prints nothing. |
| New test fails on base | NOT RUN | The only new test is `TestKoTfIDIsStrictlyIncreasing` (5000 calls of the id helper, each a valid record id and greater than the one before; `--- PASS` in 0.01s). It calls `koTfID`, which main does not define, so on main the task test binary does not build; I did not run that. The brief asks for no behaviour test; this lane changes tests only. |
| No production file, no guard on agents | HOLDS | `git diff main --stat` shows three `_test.go` files. |
| Whole-module test, cross-repo set | not mine | the orchestrator runs them at landing. |
| Webui/app walk tests | n/a | plugin repo. |

## What changed

- approval_read_test.go: `tick()` and its two calls are gone. New `koTfID()` hands out ULIDs under a mutex: the millisecond part starts at a fixed instant (2026-11-11T10:00:00Z, no clock read) and grows by one per call; the 80 low bits are fixed. Every post in the file passes one as `feed.Input.ID`: the handover in `approvalFeed`, `postJobRecord`, `acceptControl`, and the control posted in `TestApprovalReadNotAPlanApproval` (was :291). `Store.record` takes `in.ID` as is (internal/feed/store.go:488), so the later post always has the greater id, which `planJobResult` and `TestApprovalReadLatestResultWins` rely on. No test in the file posts through a path that cannot take an id. `acceptControl` returns `store.Post(...)` directly.
- flake_close_reopen_test.go: `flakeCRRun(t, actions, gap time.Duration)` is now `flakeCRRun(t, actions, waitRefused bool)`. The child prints the line `koTf refused` (const `koTfRefused`) the first time a move is refused for the current state. The parent reads every child's stdout through `StdoutPipe` and a reader goroutine (`koTfRead`) that keeps all of it for the receipt count (still only lines starting with `{`) and closes a channel at the first `koTf refused` line. With `waitRefused`, the parent starts the next child only after `<-prev.refused`; the other case of that select is `<-prev.done`, which fails the test (a first child cannot end before the second has run). No timer in the parent: `flakeCRDeadline` stays as the child's failure bound, and a child that hits it exits, which closes `done`. The parent waits for `done` before `cmd.Wait`, as `StdoutPipe` requires, and `t.Cleanup` kills each child so a failed run leaves none running. `TestFlakeCRLateClose` passes `true`; its comment says what the late start is now.
- write_test.go:176: the call passes `false` instead of `0`.

## NOT VERIFIED

- Nothing needs a live screen. Not run: `-race`, and the new test on base (see the table).
- The late close child now starts right after the reopen child's first refusal, not two seconds later: the stressor is the start order (reopen has been running and refused, close not yet started), not a measured delay. That is what the brief asked for.

Verdict: DONE
