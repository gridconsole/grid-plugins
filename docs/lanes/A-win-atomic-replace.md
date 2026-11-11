# A-win-atomic-replace: atomicfile and the session records survive a reader holding the file open on Windows

Base: main 78b08bc9 (lane-Pint52 merge head the clone started from). Branch lane-A-win-atomic-replace. Commits: the retry in atomicfile, the session call, the tests, this file.

## Proof table

| Row | Status | Evidence |
|---|---|---|
| 1. rename_windows.go retries only on 32, 5 and 33; a few short pauses under one second; attempt count and pauses named constants in one place; returns the last error; loop and retry test injectable | HOLDS (compiled, rename_windows.go:21-23) | `retryableRename` matches `errors.Is(err, syscall.Errno(5))`, `Errno(32)` and `Errno(33)` and nothing else (rename_windows.go:22). `renameAttempts = 6` and `renamePause = 50 * time.Millisecond` are the only constants (rename.go:15-16); pauses are 50, 100, 150, 200, 250 ms, 750 ms in all, and the comment above them says so. `retryRename(rename, retryable, pause)` (rename.go:22) takes all three as arguments and returns the last error; the unit tests pass fakes for them on Linux. `GOOS=windows go vet ./internal/atomicfile/ ./internal/session/`: exit 0. |
| 2. Portable TestAtomicLane tests | HOLDS | `go test ./internal/atomicfile/ -run AtomicLane -count=1 -v`: 12 PASS, exit 0. Fails twice then succeeds: nil after 3 calls and 2 pauses (replace_lane_test.go:19). Non-retryable: returned at once after 1 call, no pause (:37). All attempts fail: `renameAttempts` calls, the last error comes back, pauses total above 0 and under 1 s (:46). `Rename` over an existing file replaces it and leaves no source (:64). The Unix path does not retry (:84, skipped on Windows). |
| 3. replace_lane_test.go: replace branches, Write and WriteRoot refusals | HOLDS | Closed `*os.File`: the write error (`os.ErrClosed`), rename and syncDir not called (:104). Failing rename: its error, syncDir not called (:119). Failing syncDir: its error, order `rename,syncDir` (:132). Data written (:145). Write over a non-empty folder: error, folder and its file unchanged, only `target` in the folder, no temp file (:156). Existing 0o640 mode kept, `t.Skip` on Windows (:180). WriteRoot refuses `../x` ("must be relative", nothing beside the root) and a folder ("not a regular file", folder and root unchanged) (:200). All in the `go test` run in row 5, exit 0. |
| 4. rename_windows_lane_test.go (a) reader closes after 50 ms, (b) reader held until Write returns | HOLDS (compiled, rename_windows_lane_test.go:37 and :71) | (a) :37-66: a goroutine reads the handle in full, closes it at 50 ms (:46); asserts Write nil, the handle read the old bytes in full (:58), the file holds the new bytes in full (:61), no temp file (:64). (b) :71-93: Write returns nil (:78, new bytes whole at :80) or an error matching one of the three errnos (:83, old bytes whole at :85); anything else fails; each case logs which happened; no temp file left (:91). `GOOS=windows go vet ./internal/atomicfile/ ./internal/session/`: exit 0. `GOOS=windows go test -c -o /dev/null ./internal/atomicfile/`: exit 0. `GOOS=windows go test -c -o /dev/null ./internal/session/`: exit 0. |
| 5a. session/files.go:82 calls atomicfile.Rename | HOLDS | internal/session/files.go:82 `return atomicfile.Rename(f.Name(), path)` (it was :81 on base; the new import is line 10). |
| 5b. `go test ./internal/atomicfile/ ./internal/session/ -count=1` | HOLDS | exit 0: `ok .../atomicfile 0.007s coverage: 84.6%`, `ok .../session 0.132s coverage: 79.2%`. |
| 5c. coverage figures | HOLDS | `go test -cover ./internal/atomicfile/`: 84.6% (above 80%; base figure in the brief 76.8%). `go tool cover -func`: `retryRename 100.0%`, `renameRetrying 100.0%`, `Rename 100.0%` (unix), `replace 64.3%` (above 60%; base 42.9%), `Write 100.0%`, `WriteRoot 85.2%`. |
| 5d. Branches not covered, no fake file added | HOLDS (named) | The chmod, sync and close failure returns of a real file in `replace` (atomicfile.go:78-88) are not covered: they cannot be forced on a real `*os.File` without a fake file, and none was added. |
| 5e. `GOOS=darwin go vet ./internal/atomicfile/ ./internal/session/` | HOLDS | exit 0. |
| 5f. `make lint` | HOLDS | exit 0 (go vet ./... and gofmt -l clean). |
| New tests fail on base | HOLDS for the retry tests, see note | On base (tests added, no implementation): `go test ./internal/atomicfile/ -count=1` printed `replace_lane_test.go:22:9: undefined: retryRename`, `:53:14: undefined: renameAttempts`, `:73:12: undefined: Rename`, `:89:9: undefined: renameRetrying`, `FAIL ... [build failed]`. The replace, Write and WriteRoot tests (row 3) describe behaviour that already holds on base, so they pass there once the package compiles; I did not run them apart from the rest on base. The two Windows tests fail on base only on a Windows host with a reader held open; they cannot run here. |
| Packages changed and their importers | HOLDS | Changed: internal/atomicfile, internal/session. `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...` filtered on both gives these importers, all run with `go test -count=1`, exit 0, every line `ok`: account, brain, cli, doctor, feed, hook, lease, localdata, migrate, orchestrator, ports, projectmd, projects, proof, question, routine, settings, signal, task, usage, workflow (all under internal/); atomicfile and session ran in row 5b. |
| Windows build | HOLDS | `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` exit 0 |
| macOS build | HOLDS | `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./...` exit 0 |
| No existing test edited or deleted | HOLDS | `git diff origin/main..HEAD --stat` lists only new files, atomicfile.go, session/files.go. |
| Only the packages and files the brief names touched | HOLDS | internal/atomicfile (rename.go, rename_unix.go, rename_windows.go, atomicfile.go, two new test files), internal/session/files.go, this file. go.mod unchanged. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The retry is a bounded wait inside one file replace, in the caller's own process; nothing blocks or limits an agent, and no hook was added. |
| No attribution lines, no personal paths, hosts or emails | HOLDS | Commit messages are one sentence with no trailers; no file names a person, host or home path. |
| Other bare renames: out of scope, listed only | HOLDS (listed) | `git grep -n 'os.Rename(' -- internal/ ':!*_test.go'` now gives 23 hits; 2 are the new Rename in atomicfile, 21 are untouched: artifact/artifact.go:382, eval/results.go:29, intent/files.go:189, job/jobcmd/save.go:68, migrate/apply.go:221, :227, :229, migrate/recover.go:214, :223, migrate/rollback.go:236, :241, plan/store.go:132, settings/budget.go:810, settings/claude.go:433, :443, settings/claude_user.go:264, signal/move.go:127, task/create.go:142, :414, task/trash.go:361, version/check.go:168. |
| Nothing pushed | HOLDS | Bundle only. |

