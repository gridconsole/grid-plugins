# A-win-feed-platform: the Feed opens, posts and syncs on Windows

Base: main at 78b08bc9. Lane branch: lane-A-win-feed-platform.

## What changed

- `internal/feed/platform_windows.go` (new, `//go:build windows`): `syncFile` skips a folder and otherwise calls `f.Sync()`; `checkFilesystem` makes the path absolute, asks `GetDriveTypeW` (lazy kernel32 proc, as `internal/fslock/fslock_windows.go:19-23`) for the volume root and returns `driveTypeRefusal` of the answer; `writeOnce` is `f.Write`.
- `internal/feed/platform_drive.go` (new, untagged): `driveTypeRefusal(driveType uint32) error`. 4 (DRIVE_REMOTE) gives "Feed requires a local filesystem" (same text as `platform_linux.go:18`); every other value gives nil.
- `internal/feed/platform_other.go:1`: build tag is now `!linux && !darwin && !windows`.
- `internal/feed/platform_lane_test.go` (new, runs everywhere) and `internal/feed/platform_windows_test.go` (new, Windows only).
- No change to store.go, identity.go, gate.go, cache.go, fold.go, internal/orchestrator, internal/lease, task_lease.go or go.mod. No existing test edited or deleted.

## Proof table

| Row / rule | Result |
|---|---|
| 1 `syncFile`: folder gives nil without a flush, else `f.Sync()` | HOLDS (compiled, platform_windows.go:18-27): `if info.IsDir() { return nil }` at :23-25, `return f.Sync()` at :26. `GOOS=windows go test -c -o /dev/null ./internal/feed/` exit 0 |
| 1 `writeOnce` is `f.Write(data)` | HOLDS (compiled, platform_windows.go:45): `func writeOnce(f *os.File, data []byte) (int, error) { return f.Write(data) }` |
| 1 `checkFilesystem`: Abs, volume root, `GetDriveTypeW`, `driveTypeRefusal` | HOLDS (compiled, platform_windows.go:31-43): `filepath.Abs(path)` :32, `filepath.VolumeName(abs) + "\"` :36, `procGetDriveTypeW.Call` :40, `return driveTypeRefusal(uint32(kind))` :42. A UNC path keeps its `\\server\share` volume name, so the root is `\\server\share\`, which Windows reports as DRIVE_REMOTE; that answer is NOT VERIFIED without a host (below) |
| 1 `driveTypeRefusal` 0 to 6: only 4 errors, with that text | HOLDS: platform_lane_test.go:7-28, platform_drive.go:12-17. `go test ./internal/feed/ -run WinLane -count=1 -v` on Linux: `--- PASS: TestWinLaneDriveTypeRefusal`, exit 0 |
| 2 `GOOS=windows go list -f '{{.GoFiles}}' ./internal/feed/` | HOLDS, exit 0: `[cache.go fold.go gate.go guard.go identity.go platform_drive.go platform_windows.go questions.go read.go record.go store.go]` (platform_windows.go, not platform_other.go) |
| 2 `GOOS=freebsd` | HOLDS, exit 0: `[cache.go fold.go gate.go guard.go identity.go platform_drive.go platform_other.go questions.go read.go record.go store.go]` |
| 2 `GOOS=linux` | HOLDS, exit 0: `[cache.go fold.go gate.go guard.go identity.go platform_drive.go platform_linux.go questions.go read.go record.go store.go]` (neither file) |
| 2 `GOOS=darwin` | HOLDS, exit 0: `[cache.go fold.go gate.go guard.go identity.go platform_darwin.go platform_drive.go questions.go read.go record.go store.go]` (neither file) |
| 3 Post of a message, Read returns it, second Post gets seq 2 | HOLDS (compiled, platform_windows_test.go:21-46, TestWinLanePostAndRead): first Post :24-26, Read :28-34, second Post :35-38. Reuses testStore, message, mustPost, testWriter from store_test.go unedited |
| 3 `CheckFilesystem(t.TempDir())` is nil | HOLDS (compiled, platform_windows_test.go:48-52, TestWinLaneCheckFilesystemLocal) |
| 3 `syncFile` on a folder from `os.Open` is nil | HOLDS (compiled, platform_windows_test.go:54-63, TestWinLaneSyncFolderIsNoop) |
| 3 `syncFile` on a written temp file is nil | HOLDS (compiled, platform_windows_test.go:65-79, TestWinLaneSyncFile) |
| 4 `GOOS=windows go vet ./internal/feed/ ./internal/lease/ ./internal/cli/` | HOLDS: exit 0, no output |
| 4 `GOOS=windows go test -c -o /dev/null ./internal/feed/` | HOLDS: exit 0, no output |
| 5 `go test ./internal/feed/ -count=1` | HOLDS: `ok internal/feed 7.005s`, 0 fail |
| 5 `GOOS=darwin go vet ./internal/feed/` | HOLDS: exit 0, no output |
| 5 `make lint` | HOLDS: exit 0 (it runs `go vet ./...`, prints that line only) |
| Tests of every importer (go list, direct, test and xtest imports): cli, hook, job/jobcmd, lease, mcp, migrate, orchestrator, question, task, usage | HOLDS: `go test -count=1` on feed plus those ten: all `ok` (cli 34.8s, orchestrator 32.0s, hook 4.9s, migrate 6.8s, task 3.4s, question 2.4s, jobcmd 1.7s, lease 1.2s, mcp 0.7s, usage 0.02s), exit 0 |
| Windows and macOS builds | HOLDS: `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| New tests fail on base, pass on the lane | HOLDS for the portable test. On origin/main plus the two new test files, `go test ./internal/feed/ -run WinLane -count=1` exit 1: `internal/feed/platform_lane_test.go:13:10: undefined: driveTypeRefusal`, `internal/feed/platform_lane_test.go:25:13: undefined: driveTypeRefusal`, `FAIL ... [build failed]`. On the lane the same command passes. The Windows-only tests cannot run here. On base a Windows Post fails at `store.go:66` because `platform_other.go:11` returns "Feed is supported only on Linux and macOS" |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS: nothing of the kind added. The only refusal is the network-volume refusal the Linux and macOS twins already make |
| Commits one sentence, no attribution lines | HOLDS: `git log origin/main..lane-A-win-feed-platform` shows two one-sentence commits plus this report, no trailers |
| Public repo: no personal paths, hosts or emails | HOLDS: this file and the new code name only repo paths |
| Bug outside `platform_*.go` | NOT DONE rows: none found by reading. One item to watch is under NOT VERIFIED |

