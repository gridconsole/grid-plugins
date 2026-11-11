# C-if-eval-test-deadlines

The eval runner reads the child's pipes itself and drains them after the group is gone, instead of leaning on a 250 ms WaitDelay; the timeout case of TestFailuresAreSaved waits on the agent having started.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. TestIfEdBackgroundChild: Status passed, exit 0, PID line and ok, Error empty, background child gone (FIFO reads EOF) | HOLDS | `go test ./internal/eval -run 'TestSetupBuildsGridHomeAtGridHome\|TestFailuresAreSaved\|TestIfEd\|TestCommandNotFoundAndCancellation' -count=1 -v` printed `--- PASS: TestIfEdBackgroundChild` and `ok`; test at `internal/eval/if_process_drain_test.go` (`TestIfEdBackgroundChild`). On main (scratch clone of main, lane test files copied in): `if_process_drain_test.go:105: a command that exited 0 was not passed: status "error", exit 0, error "exec: WaitDelay expired before I/O complete", stderr ""`. |
| 2. TestIfEdFullOutput: 64 KiB in, exactly 64 KiB out, Truncated false; fails if the drain stops early | HOLDS | Passes on the lane (same run as row 1). Scratch edit (`return` after the first read in `drain`): `if_process_drain_test.go:135: stdout holds 32768 of 65536 bytes (truncated false)`; reverted with `git checkout -- internal/eval/process.go`. |
| 3. TestIfEdContextEnds: setsid descendant holds stdout; returns on cancel, Error "context canceled", not before | HOLDS, with a limit | Passes on the lane. It also passes on main: the brief's check is a non-blocking select at the instant the first line is read, and main only returns 250 ms later through WaitDelay, so no event can tell them apart without a timer. The test is not vacuous: scratch edit (no cut-off when ctx ends, `<-drained` only) gives `if_process_drain_test.go:175: runProcess did not return within 20s after the context was cancelled`; reverted. The descendant is the test binary run again (`TestIfEdHelperProcess`, chosen by `GRID_EVAL_IFED_HELPER`) with `SysProcAttr{Setsid: true}`; it reads a FIFO the test holds open and exits when the test closes it (t.Cleanup). |
| 4. Timeout case passes `-run '^TestFailuresAreSaved$' -count=20`; fails by name if the runner ignores ctx | HOLDS | `go test ./internal/eval -run '^TestFailuresAreSaved$' -count=20` exit 0 (`ok ... 1.756s`). Scratch edit (`exec.Command` instead of `CommandContext`, `cmd.Cancel = nil`): `--- FAIL: TestFailuresAreSaved/timeout` / `if_process_drain_test.go:250: Runner.Run did not return within 20s after the parent context ended with DeadlineExceeded`; reverted. The agent writes `started` to a FIFO (`ifEdTimeout`), the test reads it and ends a parent context of its own (`ifEdCtx`: Done closes, Err is DeadlineExceeded); Runner.Timeout stays at testRunner's 3 s. All assertions of the old case are kept (Status error, Skipped == len(Checks), CheckScore nil, ExitCode != 0, Error contains "deadline"). Second case `TestIfEdRunnerTimerEndsTask` (Timeout 1 ns): status error, Setup.Error contains "deadline", agent skipped, record saved and loaded (`checkSavedRun` calls LoadResult). It passes on main too: it pins behaviour the lane keeps. |
| 5. The named run and `go test ./internal/eval -count=1` exit 0 | HOLDS | First command: all listed tests PASS (row 1 run). `go test ./internal/eval -count=1`: `ok ... 64.626s`. |
| 6. `make lint`, Windows / macOS / FreeBSD vet, diff names only the Touches files | HOLDS | `make lint` (runs `go vet ./...`) exit 0. `GOOS=windows go vet ./internal/eval/`, `GOOS=darwin go vet ./internal/eval/`, `GOOS=freebsd go vet ./internal/eval/` exit 0. `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0. `git diff main --stat`: home_test.go, if_process_drain_test.go, process.go, process_unix.go, process_windows.go, runner_test.go (plus this file). |

## Rules

| Rule | Result | Proof |
|---|---|---|
| Tests of changed package and of every package that imports it | HOLDS | `go list` (Deps) shows importers cmd/grid, internal/cli, internal/evalcmd, internal/land, lib. `go test ./internal/eval ./internal/evalcmd ./internal/land ./internal/cli ./cmd/grid ./lib -count=1`: eval ok 64.6s, evalcmd ok, land ok, cli ok 122.7s, lib ok 160.9s, cmd/grid has no test files. |
| New tests fail on base | PARTLY | Row 1 fails on main (line above). Rows 2 and 3 and the second row-4 case pass on main by design (see their lines); each is shown to fail by a scratch edit instead. The timeout case is a test fix: on main it passes, and it fails by name when the runner ignores ctx. |
| No existing test edited to pass; none deleted | HOLDS | The timeout case of TestFailuresAreSaved was moved, not deleted: its body is `ifEdTimeout` in `if_process_drain_test.go` (build tag `linux \|\| darwin`, because it needs `syscall.Mkfifo`), registered as the subtest `TestFailuresAreSaved/timeout` through `ifEdTimeoutCase` (`runner_test.go`). On Windows that subtest is absent. `runTask` now calls the new helper `checkSavedRun` with the same checks. `home_test.go`: only the failure line of the setup check changed (adds error and stderr). |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | None added. The output limit is the existing `outputLimit`. |
| No sleep / elapsed-time assertion / timeout as proof in a test | HOLDS | Every wait is on an event (first line, FIFO EOF, Done); `ifEdWait` (20 s) only bounds a wait and fails by name. |
| `*RealPlugin*` tests | N/A | None named by the brief. |
| No attribution lines, no personal paths, hosts or emails | HOLDS | Two commits, one sentence each, no trailers. |

## What changed

- `internal/eval/process.go`: `runProcess` makes three `os.Pipe`s it owns and passes the child's ends as `*os.File`, so exec starts no copy goroutines. Reader goroutines drain stdout (with the tee) and stderr into the bounded buffers; a writer goroutine feeds stdin. After `proctree.Run` and `cleanupProcess` (the group is killed, or on Windows the job ends inside `proctree.Run`) the runner closes its copies of the child's ends, ends the stdin write, and reads to EOF. If ctx ends first, the read ends are closed then; no timer. Output the drain had not read yet at that moment is dropped.
- `cmd.WaitDelay` is removed. Nothing else needs it here: with `*os.File` std streams Wait has no goroutine to give up on, and its other job (forcing `Process.Kill` after Cancel) is covered, since Cancel is `kill(-pid, SIGKILL)` on Unix, `killProcessTree` with a `Process.Kill` fallback on Windows, and the default `Process.Kill` elsewhere. The other packages' own WaitDelay settings are untouched.
- `process_unix.go`, `process_windows.go`: comments only (the order: kill the group before reading to EOF). `process_other.go` unchanged: `configureProcess` and `cleanupProcess` keep their signatures, and no new per-platform function was added.
- Tests: `if_process_drain_test.go` (new), `runner_test.go` (timeout row moved out, `checkSavedRun` split from `runTask`), `home_test.go:84` (failure line prints Error and Stderr).

## Behaviour to know

- A descendant that left the group and holds the output keeps `RunProcess` waiting until ctx ends. With a context that never ends (`context.Background()`), that is for ever; before, the 250 ms margin cut it off and lost output.
- On platforms with no group kill (`process_other.go`, e.g. FreeBSD), a background child of the command now holds the output until ctx ends instead of 250 ms. Linux, macOS and Windows kill the group or job first.
- A run that passed with a background child in its own group is now `passed` with its complete output, not `error` with `exec: WaitDelay expired before I/O complete`.

## NOT VERIFIED on a live screen / platform

- Windows: the new tests are tagged `linux || darwin`, so nothing new ran on Windows; `GOOS=windows go vet` and `go build` pass, and the existing `TestWindows*` tests in the package were not run. The Windows path relies on `os.File.Close` on an `os.Pipe` read end ending a read that is blocked (needed only for the ctx cut-off) and on `proctree.Run` ending the job before the drain; neither was run on Windows.
- macOS: the FIFO tests (`O_RDWR` open of a FIFO, kqueue on a FIFO) were not run on darwin; `GOOS=darwin go vet` and `go build` pass.
- The original failure (load 278) was not reproduced; the proof is the event-based test and the line from main.
