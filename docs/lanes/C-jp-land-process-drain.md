# C-jp-land-process-drain

Plugin main read: 99c11b7a (main and origin/main in the lane clone). Lane head before this report: 0611bec9.

`grid land` now gives the deploy command two pipes it owns (stdout, stderr). Wait returns when the shell exits, the group is killed, and then the pipes are read to EOF. If ctx ends first, the read ends are closed. There is no timer. `cmd.WaitDelay` is gone, so a deploy that exits 0 while a background child still holds the output is recorded as `land landed` with its full output.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1 TestJpLdBackgroundChild | HOLDS | `go test ./internal/land -count=1 -v -run '^(TestJpLd.*\|...)$'` exit 0: `--- PASS: TestJpLdBackgroundChild (0.44s)`. Test at internal/land/jp_land_drain_unix_test.go:41-86: FIFO made in t.TempDir() and opened O_RDONLY\|O_NONBLOCK before `s.Run`. The run is under a one-minute context. It checks err nil, Status landed, ExitCode 0, Stdout `ok\n` and `land landed` in history.md, then reads the FIFO to EOF under the named deadline `background sleep still holds the FIFO`. There is no kill(pid,0) and no /proc read; bg.pid is used only by a cleanup kill. |
| 1 fails on main | HOLDS | Main's process.go (99c11b7a, via `git stash push -- internal/land/process.go`) with the lane's test, `go test ./internal/land -run '^(TestJpLdBackgroundChild\|TestJpLdFullOutput\|TestJpLdContextEnds)$' -count=1 -v` exit 1: `jp_land_drain_unix_test.go:61: land failed: exec: WaitDelay expired before I/O complete` / `--- FAIL: TestJpLdBackgroundChild (0.35s)`. Then `git stash pop`, and `git status --short` showed only the lane's own files. |
| 2 TestJpLdFullOutput | HOLDS | `--- PASS: TestJpLdFullOutput (0.03s)` (same run). jp_land_drain_unix_test.go:89-101: `sh -c` loop prints `line 1`..`line 3000` under a one-minute context. The test checks Status passed, Truncated false, and that Stdout equals all 3000 lines byte for byte. |
| 2 scratch edit | HOLDS | Scratch: `drain` returned after its first read (`if err != nil \|\| true`). `go test ./internal/land -run '^TestJpLdFullOutput$' -count=1 -v` exit 1: `jp_land_drain_unix_test.go:100: status=passed truncated=false lines=314 bytes=2718 want lines=3000 bytes=28893` / `--- FAIL: TestJpLdFullOutput (0.04s)`. Reverted with `git checkout -- internal/land/process.go`; `git status --short` empty. |
| 3 TestJpLdContextEnds | HOLDS | `--- PASS: TestJpLdContextEnds (0.08s)` (same run), no skip. jp_land_drain_unix_test.go:129-188: `sh -c "exec '<os.Executable()>' -test.run='^TestJpLdHelper$' -- jp-ld-setsid '<marker>'"`. The helper (:105-127) starts `sleep 600` with SysProcAttr{Setsid: true} on its own stdout, writes the marker with the sleep pid (tmp+rename), prints ok and exits 0. t.Cleanup kills the sleep. The test polls for the marker with a ticker under the named deadline `helper never wrote the marker`, then does a non-blocking select that runProcess has not returned, then cancel(). It checks Status `error` and Error `context canceled` under the named deadline `runProcess did not return after cancel`. |
| 3 scratch edit | HOLDS | Scratch: the `select { drained / ctx.Done }` in `finish` became `<-drained`. `go test ./internal/land -run '^TestJpLdContextEnds$' -count=1 -v` exit 1: `jp_land_drain_unix_test.go:185: runProcess did not return after cancel` / `--- FAIL: TestJpLdContextEnds (30.05s)`. Reverted with `git checkout -- internal/land/process.go`; `git status --short` empty. |
| 4 existing three | HOLDS | `go test ./internal/land -run '^(TestTimeoutKillsDescendants\|TestProcessBoundsOutput\|TestDeployEnvironmentAllowlist)$' -count=1 -v` exit 0: `--- PASS: TestTimeoutKillsDescendants (0.09s)`, `--- PASS: TestProcessBoundsOutput (0.09s)`, `--- PASS: TestDeployEnvironmentAllowlist (0.06s)`. These three passed again on 0611bec9. |
| 4 whole package | HOLDS | `go test ./internal/land -count=1` exit 0 on 0611bec9: `ok  github.com/gridconsole/grid-plugins/internal/land 5.292s`. Also `go test -race ./internal/land -count=5 -run '^(TestJpLd.*\|TestProcessBoundsOutput\|TestDeployEnvironmentAllowlist)$'` exit 0. |
| 5 lint | HOLDS | `make lint` exit 0 on 0611bec9 (go vet ./... and gofmt -l print nothing). |
| 5 cross | HOLDS | `make cross` exit 0 on ab119fb2 (full output, every package `ok` for darwin/arm64 and windows/amd64). It ran again on 0611bec9 (comment-only change) and reached `cross: windows/amd64 test compile` with `ok .../internal/land`. No FAIL line. |
| 6 diff stat | HOLDS | `git diff main --stat` before the report commit: `internal/land/jp_land_drain_unix_test.go \| 187 +++`, `internal/land/process.go \| 90 ++-`, `2 files changed, 274 insertions(+), 3 deletions(-)`. This report (docs/lanes/C-jp-land-process-drain.md) is the third file, as the builder runbook requires. |
| WaitDelay removed | HOLDS | `git grep -n WaitDelay -- internal/land ':!*_test.go'` prints nothing (exit 1, no match) on 0611bec9. |
| Importers tested | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}} ...' ./... \| grep 'internal/land\b'` gives `internal/cli` and `internal/land`. `go test ./internal/cli -count=1` exit 0 (`ok ... 70.846s`, on ab119fb2; 0611bec9 changes only a comment). |
| Edit only Touches | HOLDS | Only process.go changed in product code. The new lines are runProcess (:42-76 on main) plus three imports (`io`, `os`, `sync`), which the pipes need. process_unix.go and process_windows.go are unchanged. |
| Naming | HOLDS | Package-level test names: `jpLdWait`, `jpLdKillPidFile`, `TestJpLdBackgroundChild`, `TestJpLdFullOutput`, `TestJpLdHelper`, `TestJpLdContextEnds`. First line of the test file: `//go:build unix`. |
| No timer / no guard | HOLDS | No timer, deadline, cap or gate was added to product code. `finish` waits on EOF or on the existing ctx (the land's own s.Timeout, land.go:225-236). The 30 s `jpLdWait` exists only in tests and bounds waits that fail by name. |
| Tests: no sleep as proof, contexts | HOLDS | No sleep and no elapsed-time assertion. Rows 1 and 2 run under `context.WithTimeout(..., time.Minute)`; row 3 uses an explicit cancel. |
| No test deleted or edited | HOLDS | `git diff main --stat` shows no existing test file. |
| Scratch edits reverted | HOLDS | `git status --short` was empty after each revert. The committed process.go is the version that was tested. |
| Commits | HOLDS | ab119fb2 and 0611bec9, plus the report commit. Each has a one-sentence message with no attribution lines. |
| RealPlugin | n/a | No row names a *RealPlugin* test. |

## What changed

- internal/land/process.go: new `outputPipes` (two `os.Pipe` pairs, `start`, `finish(ctx)`) and `drain`, the same shape as eval's childPipes (internal/eval/process.go:50-130) without the stdin pipe. runProcess calls them in this order: `start`, `proctree.Run`, `cleanupProcess`, `finish(ctx)`. `cmd.Stdout`/`cmd.Stderr` are the `*os.File` write ends, so exec starts no copy goroutine. `cmd.WaitDelay` is removed. The result rule is unchanged: the shell's exit code decides.
- internal/land/jp_land_drain_unix_test.go: rows 1-3.

## NOT VERIFIED

- Windows at run time: on Windows, proctree.Run ends the job object before `finish` reads the pipes, and closing a pipe read end unblocks a pending read (os.File Close cancels pipe I/O). Both are the same as in the accepted eval shape, but neither was run on a Windows machine. `make cross` only compiles and vets.
- macOS at run time: compiled and vetted by `make cross`; the unix tests were not run on a Mac.

## Open items

- None in scope. The shared pipe helper for eval and land is left for later (owner default, INDEX Waits W1).

DONE