Windows host run: for the orchestrator

```
go test ./internal/feed/ -run WinLane -count=1 -v
```

## NOT VERIFIED on a live screen

Nothing here touches a screen. Nothing here has run on a Windows host either. These need that run:

- `GetDriveTypeW` answers 4 for a mapped drive and a UNC share, and 3 for a local NTFS temp folder. The code relies on the documented answers; the refusal is proved only for the number.
- `FlushFileBuffers` on the append handle that `fslock.OpenNoFollow` opens. `internal/fslock/fslock_windows.go:121-123` replaces GENERIC_WRITE with FILE_APPEND_DATA and related rights for `O_APPEND`, so `syncFile` at `store.go:290` and `:376` flushes a handle without FILE_WRITE_DATA. `TestWinLanePostAndRead` covers it: if Post fails on the host with an access error, the fix is in fslock_windows.go, not in `platform_*.go`, and this lane is NOT DONE for that row.
- `os.Link` at `store.go:540` publishes a body file by hard link. It needs NTFS (or ReFS). `TestWinLanePostAndRead` posts a plain message and may not reach `saveBody`; a body-carrying Post on a host is the proof. A FAT or exFAT volume reports DRIVE_FIXED or DRIVE_REMOVABLE and is not refused.

Process note: the work-in-progress bundle at call 35 was not written; the work was already committed and the final bundle follows.
