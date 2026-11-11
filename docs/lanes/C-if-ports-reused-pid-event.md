# C-if-ports-reused-pid-event

## Proof table

| Row | Result |
|---|---|
| 1. no `time.After(50 * time.Millisecond)` in ports_test.go | HOLDS: `git grep -n 'time.After(50 \* time.Millisecond)' -- internal/ports/ports_test.go` prints nothing (exit 1 = no match). |
| 2. scratch: StopRecord's `matchesProcess` always true | HOLDS: `go test ./internal/ports -run '^TestReusedPIDIsNotSignalled$' -count=1` exit 1: `ports_test.go:397: signalled reused PID` (the StopRecord check; the ladder reached SIGKILL). Reverted by `git checkout -- internal/ports/process_unix.go`. |
| 3. scratch: GC stop sends SIGTERM without identity check | HOLDS, but only with a different edit than the brief names (see below). Failing run: `ports_test.go:403: gc signalled reused PID: TERM`, exit 1. Reverted by `git checkout -- internal/ports/ports.go`. |
| 4. `-count=20` exits 0 | HOLDS: `go test ./internal/ports -run '^TestReusedPIDIsNotSignalled$' -count=20` exit 0. Also `-count=100 -race` exit 0. |
| 5a. `go test ./internal/ports -count=1` | HOLDS: exit 0. |
| 5b. importers of ports (`go list`: internal/env, internal/cli, and through cli cmd/grid and lib) | HOLDS: `go test ./internal/env ./internal/cli ./cmd/grid ./lib -count=1` exit 0 (cmd/grid has no test files). |
| 5c. `make lint` | HOLDS: exit 0 (`go vet ./...`). |
| 5d. `GOOS=windows go vet ./internal/ports/` | HOLDS: exit 0. Also `GOOS=darwin go vet ./internal/ports/` exit 0, `GOOS=darwin CGO_ENABLED=0 go build ./...` and `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| 5e. diff only touches the Touches files | HOLDS: only `internal/ports/ports_test.go`, new `internal/ports/if_signal_child_test.go`, and this report. No production file; `git status --short` was clean of process_unix.go and ports.go before the commit. |
| Rule: new tests fail on base, pass on lane | HOLDS by rows 2 and 3: the changed test fails on the two production defects and passes on the lane. |
| Rule: no sleep, elapsed-time assertion or timeout as proof | HOLDS: the only timer is `ifPrDeadline` (10 s) in `if_signal_child_test.go`, which bounds a wait and fails by name ("child never answered: ..."). |
| Rule: no test deleted or assertion weakened | HOLDS: the `Alive` check at ports_test.go:397 and the "gc" removed-count check stay; the 50 ms `select` is replaced by `assertNoSignal`, which proves more. |
| Rule: no guard, gate, cap or hold on agents | HOLDS: test files only. |
| Rule: new package-level names start with `ifPr` | HOLDS: `ifPrDeadline`, `ifPrChild`, `ifPrScript`, `ifPrStartChild`. |

## Row 3: the brief's edit cannot fail this test

In `gc` (ports.go:423-433) a `Group` record is checked by `matches(r)` before `stop` is ever called. With `PIDStart` mismatched, `ProcessMatches` returns false, so `alive` is false, the record is reclaimed and `stop` is not called at all. Passing an unchecked SIGTERM stop function to `s.gc(ProcessMatches, ...)` therefore leaves the test green (I ran it: `ok`). Forcing `matches` true as well makes `gc` return `[]` because the lease is not expired (`ports_test.go:401: gc: [] <nil>`), so `stop` is again not reached.

The scratch edit that does show the barrier catching a TERM was in `gc`, ports.go:433: `if reclaim && r.Group { _ = stop(r, s.grace()) }`, with GC's stop function being `syscall.Kill(-r.PID, syscall.SIGTERM)` (and a temporary `syscall` import). Result: `ports_test.go:403: gc signalled reused PID: TERM`. Reverted.

## What changed

- `internal/ports/if_signal_child_test.go` (new): `ifPrStartChild` starts `sh -c` with Setpgid. The shell traps INT, TERM and WINCH, writes one line per trap, prints `ready` first, and blocks in `read` on an open stdin pipe, so traps run immediately. Stdout is an `os.Pipe`, so `cmd.Wait` cannot drop buffered lines. `assertNoSignal` sends SIGWINCH, then reads lines until `WINCH`. An `INT` or `TERM` line, or EOF from SIGKILL, fails with `gc signalled reused PID: <line>`. No answer within the deadline fails with "child never answered the barrier signal".
- `internal/ports/ports_test.go`: `TestReusedPIDIsNotSignalled` uses that child instead of `sleep 60`; cleanup moved into `t.Cleanup`.

## NOT VERIFIED

- macOS: only vet and build were run (`GOOS=darwin`); the test ran on Linux only. The trap order relies on pending signals being delivered lowest number first, which I did not check on macOS.

DONE
