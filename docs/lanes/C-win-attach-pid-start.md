# C-win-attach-pid-start

Repo: plugin. Base: main 55c46cbd (`55c46cbdeab67b5afcfcb42806c49caf2a12386e`). Code head: f3957c36 (this report is committed on top).

## Proof table

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | Test fails on main, passes on lane | HOLDS | On main the new `internal/session/token_lane_test.go` does not build, a compile failure and not an assertion, because `tokenTime` sits in a windows-only file. `go test ./internal/session/ -run Token` on base 55c46cbd (test file added, no other change): `internal/session/token_lane_test.go:20:16: undefined: tokenTime` (also :29:13, :48:15, :64:17), then `FAIL github.com/gridconsole/grid-plugins/internal/session [build failed]`. On the lane the same command passes: `TestTokenTimeBareNanosecondsIsNotRead`, `TestTokenTimeLstartLineIsRead` PASS, exit 0. The integer token is `strconv.FormatInt(tokenFixed.UnixNano(), 10)` from the fixed `tokenFixed` (2026-11-03 09:15:30.123456789 UTC): not ok. The `Format(processid.LstartLayout)` token: ok, equals `tokenFixed.Truncate(time.Second)`. Row 3's first grep on main printed `internal/migrate/process_windows.go:12:	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))` and `internal/migrate/process_windows.go:21:	return strconv.FormatInt(created.Nanoseconds(), 10)`, the integer token being written. |
| 2 | RFC3339Nano reads back at second precision; "", "proc:1:2", "garbage" not ok; function moved verbatim | HOLDS | `TestTokenTimeRFC3339NanoReadsAtSecondPrecision` (utc and +01:00 offset cases, equal to the truncated instant, location UTC) and `TestTokenTimeUnreadableTokensAreNotOK` pass: `go test ./internal/session/ -run Token -v`, exit 0. Verbatim move, `git diff main -- internal/session/probe_windows.go internal/session/token.go`: probe_windows.go `@@ -63,15 +63,3 @@` removes `// tokenTime reads a recorded start ... at second precision.` / `func tokenTime(token string) (time.Time, bool) { if t, err := time.Parse(time.RFC3339Nano, token); err == nil { return t.UTC().Truncate(time.Second), true }; if t, err := processid.ParseLstart(token, time.UTC); err == nil { return t, true }; return time.Time{}, false }`; new `internal/session/token.go:9-19` adds the same comment and body, character for character. `git diff main --stat`: probe_windows.go 12 deletions, token.go 19 insertions. probe_windows.go still calls it at :57. |
| 3 | `git grep -n "Nanoseconds\|PROCESS_QUERY_INFORMATION" -- internal/migrate/` prints nothing; `processid.Start` in process_windows.go prints one line | HOLDS | First grep: no output (exit 1, no match). Second grep: `internal/migrate/process_windows.go:11:	return processid.Start(pid)`, one line. |
| 4 | Row 3 is the proof of the migrate change; no host test for the Windows file, none claims a Windows run | HOLDS | No test added for `process_windows.go`; the only new test is `internal/session/token_lane_test.go` (untagged). No test claims a Windows run. |
| 5 | Windows vet and test compile | HOLDS | `GOOS=windows go vet ./internal/migrate/ ./internal/session/ ./internal/cli/ && GOOS=windows go test -c -o /dev/null ./internal/session/ && GOOS=windows go test -c -o /dev/null ./internal/migrate/ && echo ROW5_EXIT=0` printed `ROW5_EXIT=0`. |
| 6 | `go test ./internal/session/ ./internal/migrate/ ./internal/cli/ -run 'Attach\|Liveness\|Token\|ProcessStart'` 0 fail; `make lint` exit 0 | HOLDS | `ok` for session, migrate, cli, `ROW6_EXIT=0`. `make lint` (`go vet ./...`) printed `LINT_EXIT=0`; `gofmt -l internal/session internal/migrate` printed nothing. |
| R1 | Tests of changed packages and every importer; list pasted | HOLDS | `go list -f '{{.ImportPath}}: {{join .Deps " "}}' ./...` filtered to session or migrate gives (transitive importers): cmd/grid, internal/braincmd, cli, env, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, orchestrator, packaging, packaging/cmd, ports, signal, task. Direct importers: cli, hook, lease, orchestrator, task. Full (unfiltered) `go test` of session, migrate and all of these: all `ok` except `internal/session` `TestProbeLocal`, `probe_unix_test.go:28: context deadline exceeded` (see note below); `go test ./internal/session/ -count=1` alone then printed `ok`, `SESSION_EXIT=0`. |
| R2 | Windows and macOS builds | HOLDS | `GOOS=windows go build ./... && echo WIN_BUILD_EXIT=0` printed `WIN_BUILD_EXIT=0`. `GOOS=darwin go build ./... && echo DARWIN_BUILD_EXIT=0` printed `DARWIN_BUILD_EXIT=0`. |
| R3 | No existing test edited or deleted | HOLDS | `git diff main --stat`: 4 files, none a test but the new `token_lane_test.go`. |
| R4 | No guard, gate, cap, hold or blocking hook added | HOLDS | The diff is a token shape change and a function move; nothing blocks an agent. |
| R5 | Only the brief's files touched | HOLDS | process_windows.go, probe_windows.go, token.go, token_lane_test.go (and this report). Not probe_unix.go, processid, lease, task_lease.go, task_new_first_job.go, task/attach.go. |
| R6 | Commits: one sentence, no attribution | HOLDS | f5336f85 (tokenTime move and test), f3957c36 (migrate change), then the report commit. No Co-Authored-By or Claude-Session lines. |
| R7 | Public repo: no personal paths, hosts, emails | HOLDS | Fixed instant and generic names only in the test and in this report. |

## What changed

- `internal/migrate/process_windows.go`: `pidStart` is now `return processid.Start(pid)`. The Windows token is the UTC lstart line (or "" when the process is gone or unreadable), as the Unix side gets from the same helper. It opens with PROCESS_QUERY_LIMITED_INFORMATION and treats an exited process whose handle is held as gone, which the old body did not. The `strconv`/`syscall` body is gone.
- `internal/session/token.go` (new, untagged): `tokenTime` moved verbatim from `probe_windows.go`, so Linux and macOS builds compile and test it.
- `internal/session/token_lane_test.go` (new, untagged): pins which tokens `tokenTime` reads: the bare integer is not ok, the lstart line and RFC3339Nano read at second precision, "", "proc:1:2", "garbage" are not ok.
- Effect on Windows: `grid task attach` records a token `probeHost.probe` can read, so an attached terminal is Alive while the process lives and Dead when the pid is reused or gone. `grid task lease take` with a session id is no longer refused for an attached Windows terminal on that ground, once the session attaches with this binary. A session attached by the old binary keeps its integer token and stays Unknown until it attaches again, as the brief states.

## Notes

- `TestProbeLocal` (`internal/session/probe_unix_test.go:28`, a 2 s `ps` timeout) failed once when I ran twenty-odd packages' tests in one `go test` call, a load timeout on a Unix-only test the lane does not touch. Rerun alone: pass.
- `go vet` and gofmt do not flag `tokenTime` as unused on Linux where only the test calls it; no build tag added back, as the brief says.

## NOT VERIFIED on a live screen

- Not run on a real Windows host: `grid task attach` writing the lstart token, `probe` returning Alive for the attached shell, and `grid task lease take <session id>` accepting it. The Windows evidence is `GOOS=windows` vet, build and test compile (rows 3 and 5) and the untagged `tokenTime` tests on Linux. The orchestrator or a checker with a Windows host runs that.
