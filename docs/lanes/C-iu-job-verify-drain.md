# C-iu-job-verify-drain: `grid job verify` reads the check's output to EOF after its group is ended

Base: origin/main 31824184 (the lane started there, 0 commits ahead). Code commit: 8eda4707. This report is committed on top of it.

## Proof table

| # | Row / rule | State | Proof |
|---|---|---|---|
| 1 | TestIuJvBackgroundChild: background sleep holding the output and a FIFO; err nil, Pass, exit 0, Output "ok\n"; FIFO reaches EOF under a named deadline | HOLDS | `go test ./internal/job -run '^(TestIuJv.*\|TestVerifyExitedShellKillsDescendants\|TestVerifyCancelKillsChild)$' -count=1 -v` exit 0: `--- PASS: TestIuJvBackgroundChild (0.02s)`. Base: same run with the two test changes (new file plus the row 2 edit) on unchanged production code at origin/main 31824184: `iu_verify_drain_test.go:57: a shell that exits 0 with a background child should pass: {Pass:false ExitCode:0 Tests:<nil> Commit: Reason:exec: WaitDelay expired before I/O complete Output:ok` / `--- FAIL: TestIuJvBackgroundChild (0.26s)`. No kill(pid, 0) or /proc probe: EOF on the FIFO is the proof (iu_verify_drain_test.go:61-70) |
| 2 | TestVerifyExitedShellKillsDescendants changed at :230-231 only to the default rule; descendant check kept | HOLDS | verify_test.go:230-231 (diff: 2 lines changed); :233-236 unchanged. `go test ./internal/job -run '^(TestVerifyExitedShellKillsDescendants\|TestVerifyCancelKillsChild)$' -count=1 -v` exit 0: `--- PASS: TestVerifyCancelKillsChild (0.02s)`, `--- PASS: TestVerifyExitedShellKillsDescendants (0.02s)`. Base: `verify_test.go:231: a shell that exits 0 passes after its group is ended: {Pass:false ExitCode:0 Tests:<nil> Commit: Reason:exec: WaitDelay expired before I/O complete Output: OutputTruncated:false} exec: WaitDelay expired before I/O complete` |
| 3 | TestIuJvFullOutput: 3000 `--- PASS: TestA<i> (0.00s)` lines (about 80 KiB, more than a pipe buffer), MinTests 3000, Pass and Tests 3000 | HOLDS | PASS in the row 1 run (`--- PASS: TestIuJvFullOutput (0.04s)`). Scratch edit in verify.go: the copy goroutine reads once into `output`, then discards the rest (so the shell is not stuck on a full pipe): `iu_verify_drain_test.go:83: all 3000 test lines should be read: pass=false exit=0 tests=125 reason="only 125 tests; at least 3000 required" err=<nil>` / `--- FAIL: TestIuJvFullOutput (0.37s)`. Reverted with `git checkout -- internal/job/verify.go`; `git status --short` empty. It also passes on base: base exec copies the whole output as well. The test pins the new copy |
| 4 | TestIuJvContextEnds, no skip: helper test binary starts `sleep 600` with Setsid sharing stdout, writes marker, prints ok, exits 0; RunVerify has not returned; cancel gives context.Canceled | HOLDS | PASS in the row 1 run (`--- PASS: TestIuJvContextEnds (0.09s)`, `--- PASS: TestIuJvHelper (0.00s)`). The sleep is killed in t.Cleanup from the pid in the marker (iu_verify_drain_test.go:96-100). It also passes on base: base returns 250 ms after the shell exits, and its ctx check (base verify.go:135) comes before the error check, so a cancel inside those 250 ms gives Canceled. Extra mismatch proof: scratch edit that drops `_ = pr.Close()` from the ctx case (verify.go:147): `iu_verify_drain_test.go:133: RunVerify did not return after its context ended` / `--- FAIL: TestIuJvContextEnds (60.17s)`. Reverted; `git status --short` empty |
| 5 | `go test ./internal/job -count=1`, `make lint`, `make cross` exit 0 | HOLDS | `go test ./internal/job -count=1 && echo` printed `ok ... internal/job 1.739s` and `row5 go test exit 0`; `make lint && echo` printed `make lint exit 0`; `make cross >/dev/null 2>&1 && echo` printed `make cross exit 0` (Makefile:33-40: vet plus test compile for darwin/arm64 and windows/amd64) |
| 6 | `git diff main --stat` names only the Touches files | HOLDS | The code commit lists `internal/job/iu_verify_drain_test.go`, `internal/job/verify.go`, `internal/job/verify_process_unix.go` and `internal/job/verify_test.go` (4 files, +190 -8). verify_process_windows.go is unchanged because no new per-platform function was needed. The only other file is this report, which the builder runbook requires (docs/lanes/<ID>.md) |
| R | Remove `cmd.WaitDelay`; nothing else in non-test code reads it | HOLDS | `git grep -n WaitDelay -- internal/job ':!*_test.go'` printed nothing and exited 1. The old comment at verify_process_unix.go:25-26 that named WaitDelay was rewritten. The Windows test helpers that set their own WaitDelay are unchanged |
| R | Tests of changed packages and every importer | HOLDS | The importer list from `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E " .../internal/job( \|$)"` was contract/formats, internal/cli, internal/hook, internal/job, internal/job/jobcmd, internal/job/lint, internal/jobgraph, internal/lease, internal/migrate, internal/orchestrator, internal/plan and internal/ports. `go test -count=1 <those 12> && echo` printed 12 `ok` lines and `importers go test exit 0` |
| R | New tests fail on base and pass on the lane | HOLDS | Rows 1 and 2 fail on base (lines above). Rows 3 and 4 pass on base by their nature: they pin the new copy and the new ctx path. Their mismatch proofs are pasted above |
| R | No existing test edited to pass, none deleted, skipped or quarantined | HOLDS | Only the one expectation the brief names (row 2, owner question 1 default) changed. No t.Skip in the new file |
| R | No guard, gate, cap, hold or limit | HOLDS | The change removes a timer and adds none. `grid job verify` still runs under signal.NotifyContext only (owner question 4) |
| R | No sleep, elapsed-time assertion or timeout as proof | HOLDS | Deadlines only bound waits and fail by name: "background sleep still holds the FIFO" (:66), "helper never wrote its marker" (:117), "RunVerify did not return after its context ended" (:133). `sleep` appears only as the fixture's background process |
| R | Scratch edits reverted before commit | HOLDS | Both were made after commit 8eda4707 and reverted with `git checkout -- internal/job/verify.go`. `git status --short` was empty after each |
| R | Small one-sentence commits, no attribution, no personal paths | HOLDS | `git log origin/main..HEAD` shows the code commit and this report commit |
| R | RealPlugin and UI rules | n/a | No row names a *RealPlugin* test, and there is no UI |

