# C-e1012-reopen-shoulds: end review 1012Z should-fix S1, S2 (plugin part), S4, S5

Repo: plugin. Branch lane-C-e1012-reopen-shoulds, from main faf43707. Tests commit a881ed7e, code commit e82850cc, the doc commits after it change only docs/lanes.

## What changed

- **S1** `internal/cli/reopen_continue.go`: for a closed task the Continue Job is posted only when the last receipt in history.md that moved the task to closed is a `stop` (`reopenClosedByStop`, read as `updates.closedAt` reads receipts: the `- {json}` lines of `task.Event`, kept inside internal/cli). A delivered task that the land approval closed (`close`) and once had a cancelled Job is reopened to queued as before, with nothing posted. A history with no receipt, or none readable, is not a stop. The running-task branch is unchanged.
- **S2** the header, the `reopenStopCode` and `reopenContinueJob` comments of reopen_continue.go, and the Cause and What-changed text of docs/lanes/C-r13-stopped-task-continues.md now say what the worker does since app c8bec32d (end review 1012Z ruling 1): a worker stop leaves the Jobs Ready (`worker_stopped`), only a task stop cancels them (`stop_requested`, "task stopped"), and the running + `stop_requested` branch is reached only on a Feed written before c8bec32d. The app's docs/lanes/C-r12-stopped-job-not-rerun.md line ("superseded by C-e0812-m1") is in the app repo: see the table.
- **S4** `internal/orchestrator/tick.go` `advance`: a Job that is Ready with a folded exit `cancelled` and code `worker_stopped` (the Ready the worker posts itself) clears `m.Failures`, as the tick's own `cancelled` arm does. The pass saves the memory as before.
- **S5** `reopenTaskWriter` returns a `begin` func in place of writing the Handover record itself; the continuation's `post` calls it after the state moves, before the Job is posted. A move the task write refuses leaves no Handover record. Exposure that moves with it: a Handover that fails now fails after the state moved, the same exposure as a failed Job post (the task is queued and the lease is given back; a second reopen is refused, as the task is queued, and the Job can be posted with `grid job post`).

## Proof table

Head the checks ran on: e82850cc (the lane doc and the r13 doc change only docs/lanes). `git fetch origin && git merge origin/main` before the run: `Already up to date.`

| Row | Result |
|---|---|
| S1 test: a delivered task with one cancelled Job reopens without that Job | HOLDS. `go test -count=1 ./internal/cli -run TestE1012` exit 0 on the lane (`ok .../internal/cli`). Fails on base (tests committed first, code of faf43707): `--- FAIL: TestE1012ReopenOfADeliveredTaskPostsNoContinueJob` / `e1012_reopen_shoulds_test.go:40: the reopen of a delivered task posted a Job: before map[J-1:1] after map[J-1:1 J-2:1]`. The test builds T-07's Feed, runs `grid task close`, then `grid task reopen`, and checks queued and no J-2. |
| S1 test: a stopped task still gets the Job | HOLDS, already covered: `TestR13ReopenAfterTaskStopPostsTheOrchestratorJob` (`grid task stop`, then `grid task reopen`, J-2 open and spawned) passes on base and on the lane (`go test -count=1 ./internal/cli -run 'TestE1012\|TestR13\|TestYcWs'` exit 0). It is a keep-working test, not a new one; no copy added. |
| S2 comments and the r13 Cause | HOLDS: reopen_continue.go:23-48 header, :68-70 `reopenStopCode`, :72-92 `reopenClosedByStop`, :103-108 the `reopenContinueJob` comment; docs/lanes/C-r13-stopped-task-continues.md Cause (two bullets: the worker then, the worker since c8bec32d) and the What-changed bullet. |
| S2 "superseded by C-e0812-m1" line in the app's C-r12-stopped-job-not-rerun.md | not run, not asked of this repo: the file is in the app repo (the brief: "S2 (plugin part)"); the orchestrator adds the line there. |
| S4 test: two earlier failures, a worker stop, one failure: not parked | HOLDS. `go test -count=1 ./internal/orchestrator -run TestE1012` exit 0 on the lane. Fails on base: `--- FAIL: TestE1012WorkerPostedReadyClearsTheFailureCount` / `e1012_worker_stop_failures_test.go:39: one failure after the stop parked J-1 (two failures before it): &{ID:J-1 ... Status:parked Attempt:4 ... Reason:needs_person ...}`. The test posts the Ready control itself (reason "worker stopped", `Re` the exit) and asserts J-1 is `ready` before the next tick, so the tick's `cancelled` arm is not what clears the count. |
| S5 test: a refused reopen leaves no Handover record | HOLDS. `go test -count=1 ./internal/cli -run TestE1012` exit 0 on the lane. Fails on base: `--- FAIL: TestE1012RefusedReopenLeavesNoHandoverRecord` / `e1012_reopen_shoulds_test.go:64: the refused reopen left 2 handover records on the Feed, want 1`. The refusal is a `--reason` with a control character: the state move checks it in `Change`, after the writer and the task lease are taken. |
| go test ./internal/cli/... ./internal/orchestrator/... | HOLDS, exit 0: `ok .../internal/cli 38.014s`, `ok .../internal/orchestrator 91.126s`. |
| Packages that import them (go list over Imports, TestImports, XTestImports) | HOLDS, exit 0: cmd/grid (no test files), contract/formats, internal/account, internal/braincmd, internal/golden, internal/job/jobcmd, internal/jobgraph, internal/land, internal/mcp, internal/migrate, internal/packaging, internal/question, internal/settings, lib: all `ok` in the same run as the row above. |
| `make lint` | HOLDS, exit 0 (`go vet ./...`). `gofmt -l internal/cli internal/orchestrator` is empty. |
| Windows build `GOOS=windows CGO_ENABLED=0 go build ./...` | HOLDS, exit 0. |
| macOS build `GOOS=darwin CGO_ENABLED=0 go build ./...` | HOLDS, exit 0. |
| Walk tests `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0: `ok .../internal/cli 0.033s`. |
| `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0: `ok .../internal/job/lint 0.123s`. |
| No existing test edited or deleted | HOLDS: `git diff faf43707 --stat` lists two new test files and no changed test file. |
| No guard, gate, cap, hold or blocking hook added | HOLDS: the lane posts one Job fewer (S1), moves one record later (S5) and clears a counter (S4). |
| App walk tests (`make webui-test`, TestFvArgvCollects, TestEveryWiredMount, slices Imports) and `make realplugin` guest walk tables | not run, not asked (app repo; this lane touches the plugin only) |
| Whole-module `go test ./...` and the cross-repo set | not run, not asked (the orchestrator runs them at landing) |
| *RealPlugin* tests | not run, not asked (no row names one; nothing here needs the built plugin) |

## Behaviour changes to know

- A closed task whose history.md holds no `to_state: closed` receipt whose `what` is `stop` (a hand-edited task.md, or one closed before receipts were written) no longer gets the Continue Job on reopen. `grid task stop` and the app's task stop write that receipt.
- A task closed by `grid task close` after a stop, and then reopened, gets no Continue Job: the last move to closed was a close. The person who closes a task asks for it to stay done.

## NOT VERIFIED on a live screen or a live worker

- A real worker stop, restart and a failed attempt afterwards: S4 is proved on the plugin tick with the worker's Ready control posted by the test, not by the app worker.
- A task stopped from the app and reopened from the task page: the CLI path is proved; the app's reopen button was not run.

DONE
