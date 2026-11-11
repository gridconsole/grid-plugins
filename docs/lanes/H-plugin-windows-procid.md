# H-plugin-windows-procid: process identity and the live-process probe on Windows

Repo plugin, branch lane-H-plugin-windows-procid, from main after F-processid-darwin.

## Proof table

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1a | `processid.Start(pid)` on Windows returns the creation time in the text format the Unix side writes, "" for a pid that does not exist | HOLDS (code; runs on Windows only) | `internal/processid/start_windows.go:53-59` formats `CreationTime` with `LstartLayout`, the `ps -o lstart=` line darwin records, in UTC at second precision; `CreationTime` returns `ErrGone` for pid < 1, for ERROR_INVALID_PARAMETER and for an exited process (`start_windows.go:28-47`), and `Start` turns every error into "". Tests: `TestStartCurrentProcessWindows`, `TestStartRejectsNonPositivePIDWindows`, `TestStartAbsentPIDWindows`, `TestStartChildLivesAndExitsWindows` (`start_windows_test.go`). |
| 1b | `session.ProcessStart` uses it | HOLDS (code) | `internal/session/probe_windows.go:16-25`: `processid.CreationTime`, truncated to the second like the ps read; pid < 1 is an error. Test `TestProcessStartWindows`. |
| 1c | `ProbeLocal`: Alive for the current process with its own start; Dead for a gone pid or a matching pid with a different start; Unknown only when OpenProcess is denied | HOLDS (code; fake-host cases also compiled, run on Windows) | `probe_windows.go:41-66`: `ErrGone` gives Dead, any other read error (the OpenProcess denial, ERROR_ACCESS_DENIED) gives Unknown, equal start at second precision gives Alive, different gives Dead. A `proc:` token or an unparseable token on a live pid is Unknown, as on Unix. Tests `TestProbeLocalWindows`, `TestProbeLocalGonePIDWindows`, `TestProbeLocalFakeHostWindows` (`probe_windows_test.go`, includes a denied row `syscall.Errno(5)` giving Unknown and an exited-with-open-handle child giving Dead). |
| 1d | build tags of `start_other.go` and `probe_other.go` exclude Windows | HOLDS | `start_other.go:1` is `!unix && !windows`; `probe_other.go:1` is `!linux && !darwin && !windows`. `GOOS=windows go build ./cmd/grid` exit 0 (no duplicate `Start`/`ProbeLocal`). |
| 2 | `GOOS=windows go vet ./internal/processid/... ./internal/session/...` and `GOOS=windows go test -c -o /dev/null ./internal/processid ./internal/session`: 0 fail | HOLDS | all three commands exit 0 with no output. `make cross` also compiles every package test for windows/amd64 and darwin/arm64: exit 0. |
| 2b | New tests fail on base | HOLDS | On a detached `origin/main` with only the two new test files added, `GOOS=windows go vet ./internal/processid/... ./internal/session/...` exits 1: `vet: internal/processid/start_windows_test.go:68:18: undefined: CreationTime` and `vet: internal/session/probe_windows_test.go:160:71: undefined: processid.ErrGone`. They cannot run on Linux, so base fails at compile; the runtime failure on base would be `ProbeLocal` returning Unknown. |
| 2c | Command for a Windows host | HOLDS | On a Windows host with the repo at this head and Go 1.27: `go test ./internal/processid/... ./internal/session/... -count=1 -v`. Then the importers: `go test ./internal/migrate/... ./internal/ports/... ./internal/cli/... -count=1`. Expect PASS for every `*Windows` test; the helper `TestHelperHoldWindows` is a child process and passes by returning at once. |
| 3 | Linux and darwin unchanged | HOLDS | `go test ./internal/processid/... ./internal/session/... -count=1`: ok both, exit 0. `GOOS=darwin go vet ./internal/processid/... ./internal/session/...` exit 0. No `_unix.go` file edited (`git diff origin/main --stat` lists only the two `_other.go` tag lines and the new files). |
| 4 | `GOOS=windows go build ./cmd/grid` and `go vet ./...`: 0 fail | HOLDS | `GOOS=windows go build ./cmd/grid` exit 0; `make lint` (`go vet ./...` plus `gofmt -l .`) exit 0. |
| R1 | Packages changed and their importers tested | HOLDS | `go list` importers of processid or session: cli, hook, lease, migrate, orchestrator, ports, task. `go test ./internal/processid/... ./internal/session/... ./internal/cli/... ./internal/hook/... ./internal/lease/... ./internal/migrate/... ./internal/orchestrator/... ./internal/ports/... ./internal/task/... -count=1`: all nine ok, exit 0. |
| R2 | Lint, Windows and macOS builds | HOLDS | `make lint` exit 0; `make cross` (vet and test-compile for darwin/arm64 and windows/amd64) exit 0. |
| R3 | No new module | HOLDS | only the `syscall` package; `go.mod` unchanged. |
| R4 | No guard, gate, cap, hold or blocking hook added | HOLDS | the change reads a process and returns a state; it blocks nothing. |
| R5 | No existing test edited or deleted | HOLDS | only new test files; `git diff origin/main --stat` shows no existing `_test.go`. |
| R6 | Small commits, one-sentence messages, no attribution, no personal data | HOLDS | two commits (processid, session); no trailers; no paths, hosts or emails in the files. |
| R7 | Edited only the allowed files | HOLDS | new `start_windows.go`, `probe_windows.go` and their tests; the two tag lines (plus the doc comment above each) in `start_other.go` and `probe_other.go`. Not `probe_unix.go`, `start_unix.go`, lease, fslock, `cli/worker.go` or `cli/orchestrator.go`. |

