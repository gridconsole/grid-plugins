# C-ih-plib-call: proof table

Base: ee8a96c1 (origin/main at clone time; the brief cites 91fadeff, and every line it names matched on this base). On the base, lib.Call had neither a Context nor an owner (lib/lib.go:63-93).

| Row | Status | Proof |
|---|---|---|
| 1 TestIhClStreamStopsOnContext | HOLDS | `go test -race -count=1 -v -run 'TestIhCl\|TestJobNextInterruptExits130\|TestWorkerNextStreamsSpawnLinesUntilSIGINT' ./lib/ ./internal/cli/` exit 0: `--- PASS: TestIhClStreamStopsOnContext`, `--- PASS: TestJobNextInterruptExits130`, `--- PASS: TestWorkerNextStreamsSpawnLinesUntilSIGINT` (the process tests are unchanged). worker next returns 130 (worker.go:189-192) and job next --wait returns 130 (orchestrator.go:307-310). "No handler": the helper at internal/cli/cli.go:66-72 returns `context.WithCancel(e.Context)` when Context is set. The test checks that the returned context's String has no `signal.NotifyContext` when Context is set, and has it when Context is nil. Red on base: `internal/cli/ih_call_test.go:42:8: h.env.Context undefined (type Env has no field or method Context)` |
| 2 grep and TestIhClSubEnvContext | HOLDS | `git grep -n 'signal.NotifyContext' -- internal/ lib/ ':!*_test.go'` gives `internal/cli/cli.go:71`, `internal/evalcmd/command.go:162`, `internal/job/jobcmd/verify.go:36`. On base it listed 7 files: agent, environment, land, orchestrator, worker, evalcmd/command, jobcmd/verify. `eval run` through cli.Run with a live Context gives 0; with a cancelled one it gives 1 and no passed record (evalcmd/command.go:171-173, status not passed; eval/process.go:54, eval/check.go:47). `job verify` with a cancelled Context gives 1 and "context canceled" (job/verify.go:49, jobcmd/verify.go:40-42). With Context nil, the existing evalcmd and jobcmd tests pass unchanged (row 6 run). Red on base: the same compile failure as row 1 |
| 3 TestIhClNoLockLeftAfterCancel | HOLDS | `--- PASS: TestIhClNoLockLeftAfterCancel` (race run above). A cancelled worker next gives 130. Then lease take --hold, job next, a cancelled job next --wait (130) and lease release. After that, feed post exits 0, feed state --json exits 0 and changed, and feed read shows the post. Red on base: `lib/ih_call_test.go:78:17: unknown field Context in struct literal of type Call` |
| 4a TestIhClOwnerOnHold | HOLDS | `--- PASS: TestIhClOwnerOnHold`. OwnerPID = the test's own pid (not os.Getppid). `--hold --pid` is still refused with exit 2 and the text of task_lease.go:82-83. The held lease shows pid = owner and pid_start = the looked-up start (OwnerStart zero, never empty). After release and a take with a given OwnerStart, pid_start = contract.FormatTime(given). The test skips off linux/darwin, citing task_lease.go:75-77. Red on base: `lib/ih_call_test.go:160:18: unknown field OwnerPID in struct literal of type Call` |
| 4b TestIhClOwnerOnTaskNew: lease status shows the owner's pid and pid_start after task new | NOT DONE | Cannot be shown with the read the brief names. task new holds the task lease only while it posts the first Job and releases it on return (internal/cli/task_new_first_job.go:103 `defer ts.ReleaseTask(w)`). After the call, `lease status task --json` gives `{"lease":"task","holder":null,"fence":1,"gen":0,"attempt":null,"since":"…","pid":null,"pid_start":null,"process":"free"}` whatever the owner. No Getenv, Stdout or other callback runs inside that window either (task_new_first_job.go:92-108), so no test can catch it there. The code is at task_new_first_job.go:84-95: OwnerPID and OwnerStart are used when set, else the start is looked up as today. The test that ships only shows that task new with both owner forms exits 0 and leaves the lease free, as the binary does. Split: a later brief has to choose a read for task new's owner, for example a recorded holder in the handover record or a seam on cli.Env. |
| 5 TestIhClDocNamesFive | HOLDS | `--- PASS: TestIhClDocNamesFive`. lib/lib.go:52-53 (package doc) and :119-120 (Run doc) contain "5 taken (intent already claimed or done; skip)", the words of exitcode.go:10. Red on base: `grep -c '5 taken' lib/lib.go` gave 0 |
| 6a diff names only Touches | HOLDS | `git diff origin/main --stat`: the 18 files are all in Touches, plus this report (docs/lanes/C-ih-plib-call.md, which the runbook requires) |
| 6b package tests | HOLDS | `go test -count=1 ./lib/ ./internal/cli/ ./internal/env/ ./internal/evalcmd/ ./internal/job/... ./cmd/grid/` exit 0 (all ok). Importers from `go list -f '{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}}' ./... \| grep …`: cmd/grid, internal/cli, lib |
| 6c race | HOLDS | `go test -race -count=1 -v -run 'TestIhCl…' ./lib/ ./internal/cli/` exit 0, all PASS |
| 6d lint, cross | HOLDS | `make lint && make cross && echo LINT_CROSS_OK` printed LINT_CROSS_OK (exit 0; darwin/arm64 and windows/amd64 vet and test compile) |

## Rules

| Rule | Status | Proof |
|---|---|---|
| Small commits, one sentence, no attribution | HOLDS | `git log origin/main..lane-C-ih-plib-call` |
| New tests fail on base and pass on lane | HOLDS | base production files checked out over the lane tests; the compile failures are pasted above |
| No existing test edited or deleted | HOLDS | diff stat: only the new files lib/ih_call_test.go and internal/cli/ih_call_test.go |
| No guard, gate, cap or hold on agents | HOLDS | Only a context and owner plumbing, nothing that refuses |
| Binary path unchanged: OSEnv sets no Context and no owner | HOLDS | cli.go OSEnv untouched. The SIGINT process tests pass, and commands.json and the help goldens are unchanged (cli tests green) |
| No command started with VAR= | HOLDS | none was run |
| Tests: t.TempDir homes with HOME, GRID_HOME, CLAUDE_CONFIG_DIR, CODEX_HOME | HOLDS | ihClHome and ihClEnviron (lib), ihClIsolate (cli). No sleeps or clock reads for results |
| Plugin never imports the app | HOLDS | no new imports outside the module |

## What changed

- `lib.Call` gets `Context context.Context`, `OwnerPID int` and `OwnerStart time.Time`. env() copies them to cli.Env, and OwnerStart only when OwnerPID is set.
- `cli.Env` gets the same three fields and `signalContext`. The five cli sites (agent, env, land, job next/feed tick, worker next) call it.
- `evalcmd.Env.Context` and `jobcmd.Env.Context` are passed on from cli (eval.go, jobcmdEnv). Their own SIGINT handler is used only when Context is nil.
- Owner: task new, task attach and lease take --hold use OwnerPID/OwnerStart when set. With OwnerStart zero they look up the start as today. env up uses `Manager.OwnerPID` (zero is os.Getpid), and ports.Claim reads the start itself.
- lib doc: exit 5 is named in the package and Run docs, the 130 of the streams is described, the Context scope (the seven sites; other verbs and Feed-lock waits are not stopped) is written down, and the owner host-read line now covers the Windows limit.

## NOT VERIFIED on a live screen

Nothing here needs a screen. task attach and env up with an owner are covered only by code reading (task.go:238-247, environment.go:105, env.go:113-117), not by a test; no acceptance row names them.
