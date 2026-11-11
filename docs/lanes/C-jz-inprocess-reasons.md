# C-jz-inprocess-reasons: proof table

Base: cf5b259f (lane branch was at main; `git merge main` answered "Already up to date.").
Ancestor checks on HEAD, each `git merge-base --is-ancestor <branch> HEAD` exit 0:
origin/lane-C-jz-land-env-from-call, origin/lane-C-jz-task-owner-from-call,
origin/lane-C-jz-agent-probe-env-from-call, origin/lane-C-jw-feed-lock-context-tests.
So all four verbs flip: `routine run`, `land`, `task new`, `agent`.

| Row | Verdict | Proof |
|---|---|---|
| 1 four tests PASS; TestHsIpSafe holds routine run, land, task new, agent sync | HOLDS | `go test -count=1 -v -run 'TestHsIpWalksEveryCommand\|TestHsIpSafe\|TestHsIpBinaryOnly\|TestHsIpFailClosed' ./internal/cli/` exit 0: `--- PASS` for all four. Rows at internal/cli/hs_inprocess_test.go:109-112. Fails on base (test edited first, table unchanged): see below |
| 2 lib tests | HOLDS | `go test -count=1 ./lib/` exit 0 (`ok .../lib 20.177s`) |
| 3 no flipped verb has host-environment; every why cites a live path:line | HOLDS | `git grep -n 'host-environment' HEAD -- internal/cli/inprocess.go` names only `account sign-in` (:146), which stays. Citations listed below |
| 4 lib.go changes only the list; Context comment kept; items cite path:line | HOLDS | `git diff main -- lib/lib.go` touches lines 27-41 only (the list); the Context comment and Exit codes text are untouched. lib/inprocess.go changes only its doc. Items listed below |
| 5 golden, lint, cross | HOLDS | `make golden` exit 0, then `git status --short golden/` empty; `make lint` exit 0; `make cross` exit 0 |
| Rule: test fails on base | HOLDS | output below |
| Rule: tests of changed packages and importers | HOLDS | importers of internal/cli and lib (`go list` over the module): `cmd/grid`, `lib`. `go test -count=1 ./internal/cli/ ./lib/ ./cmd/grid/` exit 0 (cli ok 64.979s, lib ok 24.631s, cmd/grid no test files) |
| Rule: plugin walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| Rule: edits only in Touches | HOLDS | `git diff --stat main`: internal/cli/hs_inprocess_test.go, internal/cli/inprocess.go, lib/inprocess.go, lib/lib.go (plus this report) |
| Rule: no guard, gate, cap or hold added | HOLDS | the diff only turns four binary-only answers into yes and rewrites comments |
| Rule: no attribution, no personal paths | HOLDS | commit messages are one sentence each |
| Rule: RealPlugin tests | n/a | none named by the brief |

Base failure (TestHsIpSafe with the moved rows, inprocess.go still as on main):

```
hs_inprocess_test.go:145: InProcess(["routine" "run" "x"]) = false "routine run: runs the routine as a host-environment child (routine.go:43, routine/execute.go)", want true
hs_inprocess_test.go:145: InProcess(["land"]) = false "land: waits on the process's interrupt signal and starts host-environment children (land.go:34)", want true
hs_inprocess_test.go:145: InProcess(["task" "new" "x"]) = false "task new: records the process id as the first Job's owner (task_new_first_job.go:84)", want true
hs_inprocess_test.go:145: InProcess(["agent" "sync"]) = false "agent: waits on process signals, probes host children and reads the host's executable (agent.go:36, agent_sync.go:17)", want true
--- FAIL: TestHsIpSafe (0.00s)
```

## Why each verb flips (code on HEAD)

- `routine run`: internal/cli/routine.go:145 passes `Getenv: env.getenv`; routine.go:146 runs on context.Background with no signal handler; internal/routine/execute.go:147-150 and :169-174 build the child's environment from that getenv.
- `land`: internal/cli/land.go:32 `env.signalContext` (the Call's Context, cli.go:71-75); land.go:34 passes `Getenv: env.Getenv`; internal/land/land.go:245 and internal/land/environment.go:82-83 build the deploy child's environment from it; internal/land/process.go:134 always sets cmd.Env.
- `task new`: internal/cli/task_new_first_job.go:84-86 takes OwnerPID; internal/cli/task.go:125 passes `Getenv: env.Getenv`, which internal/task/create.go:188-197 uses for the owner name.
- `agent` (whole verb): agent.go:14 probe with the Call's PATH and Getenv (internal/agent/runner.go:57-58); agent.go:34 signalContext; agent_sync.go:17 env.executable (Call.Executable); budget.go:19 Getenv; settings.go:194-201 Call's getenv and Stdin.

## Row 3: citations of every remaining why (all checked on HEAD)

open.go:67, open_start.go:216 (cli.go:132); hook cli.go:134, hook.go:19; task attach task.go:241; job next and feed tick orchestrator.go:303; job verify job/jobcmd/verify.go:38, :41; lease take --hold task_lease.go:85; mcp cli.go:140, mcp.go:35; env up environment.go:126, env/env.go:117 (child env.go:127); routine schedule routine.go:197, routine/schedule.go:37; worker next worker.go:188; account list/status/scan account/account.go:216, :68 (cli account.go:30 gives no Environ); account sign-in account.go:54, account/account.go:388; account note --then account.go:200, account/statusline.go:125; eval run evalcmd/command.go:169, :175. Ten of these were stale on main (cli.go:105/107/113, task.go:235, orchestrator.go:300, verify.go:31, environment.go:128, env.go:113/121, worker.go:189, account.go:183/385, command.go:155) and now point at the current line.

## Row 4: items left in lib.go "What a call reads"

Dropped: the task owner's user name (fixed by C-jz-task-owner-from-call), and the children of land, agent and routine runs (fixed by the three T2 lanes).
Kept, with paths:
- account provider child takes os.Environ: internal/cli/account.go:30, internal/account/account.go:68, :216 (the item said "when it is given no getenv"; the cli does pass Getenv but no Environ, so the item now says that).
- migrate reads CLAUDE_PLUGIN_DATA from the host without --plugin-data: internal/migrate/plan.go:86, internal/cli/migrate.go:65.
- owner process id: internal/cli/task_new_first_job.go:84, internal/env/env.go:117, internal/cli/task.go:241, internal/cli/task_lease.go:85.

## What changed

- internal/cli/inprocess.go: `routine run`, `land`, `task new`, `agent` answer yes; every kept why cites its current line; the header list adds "writes a system schedule for the host's executable" (routine schedule's reason, which the list did not cover).
- internal/cli/hs_inprocess_test.go: the four argv move from hsIpBinaryOnly to hsIpSafe; `{"routine", "schedule"}` joins hsIpBinaryOnly; the global-flag example ends in `mcp` instead of `land`.
- lib/lib.go: the "What a call reads" list as above.
- lib/inprocess.go: doc list mirrors the header list.

## Open items (notes, not acted on)

- `job next`, `feed tick`, `worker next` and `env up` wait on signals through env.signalContext, which uses the Call's Context when set (cli.go:71-75). The brief keeps them binary-only; a later brief may judge them.
- Residual host reads in flipped verbs: a bare program name (land's deploy argv[0], routine's `claude`) is looked up by exec.Command on the host's PATH; land's Windows killer reads the host's SystemRoot for taskkill (internal/land/process_windows.go:50).
- `hook` and `mcp` read env.Stdin and env.Stdout (hook.go:19, mcp.go:35), not the host's own streams; kept binary-only as the brief says.

NOT VERIFIED on a live screen: nothing; no UI in this lane.

DONE
