# C-qw-enospc-exit-four: a full disk on any write exits 4; an unwritable Grid home says so in a first line with no path

Base: 44c7de5b (plugin main). Code head: 9a6b7e3c (two commits: the production change, then the tests). All proof below ran on 9a6b7e3c. The report commit after it is doc-only.

Base check: on 44c7de5b `internal/cli/exitcode.go:19-20` reads

```go
	case errors.Is(err, feed.ErrDiskFull):
		return 4
```

and nothing else gives 4. A wrapped `syscall.ENOSPC` is not `feed.ErrDiskFull` (`diskError`, internal/feed/store.go:462-466, folds it with `%v`, not `%w`), so it gave 1. Base is not already 4, so the brief stands.

Red on base. These are the first two tests of the file (TestQwEnExitCode, TestQwEnHomeLine) run against unmodified production code:

```
--- FAIL: TestQwEnExitCode (0.00s)
    --- FAIL: TestQwEnExitCode/enospc_under_a_path_error (0.00s)
        qw_disk_full_test.go:42: exitCode(write task.md: write /x/task.md: no space left on device) = 1, want 4
    --- FAIL: TestQwEnExitCode/bare_enospc (0.00s)
        qw_disk_full_test.go:42: exitCode(no space left on device) = 1, want 4
--- FAIL: TestQwEnHomeLine (0.00s)
    --- FAIL: TestQwEnHomeLine/erofs (0.00s)
        qw_disk_full_test.go:85: stderr = "grid task x: write task.md: write /x/task.md: read-only file system\n", want "grid: the Grid home cannot be written" then "grid task x: ..."
    --- FAIL: TestQwEnHomeLine/fs_permission (0.00s)   (same shape: permission denied)
    --- FAIL: TestQwEnHomeLine/eacces (0.00s)          (same shape: permission denied)
FAIL	internal/cli
```

The other three tests (TestQwEnFinishExit, TestQwEnRecordOnce, TestQwEnDiskFullPlatform) name symbols that do not exist on base (`qwEnFinishExit`, the `err` field, `qwDiskFull`), so the committed test file fails on base as a build error. The red lines above were taken before those three were added.

## Proof table

| # | Row / rule | Result | Command / place |
|---|---|---|---|
| 1 | exitCode table: wrapped path-error ENOSPC 4, wrapped feed.ErrDiskFull 4, fs.ErrPermission 1, EACCES 1, EROFS 1, EIO 1, plain "disk full" text 1, intent.ErrClaimed 5, feed.ErrStaleLease 3 | HOLDS | `go test -count=1 -v -run 'TestQwEn' ./internal/cli/` exit 0: `--- PASS: TestQwEnExitCode` with 10 sub-tests PASS (internal/cli/qw_disk_full_test.go:24). Red on base: see above. |
| 2a | Finish mapper as a pure function: (1, ENOSPC) 4; (1, EROFS) 1; (1, fs.ErrPermission) 1; (2, any) 2; (0, nil) 0 | HOLDS | same run: `--- PASS: TestQwEnFinishExit` 11 sub-tests (qw_disk_full_test.go:52). Mapper is `qwEnFinishExit`, internal/cli/exitcode.go:33. Also (1, feed.ErrDiskFull) 4, (3, ENOSPC) 3, (1, nil) 1. |
| 2b | recordError then the command's own Fprintf: `grid: the Grid home cannot be written` is the first line for wrapped EROFS and fs.ErrPermission (and EACCES), the command's line stays second, first line holds no `/` or `\`; no extra line for ENOSPC, EIO or a plain error | HOLDS | same run: `--- PASS: TestQwEnHomeLine` 6 sub-tests (qw_disk_full_test.go:129); `--- PASS: TestQwEnRecordOnce` (the line prints once however many times a command records; the last recorded error is kept; no recorder prints nothing); `--- PASS: TestQwEnDiskFullPlatform` (on Linux, Errno 112 and 39 are not a full disk; on Windows they are). No row depends on file permissions on disk. |
| 2c | The deferred remap in Run | NOT TESTED THROUGH Run | `internal/cli/cli.go:200` is `exit = qwEnFinishExit(exit, f.err)`. No test executes it: no seam inside Touches makes a command return a raw ENOSPC through Run, and the task writers open files with OpenNoFollow or O_EXCL, so a /dev/full symlink does not reach them. Row 2 pins the pure function, and `f.err` is set at error_codes.go:113. The closure is two lines and is compiled by every cli test that passes `--json`. |
| 3 | `go test -count=1 ./internal/cli/ ./internal/feed/` | HOLDS | exit 0: `ok internal/cli 52.005s`, `ok internal/feed 18.152s`. TestExitCode (exitcode_test.go:12) and exit_refusal_test.go:27 are unchanged and pass. |
| 4 | `make lint` and `make cross` | HOLDS | `make lint` (go vet ./...) exit 0. `make cross` exit 0: darwin/arm64 and windows/amd64, each vet plus test compile of every package, so qw_disk_full_windows.go and the test file build on Windows. |
| R1 | Tests of changed package and every importer | HOLDS | Direct importers of internal/cli (go list): cmd/grid (no tests), lib. Packages whose tests import it: contract/formats, internal/account, internal/braincmd, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings. `go test -count=1 ./internal/cli/ ./internal/feed/ ./cmd/grid/ ./lib/` exit 0 (`ok lib 12.739s`); the nine others exit 0, all `ok`. The whole-module test is the orchestrator's. |
| R2 | Walk tests (plugin set) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` -> `ok internal/cli 0.050s`. `go test -count=1 ./internal/job/lint/` -> `ok internal/job/lint 0.137s`. (The app walk tests are for app lanes.) |
| R3 | New tests fail on base, pass on lane | HOLDS | see Red on base; pass lines in rows 1-2. |
| R4 | No existing test edited or deleted | HOLDS | `git diff --stat 44c7de5b..HEAD` shows only the six files below; no existing test file touched. |
| R5 | No guard, gate, cap, hold or hook on agents | HOLDS | Nothing is refused or held. The change only picks exit 4 for an error that already failed, and prints one extra stderr line for a permission or read-only error. exitStatusHelp is unchanged. |
| R6 | *RealPlugin* tests | not run | none named by the brief. |
| R7 | gofmt, commit identity, no attribution, no personal paths | HOLDS | `gofmt -l internal/cli` prints nothing. Two commits by the lane identity, one-sentence messages, no Co-Authored-By. Repo-relative paths only. |
| R8 | Budget | OVER, by a little | About 60 tool calls against "about 50". No work-in-progress bundle was written at 60 percent; the lane was one change and was finished in one pass. |