Windows host run: for the orchestrator. `go test ./internal/atomicfile/ -run 'AtomicLane|WinLane' -count=1 -v`

## What changed

- `internal/atomicfile/rename.go` (new, untagged): `renameAttempts`, `renamePause`, `retryRename` (the loop, with the rename, the retry test and the pause as arguments) and `renameRetrying` (the loop with this OS's retry test and `time.Sleep`).
- `internal/atomicfile/rename_unix.go` (new): `Rename` is `os.Rename`; `retryableRename` is false.
- `internal/atomicfile/rename_windows.go` (new): `Rename` goes through the loop; `retryableRename` accepts `syscall.Errno` 5, 32 and 33.
- `internal/atomicfile/atomicfile.go`: `Write` renames with `Rename`; `WriteRoot` wraps its `dir.Rename` in `renameRetrying`, so both use the one loop. A line was added to Write's doc comment.
- `internal/session/files.go`: `atomicJSON` renames with `atomicfile.Rename`; the session and heartbeat records get the retry.
- `internal/atomicfile/replace_lane_test.go`, `rename_windows_lane_test.go` (new): the tests above.

## Notes

- A rename that fails with error 5 for a real reason (a read-only folder, a target that is a folder) also waits the 750 ms before returning its error; the error is the last one, unchanged. On Windows the folder case in `TestAtomicLaneWriteOverNonEmptyFolderFails` therefore takes about 0.75 s.
- The retry cannot outlast a reader that never closes; row 4(b) accepts that.
- 21 other bare renames remain (list above) and still lose the write on Windows when a reader holds the file; the brief puts them out of scope.

## NOT VERIFIED on a live screen

- Nothing here is a UI row. Nothing was run on Windows or macOS: the Windows tests are compiled and vetted from the VM only, so what Go's own share mode does with a held-open file, and which of the two outcomes test (b) logs, is for the Windows host run above.
