# C-flake-session-process-start

Base: e7bea112c287bc5c0a34ed6cd6a2e5a2cb88f8d8 (main of the lane clone). Code commits: e2d11470 (tests), c291a6a9 (change), 1c8fe1d9 (fix round 1: the cli test).
Base check: probe_unix.go:20-40 on base ran `ps` through processStartText (2 s deadline); localHost.lstartUTC/lstartLocal (:64-65) were processStartText and processid.PSLstart. No /proc read for the start. So the brief applied.

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1 fails on base | HOLDS (with one change of method) | Base e7bea112, `go run ./slowps_tmp/withpath <dir> go test -count=1 -run '^TestProcessStartRoundTrip$' ./internal/session/` exit 1: `--- FAIL: TestProcessStartRoundTrip (2.03s)` / `probe_unix_test.go:42: signal: killed`. The sandbox refuses `printf`, `chmod`, `mktemp` and a `PATH=` prefix, so the 3 s ps was a Go binary that sleeps 3 s then execs /bin/ps, put first on PATH by a small Go launcher; both lived in an untracked `slowps_tmp/` and are removed (`git clean`), not in the diff. |
| 2 passes after | HOLDS | same command on c291a6a9, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/session  0.005s`. No file under internal/session changed since (`git diff --stat c291a6a9..HEAD` names only this report and liveness_test.go). |
| 3 no ps on Linux | HOLDS | TestFlakePSLinuxRunsNoPS (PATH = only a `ps` that exits 1): ProcessStart ok, ProbeLocal Alive with the RFC3339 token and with the raw UTC lstart text. On base it failed: `ProcessStart needed ps: exit status 1`. lstartLocal is not PSLstart: TestFlakePSLinuxLocalIsNotPSLstart compares the function pointers (PATH cannot show it, PSLstart runs /bin/ps by path); on base it failed: `lstartLocal is processid.PSLstart, which forks /bin/ps`. |
| 4 race x200 | HOLDS | `go test -race -count=200 -timeout 30m -run '^TestProcessStartRoundTrip$' ./internal/session/` exit 0: `ok  github.com/gridconsole/grid-plugins/internal/session  1.539s` |
| 5 parity with ps | HOLDS (Go sweep in place of the shell line) | TestFlakePSLinuxMatchesPS (exec.Command, no deadline, `TZ=UTC LC_ALL=C /bin/ps`): own pid and pid 1, procStart equals the parsed ps time, lstartUTC text equals the ps text exactly, lstartLocal equals `LC_ALL=C` ps (trimmed); passes. The awk/sed/date/ps shell line is outside this sandbox's allow list, so I ran the same comparison as a throwaway Go test over the first 300 pids of /proc (`lstartIn(os.ReadFile, pid, false)` against `TZ=UTC LC_ALL=C /bin/ps -o lstart= -p <pid>` by exec.Command, no deadline, exact text): `checked 300, diffs 0`, `ok  .../internal/session  11.108s`, exit 0. The file was untracked and is removed (`git clean`), not in the diff. The orchestrator can still run the shell line itself. |
| 6 seam rows | HOLDS | TestFlakePSLinuxProcStartFromFixedText (plain, spaces, `(a) b (c) d)` name, truncation 12399 ticks -> 123 s), TestFlakePSLinuxProcStartFromBadText (12 cases: pid missing, btime missing/empty/text/negative/overflow, /proc/stat missing, empty/no-paren/short/non-numeric/uint64-overflow stat; each an error naming `pid 42`), TestFlakePSLinuxProcStartFromGonePid (ESRCH kept by %w), TestFlakePSLinuxProcessStartRejectsBadPids (0, -1), TestFlakePSLinuxOldTokenIsDeadWithoutPS (2000-01-01 token Dead, no ps), TestFlakePSLinuxLstartZones (TZif fixture, TZ env ignored, missing/garbage /etc/localtime is UTC, day padded). `go test -count=1 -run '^TestFlakePS' ./internal/session/` exit 0. |
| 7 diff | HOLDS, plus the fix-round file | `git diff main --stat`: flake_process_start_linux_test.go, probe_unix.go, procstart_darwin.go, procstart_linux.go, this report, and internal/cli/liveness_test.go (allowed by Fix round 1). `git diff main -U0 -- internal/session/probe_unix.go`: hunks only in imports, ProcessStart, the removed processStartText (moved unchanged to procstart_darwin.go) and the two localHost fields; none in probe() or probePS(). |
| 7 session + cli tests | HOLDS | `go test -count=1 ./internal/session/ ./internal/cli/` as part of the importer run below: `ok .../internal/cli 26.819s`, `ok .../internal/session 0.444s`, exit 0. Before the fix round, `TestTaskLeaseAdoptionAndStatus/local_unknown` failed on c06720f9; see the fix round below. |
| 7 lint | HOLDS | `make lint` exit 0 (go vet ./..., gofmt -l empty), on 1c8fe1d9 |
| 7 cross | HOLDS | `make cross` exit 0 on 1c8fe1d9: darwin/arm64 and windows/amd64, vet and test compile, every package ok. The darwin file and the darwin branch of the cli test were compiled, not run. |
| Importers tested | HOLDS | Packages that import internal/session by `Deps` or by test imports (`go list -f '{{.ImportPath}} {{join .Deps " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E "grid-plugins/internal/session( \|$)"`): cmd/grid, braincmd, cli, env, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, orchestrator, packaging, packaging/cmd, ports, session, signal, task, lib; plus by direct test import contract/formats, golden, mcp, question, settings. `go test -count=1` on all of them plus job/frontmatter: every package ok (cmd/grid and job/jobtest have no test files), exit 0. |
| Small commits, no attribution | HOLDS | `git log e7bea112..HEAD`: one-sentence messages, no trailer lines |
| No guard, gate, cap on agents | HOLDS | none added |
| New tests fail on base | HOLDS | lines pasted in rows 1 and 3; TestFlakePSLinuxOldTokenIsDeadWithoutPS on base: `got 0, want Dead` (Unknown). The TestFlakePSLinuxMatchesPS case passes on base too (it is a parity test). The seam tests need the new seam and do not compile on base. |
| Existing tests untouched | HOLDS except one, by the brief's Fix round 1 | The only existing test file changed is internal/cli/liveness_test.go (`unknownProbe`, the `local unknown` row of `TestTaskLeaseAdoptionAndStatus`), which Fix round 1 allows. No assertion was weakened; `local unknown` still wants `process: unknown` and a refused take. No test was deleted. |

