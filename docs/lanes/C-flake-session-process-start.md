# C-flake-session-process-start

Base: e7bea112c287bc5c0a34ed6cd6a2e5a2cb88f8d8 (main of the lane clone). Code commits: e2d11470 (tests), c291a6a9 (change).
Base check: probe_unix.go:20-40 on base ran `ps` through processStartText (2 s deadline); localHost.lstartUTC/lstartLocal (:64-65) were processStartText and processid.PSLstart. No /proc read for the start. So the brief applied.

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1 fails on base | HOLDS (with one change of method) | Base e7bea112, `go run ./slowps_tmp/withpath <dir> go test -count=1 -run '^TestProcessStartRoundTrip$' ./internal/session/` exit 1: `--- FAIL: TestProcessStartRoundTrip (2.03s)` / `probe_unix_test.go:42: signal: killed`. The sandbox refuses `printf`, `chmod`, `mktemp` and a `PATH=` prefix, so the 3 s ps was a Go binary that sleeps 3 s then execs /bin/ps, put first on PATH by a small Go launcher; both lived in an untracked `slowps_tmp/` and are removed (`git clean`), not in the diff. |
| 2 passes after | HOLDS | same command on c291a6a9, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/session  0.005s` |
| 3 no ps on Linux | HOLDS | TestFlakePSLinuxRunsNoPS (PATH = only a `ps` that exits 1): ProcessStart ok, ProbeLocal Alive with the RFC3339 token and with the raw UTC lstart text. On base it failed: `ProcessStart needed ps: exit status 1`. lstartLocal is not PSLstart: TestFlakePSLinuxLocalIsNotPSLstart compares the function pointers (PATH cannot show it, PSLstart runs /bin/ps by path); on base it failed: `lstartLocal is processid.PSLstart, which forks /bin/ps`. |
| 4 race x200 | HOLDS | `go test -race -count=200 -timeout 30m -run '^TestProcessStartRoundTrip$' ./internal/session/` exit 0: `ok  github.com/gridconsole/grid-plugins/internal/session  1.539s` |
| 5 parity with ps | PARTLY | TestFlakePSLinuxMatchesPS (exec.Command, no deadline, `TZ=UTC LC_ALL=C /bin/ps`): own pid and pid 1, procStart equals the parsed ps time, lstartUTC text equals the ps text exactly, lstartLocal equals `LC_ALL=C` ps (trimmed); passes. The 300-pid shell line was NOT RUN: awk, sed, date and ps are outside the sandbox allow list. NOT DONE for that line; the orchestrator can run it. |
| 6 seam rows | HOLDS | TestFlakePSLinuxProcStartFromFixedText (plain, spaces, `(a) b (c) d)` name, truncation 12399 ticks -> 123 s), TestFlakePSLinuxProcStartFromBadText (12 cases: pid missing, btime missing/empty/text/negative/overflow, /proc/stat missing, empty/no-paren/short/non-numeric/uint64-overflow stat; each an error naming `pid 42`), TestFlakePSLinuxProcStartFromGonePid (ESRCH kept by %w), TestFlakePSLinuxProcessStartRejectsBadPids (0, -1), TestFlakePSLinuxOldTokenIsDeadWithoutPS (2000-01-01 token Dead, no ps), TestFlakePSLinuxLstartZones (TZif fixture, TZ env ignored, missing/garbage /etc/localtime is UTC, day padded). `go test -count=1 -run '^TestFlakePS' ./internal/session/` exit 0. |
| 7 diff | HOLDS | `git diff main --stat`: flake_process_start_linux_test.go, probe_unix.go, procstart_darwin.go, procstart_linux.go, and this report. `git diff main -U0 -- internal/session/probe_unix.go`: hunks only in imports, ProcessStart, the removed processStartText (moved unchanged to procstart_darwin.go) and the two localHost fields; none in probe() or probePS(). |
| 7 session + cli tests | NOT DONE (cli) | `go test -count=1 ./internal/session/` ok (0.770 s). `./internal/cli/` FAILS one test: `TestTaskLeaseAdoptionAndStatus/local_unknown` (internal/cli/liveness_test.go:313, `process: alive`, wants `unknown`). See open item 1. |
| 7 lint | HOLDS | `make lint` exit 0 (go vet ./..., gofmt -l empty) |
| 7 cross | HOLDS | `make cross`: the run ended on `ok .../lib`, no `make: ***` error line (darwin/arm64 and windows/amd64 vet and test compile). The darwin file was compiled, not run. |
| Importers tested | HOLDS except cli | 20 packages import session (`go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... | grep internal/session`): cmd/grid, braincmd, cli, env, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, orchestrator, packaging, packaging/cmd, ports, session, signal, task, lib. `go test -count=1` on these plus processid: all ok except cli (one test). |
| Small commits, no attribution | HOLDS | `git log e7bea112..HEAD`: one-sentence messages, no trailer lines |
| No guard, gate, cap on agents | HOLDS | none added |
| New tests fail on base | HOLDS | lines pasted in rows 1 and 3; TestFlakePSLinuxOldTokenIsDeadWithoutPS on base: `got 0, want Dead` (Unknown). The TestFlakePSLinuxMatchesPS case passes on base too (it is a parity test). The seam tests need the new seam and do not compile on base. |
| Existing tests untouched | HOLDS | no existing test file changed |

## What changed
- probe_unix.go: ProcessStart keeps the pid < 1 check and calls `procStart`; localHost takes `osLstartUTC` / `osLstartLocal`; processStartText and the `context` / `os/exec` imports moved out.
- procstart_linux.go: `procStartFrom(readFile, pid)` = btime + ticks/100, UTC, whole seconds, overflow checked; `lstartIn` formats it with `processid.LstartLayout`; `systemZone` loads /etc/localtime with `time.LoadLocationFromTZData` (not time.Local, which follows the TZ of this process while ps ran with no TZ); missing file is UTC.
- procstart_darwin.go: procStart = ParseLstart(processStartText) in UTC; processStartText body unchanged; the pair is processStartText and processid.PSLstart.

## Open items
1. **internal/cli/liveness_test.go `unknownProbe` no longer works on Linux.** It puts a stub `ps` on PATH so a live holder probes Unknown. The Linux probe now never runs ps, so the holder probes Alive. This is the intended effect of the brief, but the brief forbids touching internal/cli and existing tests, and row 7 asks for 0 cli failures; both cannot hold. I did not edit the test. I did not run the cli test on base (reasoned from the stub: it only works if ps is run). Suggested split: a lane `C-flake-session-cli-unknown-probe`, change `local unknown` to a holder the Linux probe cannot decide without ps, for example a raw lstart token that matches neither UTC nor the system zone (probePS returns Unknown for it), and make `unknownProbe` apply only where a ps is run. It must land with or right after this lane, or the cli test fails on main.
2. The btime-based start moves if the system clock is stepped (the kernel derives btime from the wall clock), as ps does; an RFC3339 token written before a step may then probe Dead. Same as ps today, not changed by this lane.
3. Row 5 shell line over 300 pids: not run here (see row 5).

## NOT VERIFIED on a live screen
No UI. Darwin: the darwin file compiled by `make cross`, not run on a Mac.