## What changed

- `internal/job/verify.go:122-151`: RunVerify gives the command a pipe it owns (`os.Pipe`; stdout and stderr both get the write end), so os/exec starts no copy goroutine and `proctree.Run` returns when the shell exits. A goroutine started before `proctree.Run` copies the read end into `verifyOutput`. After Run, `cleanupVerifyProcess` ends the group on Unix (on Windows proctree has already ended the job). Then the write end is closed and RunVerify waits for the copy to reach EOF. If ctx ends first, the read end is closed and the copy is awaited, because `verifyOutput` has no lock and must not be read while the copier writes. RunVerify then returns through the existing ctx path. `cmd.WaitDelay` and the `time` import are gone, and `io` is imported. There is no timer anywhere.
- Result rule (owner question 1, default): the shell's exit code decides. A background process still holding the output is ended with the group, and the check is judged on its exit code and output as any other check.
- `internal/job/verify_process_unix.go:25-27`: the comment now says the group is ended after the shell is waited for, so the pipe can reach EOF.
- `internal/job/verify_test.go:230-231`: the row 2 expectation.
- New `internal/job/iu_verify_drain_test.go` (`//go:build unix`): TestIuJvBackgroundChild, TestIuJvFullOutput, TestIuJvContextEnds, and TestIuJvHelper (the child-process role; it returns at once in a normal run).

Why closing the read end wakes the copier on every OS (`go doc os.File.Close`): "On files that support File.SetDeadline, any pending I/O operations will be canceled and return immediately with an ErrClosed error". `go doc os.File.SetDeadline` adds: "On most systems ordinary files do not support deadlines, but pipes do." exec passes `pw` to the child through `File.Fd()`, which puts that end in blocking mode, so the shell's writes never see EAGAIN. `pr` keeps its pollable mode.

## For review (contract and cross-platform)

- A process outside the group or job keeps RunVerify waiting until it closes the output or ctx ends. Examples are setsid, a daemon, or on Windows a descendant started before the job was attached (proctree.go:10-13). Before, it failed after 250 ms with ErrWaitDelay. `grid job verify` has no deadline (owner question 4), so such a check waits for its process or for Ctrl+C. TestIuJvContextEnds pins this.
- Output that a background process wrote before the group was killed is now in Output and counted.

## NOT VERIFIED

- Windows and macOS at runtime: `make cross` vets and compiles both, but nothing ran there. On Windows the normal path reaches EOF without entering the ctx branch, because proctree ends the job before Run returns. The ctx branch relies on the os.File.Close contract quoted above. If Close did not cancel a pending read on a Windows anonymous pipe, a cancelled verify whose output is still held by a process outside the job (proctree.go:10-13, or one that broke away) would wait for that process before returning Canceled. That is a late return, not a wrong verdict.
- No live screen is involved; there is no UI.

## Open items

None inside the brief.

DONE
