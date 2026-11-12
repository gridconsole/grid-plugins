# C-yc-worker-stopped-requeue: a worker-exit cancelled with code worker_stopped puts the Job back to Ready, with no re-plan Job

Repo: plugin. Branch lane-C-yc-worker-stopped-requeue. Base: main 2b47f62181522cb3036a3e38e186ec6d9e779eb8.

Base check: HOLDS. On the base, `case "cancelled":` (internal/orchestrator/tick.go:288) sets `contract.Cancelled` ("worker cancelled") and never reads `j.ExitCode`. Nothing had changed it, so the lane went ahead.

## Proof table

| # | Row / rule | Result | Command, exit code or file:line |
|---|---|---|---|
| 1 | cancelled + code "worker_stopped", J-2 waits: J-1 back to Ready, last control for J-1 has reason "worker stopped", no job record with role orchestrator before or after | HOLDS (see the note on "is Ready") | `go test -count=1 -v -run 'TestYcWs' ./internal/orchestrator/` exit 0, `--- PASS: TestYcWsStoppedIsReady`; internal/orchestrator/yc_worker_stopped_test.go:84 |
| 1b | the same case is red on base | HOLDS | Base tick.go restored with `git stash push internal/orchestrator/tick.go`, same command, exit 1: `yc_worker_stopped_test.go:91: last control for J-1 = map[close_reason:worker cancelled reason:worker cancelled status:cancelled target:J-1], want status ready, reason worker stopped` then `--- FAIL: TestYcWsStoppedIsReady`. The same run: `--- FAIL: TestYcWsStoppedOfferedAgainNeverParked` ("round 1: assigned 0 ... Status:cancelled ... ExitCode:"worker_stopped" ... (stopped fence 2)") and `--- FAIL: TestYcWsStoppedClearsCrashCount` (Status:cancelled). Stash popped; the lane passes. |
| 2 | the Job is offered again with a higher fence; worker_stopped exits in a row never park it | HOLDS | `--- PASS: TestYcWsStoppedOfferedAgainNeverParked` (4 rounds: `Output.Assigned` 1, `Assign.Fence` above the stopped one, not Parked, Reason not needs_person); `--- PASS: TestYcWsStoppedClearsCrashCount` (crash, crash, stop, crash does not park). The `return` at tick.go:291 comes before the park block at tick.go:339-346 (tick.go:336-343 on main). |
| 3 | no code + J-2 waiting: Cancelled and one re-plan Job; code "prepare_failed": Cancelled; code 0: Cancelled | HOLDS | `--- PASS: TestYcWsOtherCancelledStaysFinal` with subtests no_code, prepare_failed, record_failed, number_0, another_number. Each asserts Cancelled, last control reason "worker cancelled" and one re-plan Job J-3 from J-1. These subtests also pass on base, since they guard the unchanged path. `--- PASS: TestYcWsHelper` reads only the JSON string worker_stopped. |
| 4a | `go test -count=1 -run 'TestGk\|TestGq\|TestUsageLimitReoffer' ./internal/orchestrator/` | HOLDS | Run with `TestYcWs` added: `ok ... internal/orchestrator 11.092s`, exit 0. TestGkReplanNotForControlCancel is in that set and passes. |
| 4b | `go test -count=1 ./internal/orchestrator/ ./internal/contract/` | HOLDS | exit 0: `ok internal/orchestrator 165.812s`, `ok internal/contract 1.422s` |
| 4c | `git grep -n worker_stopped -- contract/docs/feed-posts.md` hits in the worker-exit section | HOLDS | contract/docs/feed-posts.md:735 (the paragraph after `## worker-exit` at :732) |
| 5a | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0: `ok internal/cli 0.048s` |
| 5b | `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0: `ok internal/job/lint 0.468s` |
| 5c | `make lint` | HOLDS | exit 0 (`go vet ./...`, no output). `gofmt -l internal/orchestrator` prints nothing. |
| 5d | `make cross` (darwin/arm64 and windows/amd64 vet and test compile) | HOLDS | exit 0, every package `ok` on both targets |
| 5e | `git diff main --stat` names only the Touches | HOLDS | contract/docs/feed-posts.md, internal/orchestrator/tick.go, internal/orchestrator/yc_worker_stopped.go, internal/orchestrator/yc_worker_stopped_test.go. This report (docs/lanes/C-yc-worker-stopped-requeue.md) is the runbook's required file and is the one addition. |
| R1 | packages that import what I changed, tested | HOLDS | `go list` shows internal/mcp, internal/migrate and internal/cli import internal/orchestrator (cmd/grid and lib only through cli). `go test -count=1 ./internal/orchestrator/ ./internal/contract/ ./contract/... ./internal/mcp/ ./internal/migrate/` exit 0. The whole cli package ran as `go test -count=1 ./internal/cli ./internal/job/lint/ -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList|.' -skip 'XXXNONE' 2>&1 | tail -5`: `ok internal/cli 55.773s`, `ok internal/job/lint 0.468s` (the exit code seen there was tail's, the `ok` lines are the evidence). cmd/grid and lib have no code of mine and compile in `make cross`. |
| R2 | repo-wide walk tests, plugin set | HOLDS | 5a and 5b above. The app-only walks (`make webui-test`, TestFvArgvCollects, TestEveryWiredMount, slices Imports) do not exist in this repo. |
| R3 | new tests fail on base | HOLDS | row 1b |
| R4 | no existing test edited or deleted | HOLDS | `git diff main --stat` shows no existing test file |
| R5 | no guard, gate, cap, hold or blocking hook added | HOLDS | The change is one branch in the existing cancelled case. It adds no limit: a Job whose worker keeps stopping is offered again every time, as row 2 says. |
| R6 | commit rules | HOLDS | one commit, one-sentence message, no attribution lines, author is the lane identity (Grid lane). No personal path, host or email in the diff. |
| R7 | RealPlugin tests | NOT APPLICABLE | The brief names none. |

