# C-eo-flake-session-probe-local

TestProbeLocal took its expected lstart text from `ps` under a 2 s deadline, so it failed on a loaded box. It now runs that `ps` with `exec.Command` and no deadline. A slow `ps` makes the test slower and never wrong, because on Linux the code under test reads /proc and forks no `ps`.

Base: `f197980d` (lane-C-flake-session-process-start, `c06720f9`, is merged in; `git merge-base --is-ancestor c06720f9 HEAD` exits 0). On the base `probe_unix_test.go:22-29` ran `exec.CommandContext(ctx, "ps", ...)` under `context.WithTimeout(..., 2*time.Second)`, and `probe_unix.go:49-50` names `osLstartUTC` and `osLstartLocal`, which `procstart_linux.go:107-108` defines over /proc.

Code commits: `503dcf6a` (new test file), `363c7216` (TestProbeLocal change).

## Proof table

| # | Row | Result | Evidence |
|---|-----|--------|----------|
| 1 | Fails on the base with a 3 s fake ps at probe_unix_test.go:28, "signal: killed" | HOLDS, by a Go test instead of the shell line | The shell line (`printf`, `chmod`, `mktemp`, `;` and a `PATH=` prefix) was refused by the build sandbox's allow list, so it was not run. `TestFlakePLProbeLocalPassesWithSlowPS` builds the same fake ps (`sleep 3`, then the real ps) with `t.Setenv("PATH", ...)` and calls `TestProbeLocal(t)`. On the unchanged base (`go test -count=1 -v -run '^TestFlakePL' ./internal/session/`, exit 1): `probe_unix_test.go:28: signal: killed` then `--- FAIL: TestFlakePLProbeLocalPassesWithSlowPS (3.04s)` and `FAIL github.com/gridconsole/grid-plugins/internal/session 6.307s`. |
| 2 | After the change the same run passes in about 3 s | HOLDS, same equivalent | `go test -count=1 -v -run '^TestFlakePLProbeLocalPassesWithSlowPS$' ./internal/session/` exit 0: `--- PASS: TestFlakePLProbeLocalPassesWithSlowPS (3.23s)`, `ok  github.com/gridconsole/grid-plugins/internal/session 3.240s`. |
| 3 | A TestFlakePL test pins the oracle text of a slow ps given to ProbeLocal as Alive; `-race -count=3` ok | HOLDS | `flake_probe_local_linux_test.go:41` `TestFlakePLSlowPSOracleIsAlive`: PATH set by `t.Setenv` to a temp folder whose `ps` sleeps 3 s then runs the real ps; the oracle runs with `exec.Command` and no deadline; its text goes to `ProbeLocal` and must be Alive. It also fails if the ps took under 3 s, so it cannot pass by running some other ps. `go test -race -count=3 -run '^TestFlakePL' ./internal/session/` exit 0: `ok  github.com/gridconsole/grid-plugins/internal/session 19.987s`. |
| 4 | `go test -race -count=200 -timeout 30m -run '^TestProbeLocal$' ./internal/session/` ok | HOLDS | Exit 0: `ok  github.com/gridconsole/grid-plugins/internal/session 16.311s`. |
| 5 | Diff names probe_unix_test.go and the new test file only; probe_unix_test.go diff is inside TestProbeLocal; the 2000-01-01 token still gives Dead | HOLDS, with the import lines noted | `git diff f197980d --stat` over the two code commits: `internal/session/flake_probe_local_linux_test.go \| 62 +++`, `internal/session/probe_unix_test.go \| 9 ++--`, 2 files changed. This lane doc is a third file, written because the runbook requires it. The probe_unix_test.go hunks are the `context` and `time` import lines (lines 6 and 15) and `:18-37` of TestProbeLocal. The brief allows dropping `context`. `time` had one use in the file, `2*time.Second` in the removed line, so dropping it is what makes the file compile; it is outside TestProbeLocal. The Dead check (`PIDStart: "2000-01-01T00:00:00.000Z"`, now `probe_unix_test.go:34-36`) is unchanged and passes. |
| 6a | `go test -count=1 ./internal/session/` 0 fail | HOLDS | Exit 0: `ok  github.com/gridconsole/grid-plugins/internal/session 7.586s`. |
| 6b | `make lint` exit 0 | HOLDS | Exit 0 (`go vet ./...`, no output). |
| 6c | `make cross` exit 0 | HOLDS | Exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package `ok`. |
| R1 | Small commits, one-sentence messages, no attribution lines | HOLDS | `git log --oneline -3`: `363c7216 Run TestProbeLocal's ps oracle with no deadline.`, `503dcf6a Add tests that run TestProbeLocal and its oracle under a ps slower than 2 s.` Neither message has a Co-Authored-By or Claude-Session line. |
| R2 | Public repo: no personal paths, hosts or emails | HOLDS | The test file and this doc hold no home folder, host name or email; the fake ps lives in `t.TempDir()`. |
| R3 | New tests fail on the base and pass on the lane | HOLDS for the wrapper; see note | `TestFlakePLProbeLocalPassesWithSlowPS` fails on the base (row 1) and passes on the lane (row 2). `TestFlakePLSlowPSOracleIsAlive` passes on the base too (`--- PASS: TestFlakePLSlowPSOracleIsAlive (3.26s)`): it pins the product side, which C-flake-session-process-start already fixed. Row 3 asks for exactly that test. |
| R4 | No existing test edited to pass, none deleted | HOLDS | The only existing test touched is TestProbeLocal, as the brief's Touches names; its checks (`Alive` at :31, `Dead` at :34-36, `Unknown` at :19) are unchanged. No test was deleted. |
| R5 | No guard, gate, cap, hold or blocking hook on agents | HOLDS | Test files only; no non-test file changed. |
| R6 | Tests of the changed package and every package that imports it | HOLDS | `go list` importers of `internal/session`: cli, hook, lease, orchestrator, session, task. `go test -count=1 ./internal/session/ ./internal/cli/ ./internal/hook/ ./internal/lease/ ./internal/orchestrator/ ./internal/task/` exit 0: session 7.586s, cli 50.760s, hook 5.489s, lease 0.180s, orchestrator 54.140s, task 7.188s, all `ok`. `gofmt -l internal/session` prints nothing. The whole-module test and the cross-repo set are left to the orchestrator. |
| R7 | UI rows | n/a | No UI in this lane. |

## What changed

- `internal/session/probe_unix_test.go`: TestProbeLocal runs its ps oracle with `exec.Command` and no context deadline, with a comment saying why; the `context` and `time` imports are gone because nothing else in the file uses them.
- `internal/session/flake_probe_local_linux_test.go` (new, build tag linux): `flakePLSlowPS` helper, `TestFlakePLSlowPSOracleIsAlive`, `TestFlakePLProbeLocalPassesWithSlowPS`. Each takes about 3 s, the fake ps's sleep.

## NOT VERIFIED on a live screen

Nothing here has a screen. Not run: the literal row-1 shell line (refused by the sandbox, covered by the Go test above), a darwin run of TestProbeLocal (darwin's `localHost` still runs ps in the code under test; only the cross build was checked), and a run on a box that is actually loaded.
