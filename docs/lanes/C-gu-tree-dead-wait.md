# C-gu-tree-dead-wait: `grid job tree` names a Job that waits on a rejected or cancelled Job

Lane branch lane-C-gu-tree-dead-wait, from main d1c76719. Code commits 095f42a4, aecd40d0, 9f24ded6.

## Proof table

| Row / rule | Status | Proof |
|---|---|---|
| 1. TestGuDwRejectedTarget: J-2 after a rejected J-1 has Orphan dead_wait, Detail "waiting for J-1, which was rejected", Since the closing control's time; J-1 has no orphan | HOLDS | `go test ./internal/orchestrator/ -run 'Orphan\|Tree\|GuDw' -count=1 -v` exit 0, `--- PASS: TestGuDwRejectedTarget`; test at internal/orchestrator/gu_tree_dead_wait_test.go:48 |
| 2. TestGuDwCancelledTarget: Detail "waiting for J-1, which was cancelled"; a waiter with a `failed` edge on the cancelled J-1 has no dead_wait | HOLDS | same run, `--- PASS: TestGuDwCancelledTarget`; test at gu_tree_dead_wait_test.go:61 (J-3 carries the `failed` edge) |
| 3. TestGuDwLiveTargetNone; TestOrphanNotWhileWaiting and TestOrphanAbsentRowJSONUnchanged pass unchanged | HOLDS | same run: `--- PASS: TestGuDwLiveTargetNone`, `--- PASS: TestOrphanNotWhileWaiting`, `--- PASS: TestOrphanAbsentRowJSONUnchanged`; `git diff d1c76719 --stat` shows tree_orphan_test.go untouched |
| 4. TestGuDwTreeLine: `grid job tree` prints "  orphan dead_wait since " after `job cancel J-1 --reason dropped` | HOLDS | `go test ./internal/orchestrator/ ./internal/cli/ ./internal/job/lint/ -run 'Orphan\|Tree\|GuDw\|Orchestrator' -count=1` exit 0 (`ok .../internal/cli`); test at internal/cli/gu_tree_dead_wait_cli_test.go:12. The harness accepted the `job cancel` verb, so the `job post --type control` fallback was not needed. No time.Sleep, no clock. |
| 5. TestGuDwAgentText; TestOrchestratorNamesTheOrphanKinds still passes | HOLDS | same command, `ok .../internal/job/lint`; test at internal/job/lint/gu_dead_wait_text_test.go:10; agents/orchestrator.md:23 names `dead_wait` |
| 6. `go test ./internal/orchestrator/ ./internal/cli/ ./internal/job/lint/ -run 'Orphan\|Tree\|GuDw\|Orchestrator' -count=1` | HOLDS | exit 0: `ok .../internal/orchestrator 3.561s`, `ok .../internal/cli 1.395s`, `ok .../internal/job/lint 0.124s` |
| New tests fail on base and pass on the lane | HOLDS | base write.go and agents/orchestrator.md checked out from d1c76719 plus a temporary one-line `const OrphanDeadWait` stub (removed with `git clean -f`, `git status --short` empty after): `go test ... -run GuDw` exit 1, lines below |
| No existing test edited or deleted | HOLDS | `git diff d1c76719 --stat` lists only write.go, agents/orchestrator.md and the three new test files plus this report |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | the change adds a report value in the tree only; nothing acts on it |
| Edits only the brief's files | HOLDS | write.go, agents/orchestrator.md, three new test files, this report |
| New helpers carry the prefix guDw | HOLDS | guDwClose, guDwClosedAt, guDwWant in gu_tree_dead_wait_test.go; tests TestGuDw*; the new `OrphanDeadWait` const, `deadDoneWait` and `closedAt` in write.go are production names the brief asks for (the const) or support it |
| Tests of changed packages and of every importer | HOLDS | importers (`go list`): internal/cli, internal/migrate (and orchestrator itself). `go test ./internal/orchestrator/ ./internal/cli/ ./internal/migrate/ ./internal/job/lint/ -count=1` exit 0: orchestrator 72.388s, cli 63.411s, migrate 12.883s, lint 0.313s |
| Lint | HOLDS | `make lint` exit 0 (go vet ./..., gofmt -l . printed nothing) |
| Windows and macOS builds | HOLDS | `make cross` exit 0: vet and test compile for darwin/arm64 and windows/amd64, every package ok |
| *RealPlugin* tests | n/a | no row names one; none run |
| Whole-module test and cross-repo set | not run | the orchestrator runs them at landing |

Failing lines on base (new tests, base write.go and agents/orchestrator.md):

```
--- FAIL: TestGuDwRejectedTarget (0.39s)
    gu_tree_dead_wait_test.go:53: J-2: want orphan dead_wait since 2026-11-11T10:05:41.232Z, got <nil>
--- FAIL: TestGuDwCancelledTarget (0.04s)
    gu_tree_dead_wait_test.go:67: J-2: want orphan dead_wait since 2026-11-11T10:05:41.281Z, got <nil>
--- FAIL: TestGuDwClosedWaiterNone (0.07s)
    gu_tree_dead_wait_test.go:104: want dead_wait before anything else: <nil>
--- FAIL: TestGuDwTreeLine (0.03s)
    gu_tree_dead_wait_cli_test.go:29: text tree:
        J-1  builder  cancelled
        J-2  builder  posted
--- FAIL: TestGuDwAgentText (0.00s)
    gu_dead_wait_text_test.go:14: ../../../agents/orchestrator.md does not contain "`dead_wait`"
```

TestGuDwLiveTargetNone passes on base too: it pins what must not change (a live wait, a gate, an accepted target are not dead waits), as rows 3 asks.

## What changed

- internal/orchestrator/write.go: new `OrphanDeadWait = "dead_wait"` beside the other kinds. TreeAt collects, for each Job, the first entry of `readiness.Waiting` whose Target is a Job (not `gate:`) that is Rejected or Cancelled and whose Reason equals `jobgraph.DoneReason(target, status)` (`deadDoneWait`). A Posted or Ready Job that `orphanOf` left without an orphan then gets `Orphan{Kind: "dead_wait", Since: closing control time, Detail: the wait reason}`. `closedAt` reads the target's closing control as ready.go:135-139 reads it (the latest control in its History whose status is its status). A Job `orphanOf` already names keeps that orphan. A `failed` edge on a cancelled Job is met, so it is never in Waiting as a dead wait. When a Job has several dead waits the first in the sorted Waiting order (by target id) is named.
- agents/orchestrator.md:23: names `dead_wait` (waits through a done edge on a Job that was rejected or cancelled) and says to cancel the waiting Job or replace it with one that does not wait on the dead Job.
- New tests in the three files the brief names.
- Data format: `orphan.kind` in `grid job tree --json` gains the value `dead_wait`. No schema under contract/ lists the kinds; no app code reads them (brief).

## NOT VERIFIED on a live screen

Nothing here is a UI row. The orchestrator agent's behaviour on a `dead_wait` line (whether it cancels or replaces the Job) needs a live agent run; the tests prove the tree line and the agent text, not the agent's decision.