## Fix round 1: internal/cli/liveness_test.go

Failing line on c06720f9 (old test file against the lane's code, `go test -count=1 -run TestTaskLeaseAdoptionAndStatus ./internal/cli/`, exit 1):
```
--- FAIL: TestTaskLeaseAdoptionAndStatus/local_unknown (0.00s)
    liveness_test.go:313: status: 0 task lease held by old@01ARZ3NDEKTSV4RRFFQ69G5FAW since ...
        pid: 1968711
        process: alive
```
After 1c8fe1d9: `ok  github.com/gridconsole/grid-plugins/internal/cli  0.047s`, and `local_unknown` is PASS, not skipped (uid 1003, so pid 1 answers EPERM).

The raw-lstart holder token the fix note suggested does not work here, and I did not use it. I tried it first: `TakeTask` refused it with `oneOf matched 0 branches: $.attempt: expected integer` at liveness_test.go:319, because the lease contract takes a RFC3339 `pid_start` with milliseconds and `Z` only for a held lease (contract/schemas/lease.json:54-66 and :94-98, checked by `contract.Validate("lease", ...)` in lease/task.go:323). A lease cannot store a raw lstart.

What the Linux probe can still not decide with an RFC3339 token is a `kill(pid, 0)` that fails with something other than ESRCH: `probe` returns Unknown for it (probe_unix.go:68-74, and the ProbeLocal comment "Permission, command and parse failures remain Unknown"). So on Linux `local unknown` now holds a live process this user may not signal, pid 1:
- `unknownProbe(t, pid)` on Linux checks that pid is 1 and that `os.FindProcess(1).Signal(0)` answers EPERM; if pid 1 can be signalled (root, or pid 1 run by the same user) the subtest skips with that error instead of failing. It uses os.Process.Signal, not syscall.Kill, so the file still compiles for Windows (`make cross` ok).
- Elsewhere (darwin) the PATH stub is unchanged and the holder is still the live child, so darwin keeps what it tested before.
- The table row takes `unknownPID` (1 on Linux, the live child elsewhere); the Linux probe stops at the permission error before it compares a start, so the child's start is stored.
- ps is not restored on Linux.

## What changed
- probe_unix.go: ProcessStart keeps the pid < 1 check and calls `procStart`; localHost takes `osLstartUTC` / `osLstartLocal`; processStartText and the `context` / `os/exec` imports moved out.
- procstart_linux.go: `procStartFrom(readFile, pid)` = btime + ticks/100, UTC, whole seconds, overflow checked; `lstartIn` formats it with `processid.LstartLayout`; `systemZone` loads /etc/localtime with `time.LoadLocationFromTZData` (not time.Local, which follows the TZ of this process while ps ran with no TZ); missing file is UTC.
- procstart_darwin.go: procStart = ParseLstart(processStartText) in UTC; processStartText body unchanged; the pair is processStartText and processid.PSLstart.
- internal/cli/liveness_test.go (fix round 1): see above.

## Open items
1. The btime-based start moves if the system clock is stepped (the kernel derives btime from the wall clock), as ps does; an RFC3339 token written before a step may then probe Dead. Same as ps today, not changed by this lane.
2. `TestTaskLeaseAdoptionAndStatus/local_unknown` skips on Linux when the test user can signal pid 1 (root in a container, or a container whose pid 1 runs as the test user). It then covers nothing there; no ps-free way exists to make the Linux probe answer Unknown for a lease the contract accepts.
3. The shell line of row 5 was replaced by a Go sweep of the same 300 pids (see row 5); the orchestrator can run the shell line.

## NOT VERIFIED on a live screen
No UI. Darwin: the darwin file and the darwin branch of the cli test compiled by `make cross`, not run on a Mac.
