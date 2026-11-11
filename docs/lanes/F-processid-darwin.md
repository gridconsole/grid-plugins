# F-processid-darwin: process identity and liveness on macOS

Repo plugin, branch lane-F-processid-darwin. Packages touched: internal/processid, internal/session (probe_unix.go, probe_unix_test.go). Nothing in fslock or internal/env.

## Proof table

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | Inventory of every /proc read and ps call in internal/processid and internal/session, with the OS | HOLDS | See "Inventory" below (base and lane lines). |
| 2a | On darwin a `proc:` PIDStart returns Unknown, never Dead or Alive | HOLDS | `probe_unix.go:79-82` returns Unknown before kill or any read when goos is not linux. Test `TestProbeLocalProcTokenOnDarwinIsUnknown` (`probe_unix_test.go:114`): 4 tokens x kill nil/ESRCH/EPERM, zero /proc reads and zero ps runs. `go test ./internal/session/...` exit 0. |
| 2b | A darwin-written PIDStart is compared through the ps path only | HOLDS | `TestProbeLocalDarwinTokensUsePSOnly` (`probe_unix_test.go:130`) makes `procStat` and `bootID` call `t.Fatal`; raw lstart in the writer's zone and in UTC read Alive, a raw line matching neither zone and garbage read Unknown. `TestStartDarwinUsesPSOnly` (`start_host_test.go:57`): `processid.Start` on goos darwin reads no file. |
| 2c | ps output parse is a pure function with table tests: C locale lines, padded day, pid that has gone | HOLDS | `processid.ParseLstart` (`parse.go:21`), `TestParseLstart` (`parse_test.go:9`): C locale line, padded day, ps newline, BSD column padding, named zone, pid gone (`""` and `"\n"` give `ErrNoProcess`), non C locale line, two lines, truncated line. Also `TestProcStartTicks` (`:45`) and `TestProcTokenRoundTrip` (`:66`) for the /proc side. Exit 0. |
| 3 | Pid reuse: same pid, different start reads Dead on both paths, fake reader, no real process | HOLDS | `TestProbeLocalPidReuseIsDeadOnBothPaths` (`probe_unix_test.go:87`): /proc path (ticks differ, boot and ticks differ, reboot) and ps path (start differs by one second, by a day) all Dead; matching starts Alive. Uses `probeHost` fakes only. Exit 0. |
| 4 | `GOOS=darwin go vet` and `go test -c` for both packages | HOLDS | `GOOS=darwin go vet ./internal/processid/... ./internal/session/...` exit 0. `GOOS=darwin go test -c -o /dev/null ./internal/processid/` exit 0. `GOOS=darwin go test -c -o /dev/null ./internal/session/` exit 0. |
| 5 | `go test ./internal/processid/... ./internal/session/...` and `make lint` | HOLDS | `go test ./internal/processid/... ./internal/session/... -count=1`: both `ok`, exit 0. `make lint` (go vet ./...) exit 0. |
| R1 | New tests fail on base, pass on lane | HOLDS | Run against the base probe logic (same `probeHost` seam, old decision code): `FAIL TestProbeLocalPidReuseIsDeadOnBothPaths/proc_boot_and_ticks_match: got 2, want 1`; `FAIL TestProbeLocalProcTokenOnDarwinIsUnknown`: `kill <nil>, token "proc:222": got 1, want Unknown` and `darwin read /proc or ran ps 1 times` (and the same for `proc:boot-a:222`, `proc:111`, `proc:boot-b:222`; with ESRCH `got 2, want Unknown`); `FAIL TestProbeLocalDarwinTokensUsePSOnly/raw_lstart_in_the_writer's_zone: got 0, want 1`. States are Unknown=0, Alive=1, Dead=2. All three pass on the lane. The processid tests are new files (new functions), so on plain base they fail to compile. |
| R2 | No existing test edited or deleted | HOLDS | `probe_unix_test.go` only gained `errors` and `syscall` imports and tests appended after `TestProcessStartRoundTrip`; `TestProbeLocal` and `TestProcessStartRoundTrip` unchanged and pass. `start_unix_test.go` untouched. |
| R3 | No guard, gate, cap, hold or blocking hook on agents added | HOLDS | Diff touches only start-token parsing and the liveness read. No agent path touched. |
| R4 | Tests of packages changed and of every importer, plus lint, Windows and macOS builds | HOLDS | Importers from `go list -f '{{.ImportPath}}: {{join .Deps " "}}' ./...` filtered for processid or session: cmd/grid, braincmd, cli, env, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, migrate, orchestrator, packaging, packaging/cmd, ports, signal, task. `go test -count=1` over all of them: every package `ok` or `[no test files]`, exit 0 (cli 176s, orchestrator 110s). `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| R5 | Small commits, one-sentence messages, no attribution, no personal paths, hosts or emails | HOLDS | `git log --oneline origin/main..HEAD` shows the lane commits; messages carry no trailer lines; no paths, hosts or addresses in code or tests. |

## Inventory (row 1)

Base lines, then where they are on the lane.

| What | Base | Lane | Runs on |
|------|------|------|---------|
| read `/proc/<pid>/stat` for the token | `processid/start_unix.go:24` | `processid/start_unix.go:46`, inside the `goos == "linux"` branch | Linux only (runtime.GOOS check; file compiles on every unix) |
| read `/proc/sys/kernel/random/boot_id` | `processid/start_unix.go:28` | `processid/start_unix.go:50` (constant `BootIDPath`, `parse.go:34`) | Linux only |
| `/bin/ps -o lstart= -p`, LC_ALL=C, system zone | `processid/start_unix.go:42` | `processid/start_unix.go:31` (`PSLstart`), called for every unix that is not Linux | darwin and other non-Linux unix |
| `ps -o lstart= -p` via PATH, LC_ALL=C TZ=UTC | `session/probe_unix.go:34` | `session/probe_unix.go:36` (`processStartText`) | Linux and darwin: `ProcessStart` and the ps branch of `ProbeLocal` |
| read `/proc/<pid>/stat` when PIDStart starts with `proc:` | `session/probe_unix.go:55` (any OS) | `session/probe_unix.go:57` (`localHost.procStat`), reached only through `probeProc` after the goos check at `:79-:82` | Linux only now; on darwin the token returns Unknown before any read |
| (new) read boot_id to check a `proc:<boot>:<ticks>` token | none | `session/probe_unix.go:61` | Linux only |
| (new) ps in the system zone, to match a raw lstart token written by `processid.Start` | none | `session/probe_unix.go:65` (`processid.PSLstart`) | Linux and darwin, only for a raw lstart token |
| ps in tests | `probe_unix_test.go:22` real `ps` for the current pid | unchanged | Linux and darwin (existing test) |

Outside the two packages, for the record: `internal/migrate/process_unix.go` calls `processid.Start` and strips the boot ID on Linux, so Linux attach tokens stay in the older `proc:<ticks>` form. `ProbeLocal` accepts both forms. `internal/cli/orchestrator_test.go:272` and `liveness_test.go:389` run or fake `ps`; they pass.

## What changed

- `internal/processid/parse.go` (new, no build tag): pure `ParseLstart`, `ProcStartTicks`, `ProcToken`, `ParseProcToken`, `BootIDPath`, `ErrNoProcess`.
- `internal/processid/start_unix.go`: `Start` goes through a `host` value (goos, file reader, ps reader) so tests need no real process. Its output is unchanged on both OSes. `PSLstart` is exported so `session` runs ps the way `Start` does.
- `internal/session/probe_unix.go`: `ProbeLocal` goes through a `probeHost` value. A `proc:` token on a non-Linux host returns Unknown before kill and before any read. On Linux a `proc:<boot>:<ticks>` token, which `processid.Start` writes and the old code always read as Dead, now compares boot ID and ticks. A raw lstart token (what `processid.Start` writes on darwin, in the writer's zone) reads Alive when it equals ps's text in UTC or in the system zone; before, any non-UTC zone gave Unknown, so `task take` refused a live session on a Mac. `ProcessStart` parses through `processid.ParseLstart`.
- Tests: `parse_test.go`, `start_host_test.go` (new), and four new tests in `probe_unix_test.go`.

Decisions to check: a raw lstart token that matches neither zone returns Unknown, not Dead, because its zone cannot be known and a wrong Dead would free a live lease. RFC3339 tokens (`session.ProcessStart`) still read Dead on a changed start.

## NOT VERIFIED on a live screen or machine

- Real macOS `ps` output and a real macOS process: the darwin code is vetted and the tests compile for darwin (rows 4, R4) but ran only on Linux, through fakes. A run of `go test ./internal/processid/... ./internal/session/...` on the Mac would close this.
- A non-UTC system zone on a real Mac for the raw lstart match (tested with fakes only).
