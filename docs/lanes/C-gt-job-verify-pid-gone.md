# C-gt-job-verify-pid-gone

| Row | Result |
|---|---|
| 1. run both tests -v | HOLDS, exit 0: `--- PASS: TestVerifyCancelKillsChild (0.03s)`, `--- PASS: TestVerifyExitedShellKillsDescendants (0.27s)`; `-count=10`: `ok ... 3.067s` |
| 2. `git grep -n -E 'time\.(Since\|Sleep)' -- internal/job/verify_test.go` | HOLDS: no output |
| 3. break: kill `cmd.Process.Pid` at both lines in verify_process_unix.go | HOLDS: both FAIL, exit 1: `verify_test.go:218: child survived cancellation` / `--- FAIL: TestVerifyCancelKillsChild (30.02s)`; `verify_test.go:235: descendant survived its shell` / `--- FAIL: TestVerifyExitedShellKillsDescendants (30.01s)`. Restored with `git checkout -- internal/job/verify_process_unix.go`; `git status --short` showed only verify_test.go modified. Note: with the break, the pid-gone wait ran until the 30 s `sleep` ended on its own (the pid then went), so the failing line was the `survived` assertion, not "descendant still alive"; the wait bound (30 s) equals the sleep, so which message shows is a tie. Either way the test fails. |
| 4. `go test -count=1 ./internal/job/` | HOLDS, ok. `gofmt -l internal/job` empty; `go vet ./internal/job/` exit 0 |
| Importers tests | HOLDS, ok: internal/job/..., hook, jobgraph, lease, migrate, orchestrator, ports, cli (direct importers from go list) |
| Windows/macOS | HOLDS: `GOOS=windows go vet ./internal/job/` and `GOOS=darwin go vet ./internal/job/` exit 0 (test uses os.FindProcess + Signal(syscall.Signal(0)), no syscall.Kill) |
| Lint | HOLDS: gofmt and go vet clean; make lint not run |
| Diff scope | HOLDS: only internal/job/verify_test.go (plus this report); helpers `gtVerifyReadPid`, `gtVerifyWaitPidGone` |
| No clock / no guard | HOLDS: no Sleep; 30 s bounds only name the missing event; nothing added to agents |

## What changed
Both tests use `sleep 30` and write `$!` to bg.pid. Cancel test cancels only when ready exists and bg.pid holds a full line; bounds raised to 30 s. After verify returns, both wait until the pid is gone (failure text "descendant still alive"), then assert `survived` is absent. Sleeps dropped.

## NOT VERIFIED
Nothing needs a live screen. Not run: the whole-module test; make lint.
