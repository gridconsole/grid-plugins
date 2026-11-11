# A-win-verify-land-tree: job verify and land stop their process tree on Windows the way eval and routine do

Repo: plugin, branch `lane-A-win-verify-land-tree`, from main. Six files, 5 commits, no tag lines.

## Proof table

| Row / rule | Status | Proof |
| --- | --- | --- |
| 1a. `verify_process_windows.go`: `CreationFlags` gains `CREATE_NEW_PROCESS_GROUP`, `CmdLine` from `windowsVerifyCommandLine` kept | HOLDS | `internal/job/verify_process_windows.go:20-23`: `cmd.SysProcAttr = &syscall.SysProcAttr{ CmdLine: windowsVerifyCommandLine(command), CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP, }`. |
| 1b. Cancel and cleanup use one `killProcessTree`: taskkill by absolute path from `SystemRoot`, 5 s context, `Process.Kill` fallback | HOLDS | `verify_process_windows.go:24`: `cmd.Cancel = func() error { return killProcessTree(cmd) }`; `:33`: `_ = killProcessTree(cmd)`; `:39`: `ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)`; `:41`: `exec.CommandContext(ctx, windowsTaskkillPath(os.Getenv("SystemRoot")), taskkillArgs(cmd.Process.Pid)...)`; `:43`: `if kerr := cmd.Process.Kill(); errors.Is(kerr, os.ErrProcessDone)` (same body as `eval/process_windows.go:35-46`). The old `exec.Command("taskkill", ...)` found on PATH is gone. |
| 1c. `cleanupVerifyProcess` acts only while `cmd.ProcessState == nil` | HOLDS | `verify_process_windows.go:32`: `if cmd.Process != nil && cmd.ProcessState == nil {`. |
| 2. `land/process_windows.go`: same flag keeping a `SysProcAttr` already set; `cleanupProcess` ends a still-running tree; Cancel unchanged in behaviour | HOLDS | `land/process_windows.go:18-21`: `if cmd.SysProcAttr == nil { cmd.SysProcAttr = &syscall.SysProcAttr{} }` then `cmd.SysProcAttr.CreationFlags \|= syscall.CREATE_NEW_PROCESS_GROUP`; `:28-31` cleanup under `cmd.Process != nil && cmd.ProcessState == nil`. The Cancel body moved unchanged into `killProcessTree` (`:34-45`): same 5 s timeout, same `windowsTaskkillPath(os.Getenv("SystemRoot"))`, same `/T /F /PID`, same `killer.Env = allowedEnvironment(nil)`, same `Process.Kill` fallback and the same `terminate deploy process tree` error. |
| 3. `verify_taskkill_test.go` (portable) | HOLDS | `go test ./internal/job/ -run 'TestVerifyWindowsTaskkillPath\|TestVerifyTaskkillArgs' -count=1 -v`: exit 0, `--- PASS: TestVerifyWindowsTaskkillPath` with `#00` (empty), `C:\Windows`, `C:/Windows/`, `D:\Windows`, `D:\Windows\`, `D:/Windows/` all PASS (`verify_taskkill_test.go:9-14`), and `--- PASS: TestVerifyTaskkillArgsEndTheWholeTree` (`:25-29`, `["/T","/F","/PID","42"]`). |
| 4a. Job Windows test: grandchild tree, cancel after the ready marker, returns in 10 s, both pids (and the shell) gone | HOLDS (compiled, `internal/job/verify_process_windows_test.go:113-138`) | Drives `verifyShell` with `cmd.WaitDelay = 250ms` (`:52`, as `verify.go:122`); waits for the pid line (`:124`), cancels (`:125`), asserts return in 10 s (`:131`), `cleanupVerifyProcess` as `verify.go:127` (`:134`), `requireGone` shell, child, grandchild (`:135-137`). |
| 4b. Land Windows test: `runProcess` with the same tree and cancel leaves no pid | HOLDS (compiled, `internal/land/process_windows_test.go:98-121`) | `runProcess` on the helper tree; pids read from a file because `runProcess` keeps output until the end (`:60-75`); pids read at `:109`, cancel (`:110`), return in 10 s (`:116`) with status not `passed` and error `context canceled` (`:113-114`), `requireGone` child and grandchild (`:119-120`). |
| 4c. Cleanup on a started, not yet waited command ends both pids | HOLDS (compiled, `verify_process_windows_test.go:140-153`; `land/process_windows_test.go:123-136`) | `cleanupVerifyProcess` / `cleanupProcess` before `Wait`, then both pids gone. Both packages also have a test that cleanup on an unstarted and on a finished command does nothing (`verify_process_windows_test.go:169`, `land/process_windows_test.go:147`). |
| 4d. Keeps-attributes tests | HOLDS (compiled, `land/process_windows_test.go:138-145`; `verify_process_windows_test.go:155-167`) | Land: pre-set `HideWindow` kept, flag set, Cancel set (as eval :135). Job: `CmdLine == windowsVerifyCommandLine(command)`, flag set, Cancel set. |
| 4e. `GOOS=windows go vet` and `go test -c` | HOLDS | `GOOS=windows go vet ./internal/job/ ./internal/land/ && echo` printed `WINDOWS_VET_EXIT=0`; `GOOS=windows go test -c -o /dev/null ./internal/job/` printed `JOB_WINDOWS_TEST_COMPILE_EXIT=0`; the same for `./internal/land/` printed `LAND_WINDOWS_TEST_COMPILE_EXIT=0`. |
| 5a. Linux unchanged: `go test ./internal/job/ ./internal/land/ -count=1` | HOLDS | `ok .../internal/job 3.635s`, `ok .../internal/land 3.409s`. |
| 5b. `GOOS=darwin go vet ./internal/job/ ./internal/land/` | HOLDS | printed `DARWIN_VET_EXIT=0`. |
| 5c. `make lint` | HOLDS | printed `go vet ./...` then `LINT_EXIT=0`; `gofmt -l internal/job internal/land` printed nothing. |
| Tests of every package that imports job or land | HOLDS | List: `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E "internal/(job\|land)( \|$)"` gives contract/formats, internal/cli, internal/hook, internal/job, internal/job/jobcmd, internal/job/lint, internal/jobgraph, internal/land, internal/lease, internal/orchestrator, internal/plan, internal/ports. `go test -count=1` on all of them: all `ok` (cli 35.3s, orchestrator 29.0s; frontmatter ok; jobtest has no test files). The whole-module test is the orchestrator's step. |
| Windows build | HOLDS | `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` printed `WINDOWS_BUILD_EXIT=0`. |
| macOS build | HOLDS | `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./...` printed `DARWIN_BUILD_EXIT=0`. |
| New tests fail on base and pass on the lane | HOLDS for the portable test; NOT VERIFIED for the Windows-only ones | With `verify_taskkill.go` stashed, `go test ./internal/job/ -run 'TestVerifyWindowsTaskkillPath\|TestVerifyTaskkillArgs' -count=1` printed `internal/job/verify_taskkill_test.go:18:14: undefined: windowsTaskkillPath`, `internal/job/verify_taskkill_test.go:27:12: undefined: taskkillArgs`, `FAIL .../internal/job [build failed]`; with it restored both pass (row 3). The Windows-only tests were not run (no Windows runtime): on base `cleanupVerifyProcess` and `cleanupProcess` are empty, so the cleanup tests would leave the grandchild running, and the keeps-attributes tests would find no `CREATE_NEW_PROCESS_GROUP`. |
| No existing test edited or deleted | HOLDS | `git diff --stat origin/main..HEAD`: six files, three new test files, 445 insertions, 22 deletions; the deletions are the old bodies of `verify_process_windows.go` and `land/process_windows.go`. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change ends a process tree when a run is already being cancelled or is cleaned up; nothing runs before an agent starts. |
| Only the named files; verify.go, land/process.go, eval, routine untouched; Unix files untouched | HOLDS | Same `git diff --stat` output. |
| One commit message per sentence, no attribution lines, no personal paths, hosts or emails | HOLDS | `git log --format='%h %s' origin/main..HEAD` lists five one-sentence subjects, no trailers; the files name no person, host or home path. |

## What changed

- `internal/job/verify_taskkill.go` (new): portable `windowsTaskkillPath` and `taskkillArgs`, a copy of eval's, so job keeps its own copy as the brief asks.
- `internal/job/verify_process_windows.go`: `verifyShell` sets `CREATE_NEW_PROCESS_GROUP` and keeps `CmdLine`; Cancel and `cleanupVerifyProcess` share `killProcessTree` (OS taskkill by absolute path, 5 s timeout, `Process.Kill` fallback); cleanup acts only on a command that has not been waited for.
- `internal/land/process_windows.go`: `configureProcess` sets the same flag and keeps any `SysProcAttr`; `cleanupProcess` ends a still-running tree; the Cancel body moved unchanged into `killProcessTree`.
- Tests: `internal/job/verify_taskkill_test.go` (portable), `internal/job/verify_process_windows_test.go`, `internal/land/process_windows_test.go` (Windows only).

One difference in the job Windows runner: when taskkill itself fails and `Process.Kill` succeeds, Cancel now returns a `terminate process tree` error (as eval does) where it used to return the result of `Process.Kill()`. `verify.go:133` checks `ctx.Err()` first, so the verification result is the same.

## Open items

- Not claimed, not changed: a `cmd.exe` that exits while its descendants live on. `verify.go:126-127` and `land/process.go:54-56` call cleanup after `Run` has returned, so `ProcessState` is set and cleanup does nothing then. This stays open exactly as in `docs/lanes/G-windows-process-tree.md` (Open items): it needs a job object and a caller change.
- `windowsTaskkillPath` now exists in eval, routine, land and job; a shared home would remove the copies (brief says keep them).

## NOT VERIFIED on a live screen

- No Windows process was started: the Windows-only tests (`TestWindowsVerifyCancelKillsProcessTree`, `TestWindowsVerifyCleanupKillsProcessTree`, `TestWindowsVerifyShellKeepsCommandLineAndStartsProcessGroup`, `TestWindowsVerifyCleanupSkipsUnstartedAndFinishedCommands`, and in land `TestWindowsCancelKillsProcessTree`, `TestWindowsCleanupKillsProcessTree`, `TestWindowsConfigureProcessKeepsAttributes`, `TestWindowsCleanupSkipsUnstartedAndFinishedCommands`) compile and vet for windows/amd64 only. In the job test the cmd.exe command line is `"<test binary>" -test.run=TestWindowsVerifyTreeHelper`; how cmd.exe `/d /s /c` strips the outer quotes was read from the cmd.exe rules, not run.
- Whether `CREATE_NEW_PROCESS_GROUP` changes how a console agent handles the parent's Ctrl+C on a real console.
- Nothing here is a UI row.

Windows host run: for the orchestrator: `go test ./internal/job/ ./internal/land/ -run 'Windows' -count=1 -v`
