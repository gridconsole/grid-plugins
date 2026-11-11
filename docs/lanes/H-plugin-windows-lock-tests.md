# H-plugin-windows-lock-tests: the lock tests of localdata and the brain also build and run for Windows

Both `lock_unix_test.go` files held only portable code behind a `//go:build linux || darwin` tag. They are renamed to `lock_test.go` and the tag is removed. No test body changed. Lane head: one commit on top of main.

## Proof table

| Row | Result |
|---|---|
| 1. Each test, and what in it is unix-only | HOLDS. Table below: every test is portable; the one OS-dependent call is `os.Symlink` at `internal/localdata/lock_test.go:117`, which already skips when symlinks are unavailable. |
| 2. Portable tests carry no build tag | HOLDS. `grep -rn "go:build" internal/localdata internal/brain` finds nothing. `GOOS=windows go list -f '{{.ImportPath}} {{.TestGoFiles}}' ./internal/localdata ./internal/brain` now lists `lock_test.go` in both (on base it listed neither `lock_unix_test.go`; localdata showed only `[localdata_test.go]`). |
| 2. `GOOS=windows go vet ./internal/localdata/... ./internal/brain/...` | HOLDS. exit 0, no output. |
| 2. `GOOS=windows go test -c -o /dev/null ./internal/localdata` and `./internal/brain` | HOLDS. Both exit 0, no output. |
| 2. Command for a Windows host | Below, under "Windows host command". Not run here: no Windows host in the sandbox. |
| 3. `go test ./internal/localdata/... ./internal/brain/... -count=1` on Linux | HOLDS. exit 0: `ok .../internal/localdata 0.277s`, `ok .../internal/brain 2.090s`. |
| 3. `GOOS=darwin go vet ./internal/localdata/... ./internal/brain/...` | HOLDS. exit 0, no output. |
| 4. No test deleted | HOLDS. `git diff -M --stat origin/main..HEAD`: two renames, 4 deletions in total, all of them the tag line and the blank line after it. Nothing moved to a tagged file; no `_unix_test.go` was needed. |
| New tests fail on base and pass on the lane | There are no new test functions. The same 9 tests (table below) were not in the Windows test binary on base and are in it now (`go list` lines above). The base-side "fail" is that absence, because a Windows test binary cannot be run on the VM. Running them on Windows is NOT VERIFIED here. |
| Tests of changed packages and of every importer | HOLDS. `go test -count=1` over localdata, brain and the 30 other packages that import either one, directly or through another package, exit 0, every package ok. The list came from `go list -f '{{.ImportPath}}\|{{join .Deps " "}} {{join .TestImports " "}}' ./...` filtered for `internal/localdata` and `internal/brain`: cmd/grid, braincmd, cli, env, feed, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, orchestrator, packaging, packaging/cmd, people, ports, projectmd, projects, proof, question, routine, signal, task, usage, worker, workflow. |
| `make lint` | HOLDS. exit 0 (`go vet ./...`, `gofmt -l .` empty). `gofmt -l internal/localdata internal/brain` also exit 0. |
| Windows and macOS builds | HOLDS. `make cross` exit 0: `go vet ./...` and a test compile of every package for darwin/arm64 and windows/amd64. |
| No guard, gate, cap, hold or blocking hook added | HOLDS. The diff is two renames and four deleted lines in test files. |
| One-sentence commits, no attribution lines, no personal paths | HOLDS. `git log --format='%h %s%n%b' origin/main..HEAD` shows one commit, one sentence, no trailer. |
| Edit only the named packages; existing tests unchanged | HOLDS. Only `internal/localdata/lock_test.go` and `internal/brain/lock_test.go` (and this report) changed. `internal/fslock`, `localdata.go`, `store.go`, `internal/cli` and `internal/env` are untouched. |
| UI rows | None in this brief. |

## Row 1: the tests and what is unix-only

| Test or helper | Unix-only | Note |
|---|---|---|
| `internal/localdata/lock_test.go:17` `TestLockHelperProcess` | portable | `Lock`, `os.WriteFile`, `time.Sleep`; the parent kills it. |
| `:30` `waitUntil`, `:42` `lockWithin` | portable | helpers. |
| `:65` `TestKilledWriterDoesNotBlockTheNextWrite` | portable | `exec.Command(os.Args[0])` and `Process.Kill` work on Windows (TerminateProcess). `0o600` is ignored there. |
| `:85` `TestLeftoverLockFileDoesNotBlock` | portable | |
| `:93` `TestLockSerializesWriters` | portable | eight goroutines, each with its own handle on the same file. |
| `:115` `TestLockRefusesSymlink` | `:117` `os.Symlink` | Windows needs a privilege or Developer Mode for symlinks. The test already calls `t.Skip` when the call fails, so it skips there and does not fail. |
| `internal/brain/lock_test.go:18` `TestLockHelperProcess` | portable | modes `propose` and `hold`. |
| `:47` `touch`, `:55` `start`, `:72` `waitFor`, `:83` `exists` | portable | helpers. |
| `:85` `TestTwoProcessesProposeAtOnce` | portable | |
| `:119` `TestKilledWriterLeavesNoStaleLock` | portable | `Process.Kill` as above. |
| `:137` `TestLeftoverLockFileDoesNotBlock` | portable | |

Searched both files for `syscall`, `/proc` and `Signal`: none. `internal/testenv.Temp` already sets `TMP` and `TEMP` for the child, so the helper process gets its own temp directory on Windows.

## Windows host command

Run from the repo root on a Windows host:

```
go test -count=1 -v -run "TestLockHelperProcess|TestKilledWriter|TestLeftoverLockFile|TestLockSerializesWriters|TestLockRefusesSymlink|TestTwoProcessesProposeAtOnce" ./internal/localdata ./internal/brain
```

That pattern selects exactly the 9 tests this lane moved. Expected: `PASS` for `TestKilledWriterDoesNotBlockTheNextWrite`, `TestLeftoverLockFileDoesNotBlock` and `TestLockSerializesWriters` (localdata), and for `TestTwoProcessesProposeAtOnce`, `TestKilledWriterLeavesNoStaleLock` and `TestLeftoverLockFileDoesNotBlock` (brain); `SKIP` for `TestLockHelperProcess` in both packages (it only runs as the child); `TestLockRefusesSymlink` passes with symlink rights and skips without them. On Linux the same command printed those results.

## What changed

- `internal/localdata/lock_unix_test.go` renamed to `internal/localdata/lock_test.go`; the `//go:build linux || darwin` line and the blank line after it removed.
- `internal/brain/lock_unix_test.go` renamed to `internal/brain/lock_test.go`; same two-line removal.

## NOT VERIFIED

- The tests have not run on Windows. The VM can vet and compile them for Windows, not run them. The orchestrator's Windows host command above is the proof that two processes exclude each other through `fslock` on Windows, and that `brain.locked` (`internal/brain/store.go:147`, which opens the lock through `os.Root` before `fslock.LockExclusive`) behaves the same way.
- `TestLockRefusesSymlink` skips on a Windows host without symlink rights, so on such a host the Windows symlink refusal in `localdata.Lock` is covered only by `internal/fslock/fslock_windows_test.go:400`, not by this lane.

## Open items

None that block. If the Windows run shows a failure, it is a real Windows bug in `fslock`, `localdata.go` or `store.go`; this lane does not touch those, so write it down as NOT DONE with the failing line.