## Where the answer differs from the brief's wording

Row 1 says "tick: J-1 is Ready". After one tick J-1 is not left Ready, because the same tick offers every Ready Job (tick.go:148-166, `s.assign`). The crash retry behaves the same: TestMoves "crash" reads Assigned after one tick. What the Feed shows is a `control` with status ready and reason "worker stopped" for J-1, `re` the worker-exit, then an assign. So `TestYcWsStoppedIsReady` asserts that control and that J-1 is Assigned (not Cancelled). Row 2 says "the next tick assigns J-1 again". It is the first tick after the exit that assigns (`Output.Assigned` 1), and the test reads that tick. Nothing else differs.

## What changed

- internal/orchestrator/tick.go:288-293: in `case "cancelled":`, when `ycWorkerStopped(j.ExitCode)` it posts `s.control(j, contract.Ready, "worker stopped", j.Exited)`; `m.Failures = nil` runs before it as on main. Any other code takes the Cancelled line as on main.
- internal/orchestrator/yc_worker_stopped.go (new): `ycWorkerStopped(raw json.RawMessage) bool`, true only for the JSON string `"worker_stopped"` (decodes as gqCauseText does). A number, no code, another string or a different case is false.
- replan.go is untouched. A Job that went Ready is not Cancelled, so `gkReplan` skips it (replan.go:30).
- contract/docs/feed-posts.md:735: one sentence on the worker-exit page that cancelled with code `worker_stopped` means the worker stopped or restarted and the Job is offered again, with no re-plan Job.
- internal/orchestrator/yc_worker_stopped_test.go (new): `ycWsExit`, `ycWsRig` and the tests above.
- No schema change was needed: contract/schemas/worker-exit.json:42-52 already allows a non-empty string code, and fold.go:378, :392-399 keeps it.

## Open items

- The app worker sends `worker_stopped` only after C-yc2-worker-posts-stopped lands. The string must match exactly. Until then no worker sends the code and nothing changes at run time.
- The code is read only under reason `cancelled`. A `crash` exit with the same code takes the crash retry path.
- The cross-repo set and the whole-module test are the orchestrator's at landing. The app rigs named in the brief were not run here.

## NOT VERIFIED on a live screen

Nothing here is a UI change. Not run: a real `grid-worker stop` against a running Job, which needs the app worker from C-yc2-worker-posts-stopped.

Verdict: DONE
