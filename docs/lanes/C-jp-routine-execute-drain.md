# C-jp-routine-execute-drain

A routine run with a Go writer for its output reads it to the end after its group is gone, instead of a 2 s wait delay that records a routine that passed as failed.

Base: plugin main 99c11b7a (the head the lane was cut from). Lane head: see the report hand-off (the commit that adds this file).

## Outcome

`ProcessExecutor.Execute` no longer sets a wait delay. `git grep -n WaitDelay -- internal/routine ':!*_test.go'` prints nothing (exit 1, no match).
A writer that is not an `*os.File` gets a pipe the executor owns (`startRoutineOutput`, internal/routine/execute.go). After `runCommand` returns the group is ended (`cleanupProcess`, moved from the defer to before the drain), the write ends are closed, and the copies are read to EOF; if the run's context ends first the read ends are closed and the copies return. A nil or `*os.File` writer goes to the command exactly as before, so the timer path (no `--json`) starts nothing to wait for. The same writer for stdout and stderr gets one pipe and one copy goroutine, as exec does. No timer was added. The result rule is the default: the child's exit code decides, and a background process that held the output is ended with the group.

## Proof table

| Row / rule | Result | Command and result |
|---|---|---|
| 1 TestJpReBackgroundChild | HOLDS | `go test ./internal/routine -run TestJpRe -count=1 -v` exit 0: `--- PASS: TestJpReBackgroundChild (0.05s)`. On main (scratch clone of the ref repo, head 99c11b7a, test file copied in): `jp_routine_drain_unix_test.go:105: Execute: exec: WaitDelay expired before I/O complete` / `--- FAIL: TestJpReBackgroundChild (2.05s)`. |
| 2 TestJpReFullOutput | HOLDS | Passes on the lane (`--- PASS: TestJpReFullOutput (0.07s)`): 3000 lines (150 KB, over a 64 KiB pipe) through one `*bytes.Buffer` for both writers, two buffers keep their streams apart, an `*os.File` Stdout is the same `cmd.Stdout` through the runCommand seam and a nil Stderr stays nil. It also passes on main: exec's own copy completes there, so the scratch edit is the proof. Scratch edit (copy goroutine reads once, then closes the read end): `jp_routine_drain_unix_test.go:135: one writer: signal: broken pipe` / `--- FAIL: TestJpReFullOutput (0.10s)`. Reverted with `git checkout -- internal/routine/execute.go`. |
| 3 TestJpReContextEnds | HOLDS | Passes on the lane (`--- PASS: TestJpReContextEnds (0.02s)`), no skip: the helper is this test binary run as `-test.run='^TestJpReHelper$' -- jp-re-setsid <marker>`, starting `sleep 600` with Setsid sharing stdout. Scratch edit (ctx branch of the drain removed): `jp_routine_drain_unix_test.go:258: Execute did not return after cancel` / `--- FAIL: TestJpReContextEnds (30.12s)`. Reverted. It also passes on main (the 2 s delay is longer than the test's non-blocking check), so the scratch edit is the proof. |
| 4 existing test, then the two packages | HOLDS | `go test ./internal/routine -run '^(TestProcessEnvironmentAndTimeout)$' -count=1 -v` exit 0: `--- PASS: TestProcessEnvironmentAndTimeout (0.16s)`. `go test ./internal/routine ./internal/cli ./cmd/grid ./lib -count=1` exit 0: `ok internal/routine 4.142s`, `ok internal/cli 83.398s`, `cmd/grid` no test files, `ok lib 98.243s`. Importers of internal/routine from `go list`: internal/cli directly; cmd/grid and lib through internal/cli. |
| 5 lint and cross | HOLDS | `make lint` exit 0 (vet clean, gofmt prints nothing). `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package ok. |
| 6 diff stat | HOLDS, one addition | `git diff main --stat`: execute.go, process_unix.go, process_windows.go, jp_routine_drain_unix_test.go, plus this file docs/lanes/C-jp-routine-execute-drain.md that the runbook requires. execute.go changes the Execute tail (the part the brief names) and appends the helper type and functions after it. |
| New tests fail on base, pass on lane | HOLDS for row 1; rows 2 and 3 by scratch edit | See rows 1 to 3. TestJpReSeamNeverStarts (a replaced runCommand that never starts the command still returns) passes on main as well; it guards the new code and is not a base-failure proof. |
| No edit to an existing test, none deleted | HOLDS | `git diff main --stat` shows no existing test file. |
| No guard, gate, cap, hold or new timer | HOLDS | No timer in execute.go: the drain waits on the copies and on the run's existing context only. The existing `withTimeout` is unchanged. |
| Tests: no sleep, no elapsed-time assertion | HOLDS | Waits are a ticker poll or a channel under a named deadline that fails with what it waited for (`helper marker was not written`, `background sleep still holds the FIFO`, `Execute did not return after cancel`). Every run is under a one-minute context. |
| Scratch edits reverted | HOLDS | `git status --short` printed nothing after both reverts, before the commit. |
| No attribution lines, no personal paths or hosts | HOLDS | Commit message is one sentence. The test uses `os.Executable` and t.TempDir only. |
| Windows, macOS builds | HOLDS (compile only) | `make cross` exit 0. The new test file is `//go:build unix`, so it is not built for Windows. |
| *RealPlugin* tests | not applicable | No row names one. |

## What changed

- internal/routine/execute.go: the Execute tail builds the output with `startRoutineOutput`, runs the command, calls `cleanupProcess`, then `output.finish(ctx)`. The wait delay and the deferred cleanup are gone. New `routineOutput` type, `startRoutineOutput`, `routineSameWriter` (exec's comparison with its recover) and `finish`.
- internal/routine/process_unix.go, process_windows.go: doc comments on `cleanupProcess` say it now runs before the drain; on Windows it does nothing after Wait because `proctree.Run` has already ended the job.
- internal/routine/jp_routine_drain_unix_test.go: TestJpReBackgroundChild, TestJpReFullOutput, TestJpReSeamNeverStarts, TestJpReContextEnds and its helper TestJpReHelper.

## Notes

- When the run's context has already ended as the drain starts (a timeout), the drain may close the read ends before the last buffered bytes are read; what the copies had read during the run is delivered. Same behaviour as internal/eval's finish. The run returns the timeout error either way.

## NOT VERIFIED on a live screen / platform

- The tests were run on Linux only. The Windows and macOS builds compile (`make cross`), but the drain on Windows (closing a read end while a copy blocks in a read, and the job end closing descendants' handles) was not run there.
- No claude routine was run and no `grid routine run --json` against a real agent; the `--json` writer path is covered by the Go-writer tests (a `*bytes.Buffer` shared for stdout and stderr), not by the CLI.