## What changed

- `processid.CreationTime(pid)` (Windows only): OpenProcess with PROCESS_QUERY_LIMITED_INFORMATION, GetProcessTimes, creation time in UTC. `ErrGone` for no process, including a process that has exited while another handle to it is open (exit time set). Any other error, such as a denied OpenProcess, is returned wrapped.
- `processid.Start(pid)` on Windows is that time in `LstartLayout` (the darwin text, "Tue Nov  3 09:15:30 2026"), UTC, second precision. Always UTC, so the text does not depend on the zone setting; `ParseLstart(token, time.UTC)` reads it back.
- `session.ProcessStart` (Windows) is `CreationTime` truncated to the second, so `contract.FormatTime` of it matches the Unix `.000Z` form. `worker.go` and `task_lease.go` pass such an RFC3339 token.
- `session.ProbeLocal` (Windows) takes an RFC3339 token or the UTC lstart line. Different start at second precision is Dead; a process that cannot be read is Unknown; a token it cannot read is Unknown.
- `processid.CreationTime` and `ErrGone` exist only in the Windows build; no other package calls them.

## Open items

- `internal/cli/task_lease.go:73` refuses `lease take` when `runtime.GOOS` is not linux or darwin. That file belongs to G-leases-remove, so I left it; once the lease code is gone, the Windows probe here is what `worker.go:129` and `orchestrator.go:513` use.
- `internal/migrate/process_windows.go` still records the creation time as FILETIME nanoseconds in decimal, a different text from `processid.Start`. Out of this lane's files; `migrate` does not call `ProbeLocal`.
- `probe_other.go` still returns the "task lease needs a Unix host" error for hosts other than Linux, macOS and Windows (for example FreeBSD); this lane did not change that text.

## NOT VERIFIED on a live screen / host

- No Windows host here: the `*Windows` tests compile and vet but have not run. The OpenProcess and GetProcessTimes behaviour (ERROR_INVALID_PARAMETER for an absent pid, exit time set for a zombie handle, ERROR_ACCESS_DENIED for a protected process) is from the Win32 documentation and needs the Windows run in row 2c.
- A real access-denied process (for example a protected system process from a non-elevated shell) is covered only by the fake-host row.
