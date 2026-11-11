# C-iu-agent-probe-drain: proof table

Lane head with the code and tests: `d91f5c82` (the report commit follows it). Base: `origin/main` 31824184.
Go 1.27.3, linux. Windows and macOS are compile and vet only.

## What changed

- `internal/agent/runner.go`: `probeOutput(ctx, cmd)` gives the child a pipe it owns (`os.Pipe`) as stdout, so os/exec starts no copy goroutine and `Wait` returns when the child exits. One goroutine copies the read end into the 64 KiB `probeBuffer` (wrapped in `iuApSink`, a mutex around it, so the buffer can be read while the copy still runs). After the child exits: end the group, close the write end, wait for EOF. If `ctx` ends first, the read end is closed (in its own goroutine, so a Close that waits for a blocked Read cannot hang the probe) and the output read so far returns with -1. `cmd.WaitDelay` and the `time` import are gone; no timer. The exit-code rules are unchanged, with one addition: a failed run whose context has ended returns -1 (see note 1). `Env.run` passes its ctx.
- `internal/agent/iu_probe_process_unix.go` (`!windows`): `iuApRun` sets `Setpgid`, starts, waits, then `kill(-pid, SIGKILL)`.
- `internal/agent/iu_probe_process_windows.go`: `iuApRun` adds `CREATE_NEW_PROCESS_GROUP` (as `internal/eval` does) and runs the command through `proctree.Run`, which ends the job after the wait.
- `internal/agent/lookup_test.go:93`, `internal/agent/runner_test.go:136`: call sites only (the new `ctx` argument).
- `internal/agent/iu_probe_drain_test.go` (new): `TestIuApBackgroundChild`, `TestIuApFullOutput`, `TestIuApContextEnds`, helper `TestIuApHelper`.

## Proof table