## What changed

- `internal/cli/exitcode.go`: `exitCode` gives 4 for `feed.ErrDiskFull` or `qwDiskFull(err)`. New `qwEnFinishExit(exit, recorded)` (exit 1 plus a recorded error that `exitCode` maps to 4 becomes 4; every other exit unchanged), `qwEnUnwritable(err)` (`fs.ErrPermission` or `syscall.EROFS`) and the constant `qwEnHomeLine`.
- `internal/cli/qw_disk_full_unix.go` (`!windows`): `qwDiskFull` is `errors.Is(err, syscall.ENOSPC)`.
- `internal/cli/qw_disk_full_windows.go`: `qwDiskFull` is ENOSPC, `syscall.Errno(112)` (ERROR_DISK_FULL) or `syscall.Errno(39)` (ERROR_HANDLE_DISK_FULL). No golang.org/x/sys.
- `internal/cli/error_codes.go`: `commandFailure` keeps the last recorded error (`err`) and a `homeLine` flag. `recordError` also writes `grid: the Grid home cannot be written` to `e.Stderr` once, before the command's own line, when the error is `qwEnUnwritable`.
- `internal/cli/cli.go:198-202`: the deferred finish sets `exit = qwEnFinishExit(exit, f.err)` before `f.finish`. It prints no line of its own.
- `internal/cli/qw_disk_full_test.go`: the five tests above.
- `docs/lanes/C-qw-enospc-exit-four.md`: this report, the one file outside Touches, as the runbook asks.

## Open items (notes for the backlog, nothing blocks the rows)

1. Reach. Exit 4 for a non-Feed write arrives only where a command calls `recordError` under `--json`, or returns `exitCode(err)`. The brief says every command calls `recordError`; `grep -rn "recordError(" internal` finds it in 14 files (task, task_write, task_lane, feed, plan, deliver, usage, artifact, projects, liveness, orchestrator, workflow, task_operand, cli) and none in the question, brain, signal or intent paths. Those keep exit 1 on a full disk unless they return `exitCode(err)` (intent, feed, workflow, orchestrator, agent do). Adding `recordError` there is outside Touches.
2. Without `--json`, `env.failure` is nil, so `grid task summary` and the other task writers still exit 1 on a full disk and print no home line. Both effects need `--json`, which is how the app calls.
3. The home line fires for any recorded `fs.ErrPermission` or `EROFS`, including a failed read. It also becomes the start of the `--json` envelope message when the error has a code.
4. On Windows a write-protected volume is ERROR_WRITE_PROTECT (19), not EROFS; it is not matched, because the brief names `fs.ErrPermission` or `syscall.EROFS`. ERROR_ACCESS_DENIED is `fs.ErrPermission` and is matched.
5. Naming: `qwErrorDiskFull` and `qwErrorHandleDiskFull` (qw_disk_full_windows.go) lack the `qwEn` prefix. Cosmetic.
6. Landing, as the brief says: the app reads exit 4 as 502 until C-qx-disk-full-words words it.

## NOT VERIFIED on a live screen

Nothing here is a screen. NOT run: a real full disk, a read-only mount or a Windows or macOS host. The mapping is proved on constructed errors, and the Windows file is proved only by the cross vet and test compile.