| # | Row or rule | Result |
|---|---|---|
| 1 | `TestIuApBackgroundChild`: `("v1.2.3\n", 0)`, then the FIFO reads to EOF under the deadline "background sleep still holds the FIFO" | HOLDS. `go test ./internal/agent -count=1 -v -run TestIuAp` exit 0, `--- PASS: TestIuApBackgroundChild`. On a scratch checkout of origin/main (test copied, call changed to the old `probeOutput(cmd)`): exit 1, `iu_probe_drain_test.go:80: probeOutput = "v1.2.3\n", -1; want "v1.2.3\n" and 0` / `--- FAIL: TestIuApBackgroundChild (0.11s)`. Test at `iu_probe_drain_test.go:63`. |
| 2 | `TestIuApFullOutput`: code 0 and 64 KiB of "x" | HOLDS. `--- PASS: TestIuApFullOutput`. Scratch edit (the copy goroutine returns after its first read, `runner.go` copy loop), exit 1: `iu_probe_drain_test.go:102: probeOutput: code -1, output length 32768; want 0 and 65536 bytes of x` / `--- FAIL: TestIuApFullOutput (60.06s)`. Reverted; `git status --short` clean before the commit. Test at `iu_probe_drain_test.go:92`. |
| 3 | `TestIuApContextEnds`, no skip: the helper's `setsid` sleep holds stdout, probeOutput has not returned, cancel gives -1 | HOLDS. `--- PASS: TestIuApContextEnds`, also under `-race`, exit 0. Test at `iu_probe_drain_test.go:106`. It does NOT fail on base: on the scratch main run it passed (0.15 s), because the old 100 ms timer had not fired when the test cancelled. So row 3 pins the lane's context path; it is not a regression test for the old bug (row 1 is). |
| 4 | `TestAgentProbeHelper`, `TestProbeOutputTimeout`, `lookup_test.go:93` and the package | HOLDS. `go test ./internal/agent -count=1` exit 0 (`ok ... 5.069s`); also exit 0 with `-race`. |
| 5 | `make lint` | HOLDS, exit 0 (`go vet ./...`, gofmt clean). |
| 5 | `make cross` | HOLDS, exit 0: `cross: darwin/arm64 vet`, `cross: windows/amd64 vet` and both test compiles, every package `ok`. This compiles and vets `iu_probe_process_windows.go`. |
| 6 | `git diff origin/main --stat` names only the Touches files | HOLDS: `iu_probe_drain_test.go`, `iu_probe_process_unix.go`, `iu_probe_process_windows.go`, `lookup_test.go`, `runner.go`, `runner_test.go`, plus `docs/lanes/C-iu-agent-probe-drain.md` (the report the runbook asks for). |
| R1 | Tests of the changed package and every importer | HOLDS. Importers by `go list` (Deps, and Test/XTestImports): `internal/cli`, `lib`, `cmd/grid`. `go test -count=1 ./internal/agent ./internal/cli ./lib ./cmd/grid` exit 0: `ok internal/agent 5.069s`, `ok internal/cli 42.553s`, `ok lib 40.035s`, `cmd/grid [no test files]`. |
| R2 | Windows and macOS builds | HOLDS: `make cross` above. |
| R3 | New tests fail on base | Row 1 HOLDS (pasted above). Row 2 by the scratch edit the brief names (pasted above). Row 3 passes on base, see its line. |
| R4 | No existing test edited to pass, none deleted, skipped or quarantined | HOLDS. The only edits in existing tests are the `ctx` argument at `runner_test.go:136` and `lookup_test.go:93`. |
| R5 | No sleep, elapsed-time assertion or timeout as proof in a test | HOLDS. The tests wait on the FIFO, on channels and on the helper's exit; `iuApDeadline` (60 s) and the 10 s FIFO read deadline only bound a wait and fail by name. The FIFO deadline is below the 30 s of the background sleep, so an expiry cannot be mistaken for the sleep ending. |
| R6 | No guard, gate, cap, hold or blocking hook on agents | HOLDS. The change reads a probe to the end and ends the probe's own process group. Nothing is added to an agent run. |
| R7 | Names: package-level `iuAp*`, tests `TestIuAp*`, no `probeContext` | HOLDS: `iuApSink`, `iuApRun`, `iuApProbe`, `iuApHelperMode`, `iuApDeadline`, `TestIuAp*`. |
| R8 | Commits: one sentence each, no attribution lines, no personal paths or hosts | HOLDS: three commits `596af7c6`, `c8714124`, `d91f5c82` (checked with `git log -3 --format=%B`), plus the report commit. |
| R9 | Pushing, `claude`, `curl`, background servers, billing | None used. No push. |

## Notes (not rows)

1. A failed run with an ended context returns -1. Without it, a child killed by the context returned 1 or -1 depending on whether the copy or the context finished first. `Check` reads `ctx.Err()` itself, so it sees the same thing either way. No test pins this exact branch (`TestProbeOutputTimeout` checks only `code != 0`).
2. `iu_probe_drain_test.go` carries `//go:build !windows` for the whole file, so `TestIuApFullOutput` (row 2) is Unix-only too. The brief names one new test file; `TestRunnerBoundedOutputAndWorkingDirectory` still covers the 64 KiB bound on Windows.
3. The kill after the wait uses the pid of an already reaped child. The kernel keeps a group id reserved while any member lives; with no member left the kill finds nothing. It could reach an unrelated process only if the pid were reused within microseconds and became a group leader.
4. The `go-conventions` skill is not in this session's skill list; I followed the neighbouring files (`internal/eval/process_*.go`, `internal/proctree`).

## NOT VERIFIED

- Windows runtime: `iu_probe_process_windows.go` and the read-end close with a blocked Read are compiled and vetted by `make cross` only, and I could not read the Go `pipe_windows.go` source in this sandbox. The close runs in its own goroutine so that a Close which waits for a blocked Read leaves the probe returning; the cost is one goroutine that ends when the last holder of the pipe exits. The known proctree gap (a descendant started before the job is assigned) applies.
- macOS runtime: compiled and vetted only (`make cross`); the Unix code is the same file as on Linux.
- No live screen: this lane has no UI row.
- A real `claude --version` or `codex --version` was not run (the brief forbids it).
