# C-jw-deliver-after-approve: `grid deliver` works on a task whose plan was just approved

Base: main 750ac8de53646d727e0accdb0e44057b3c404fd9. Base check: on the base,
`internal/task/deliver.go:153` and `:226` both called
`NextState(t.State, "status", "waiting")` with no approval path, and
`internal/task/write.go:63` refused waiting to waiting. Delivery did not accept a
waiting task with a current approval, so the lane went ahead.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. approve, then `grid deliver "Result ready"` exits 0, task.md is waiting at step ship; on the base it exits non-zero with the write.go:63 message | HOLDS | `TestJwDaDeliverAfterApprove` (internal/cli/jw_deliver_after_approve_test.go:96). Base run below; lane run: `go test -count=1 -v -run 'TestJwDa\|Deliver\|Approve' ./internal/cli/ ./internal/task/` exit 0 |
| 2. the same with `--dry-run` (deliver.go:153 path) | HOLDS | `TestJwDaDryRunAfterApprove` (:107): exit 0 on the lane and task.md keeps its bytes; refused on the base (below) |
| 3. no approval, plan changed after approval, already delivered (with and without approval): refused with the write.go:63 message, task.md byte-equal | HOLDS | `TestJwDaRefusesWaitingWithoutCurrentApproval` (:123), four subtests. Each runs `deliver` and `deliver --dry-run`, wants exit 1 with `cannot status task in current state "waiting" to "waiting"` and compares task.md bytes. `delivery_test.go:282-288` still passes (internal/cli package run, exit 0) |
| 4. a running task delivers as before (approve, task status running, deliver) | HOLDS | `TestJwDaRunningDeliversAsBefore` (:157); it passes on base and lane |
| 5. row-5 test command 0 fail | HOLDS | `go test -count=1 -v -run 'TestJwDa\|Deliver\|Approve' ./internal/cli/ ./internal/task/` exit 0: `ok internal/cli 6.616s`, `ok internal/task 3.737s` (PASS lines below) |
| 5. `make lint` | HOLDS | `make lint` exit 0 |
| 5. `make cross` | HOLDS | `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| 5. `git diff main --stat` names only the three files | HOLDS for the code | Before this report: `docs/delivery.md`, `internal/cli/jw_deliver_after_approve_test.go`, `internal/task/deliver.go` (3 files, 206+, 7-). This report, `docs/lanes/C-jw-deliver-after-approve.md`, is a fourth file because the builder runbook requires it |
| Touches: one helper for both NextState checks | HOLDS | `delivery.nextState` deliver.go:263; called at deliver.go:154 (check, used for dry run and under the locks) and deliver.go:231 (the write) |
| Touches: docs/delivery.md :125-127 | HOLDS | docs/delivery.md:125-132 |
| No change to gates.go, write.go StateMoves or any record shape | HOLDS | `git diff main --stat` above |
| Rule: new tests fail on base, pass on lane | HOLDS | base run below |
| Rule: no existing test edited or deleted | HOLDS | only one new test file |
| Rule: no guard, gate, cap, hold or blocking hook | HOLDS | the change accepts one more move; it refuses nothing that main accepted |
| Rule: tests of changed packages and every package that imports them | HOLDS | see "Dependent packages" below, all `ok` |
| Rule: no sleep, no timers | HOLDS | the test file has none |
| Rule: `git merge main` before the last proof run | HOLDS | `git merge main`: "Already up to date." (main = origin/main = 750ac8de) |
| Rule: one-sentence commits, no attribution lines | HOLDS | `git log origin/main..HEAD` |
| RealPlugin tests | n/a | the brief names none |
| UI rows | n/a | no UI change |

### Base run (tests added in 93d2a643, before the fix in 1a75837c)

```
    jw_deliver_after_approve_test.go:98: deliver [Result ready]: exit 1 deliver: cannot status task in current state "waiting" to "waiting"
--- FAIL: TestJwDaDeliverAfterApprove (0.15s)
    jw_deliver_after_approve_test.go:113: deliver [--dry-run Result ready]: exit 1 deliver: cannot status task in current state "waiting" to "waiting"
--- FAIL: TestJwDaDryRunAfterApprove (0.05s)
    jw_deliver_after_approve_test.go:141: deliver [Result ready]: exit 1 deliver: cannot status task in current state "waiting" to "waiting"
--- FAIL: TestJwDaRefusesWaitingWithoutCurrentApproval (0.15s)
    --- PASS: TestJwDaRefusesWaitingWithoutCurrentApproval/no_approval (0.01s)
    --- PASS: TestJwDaRefusesWaitingWithoutCurrentApproval/plan_changed_after_approval (0.07s)
    --- FAIL: TestJwDaRefusesWaitingWithoutCurrentApproval/delivered_after_approval (0.05s)
    --- PASS: TestJwDaRefusesWaitingWithoutCurrentApproval/delivered_without_approval (0.02s)
--- PASS: TestJwDaRunningDeliversAsBefore (0.06s)
FAIL
FAIL	github.com/gridconsole/grid-plugins/internal/cli	0.428s
```

Rows 3 and 4 hold on the base by design: they show that what main refuses
(rows 3) and accepts (row 4) stays the same. The subtest "delivered after approval"
cannot reach its second deliver on the base, because the base refuses the first
deliver after approve. On the base, `delivery_test.go:282-284` covers the refusal
of a second deliver.

### Lane run (row 5)

```
--- PASS: TestJwDaDeliverAfterApprove (0.06s)
--- PASS: TestJwDaDryRunAfterApprove (0.05s)
--- PASS: TestJwDaRefusesWaitingWithoutCurrentApproval (0.18s)
    --- PASS: TestJwDaRefusesWaitingWithoutCurrentApproval/no_approval (0.02s)
    --- PASS: TestJwDaRefusesWaitingWithoutCurrentApproval/plan_changed_after_approval (0.07s)
    --- PASS: TestJwDaRefusesWaitingWithoutCurrentApproval/delivered_after_approval (0.07s)
    --- PASS: TestJwDaRefusesWaitingWithoutCurrentApproval/delivered_without_approval (0.03s)
--- PASS: TestJwDaRunningDeliversAsBefore (0.06s)
ok  	github.com/gridconsole/grid-plugins/internal/cli	6.616s
ok  	github.com/gridconsole/grid-plugins/internal/task	3.737s
```

### Dependent packages

`go list -test -f '{{.ImportPath}} {{join .Deps " "}}' ./...` filtered on
`internal/task` lists: cmd/grid, contract/formats, internal/account, braincmd,
cli, contract, env, golden, guard, hook, job, jobgraph, job/jobcmd, job/jobtest,
job/lint, land, lease, mcp, migrate, orchestrator, packaging, packaging/cmd,
plan, ports, question, settings, signal, updates, workflow, lib.
`go test -count=1` on internal/task and all of them: every package `ok` (or
`[no test files]`). This includes internal/cli 262.5s and internal/orchestrator
264.9s.

## What changed

- `internal/task/deliver.go`: new `delivery.nextState(t, data)` (:256-284). It
  asks the state table first, so every move main allowed is unchanged. A
  `waiting` task is accepted only when all of these hold: its task.md step is
  not `ship`; `keptApproval(t)` (gates.go:120) finds an approve receipt of this
  task in approval.json; and the receipt's `PlanHash` equals
  `plan.Hash(plan.md now)`, the comparison gates.go:92 makes. In every other
  case it returns the state table's own error, so the refusal text is unchanged.
  `check` (:154) and `write` (:231) both go through it. `write` now reads
  task.md just before the state move instead of just after; the bytes are the
  same, read under the same locks.
- `docs/delivery.md:125-132`: says which waiting task is delivered and which
  are still refused.
- `internal/cli/jw_deliver_after_approve_test.go`: rows 1-4 through the CLI
  harness. Its setup is `approveJobHarness` (approve_job_test.go:19) with the
  plan and proof of `deliveryFixture`, because delivery's proof check needs a
  plan step with proof.

## Choices to check

- Only the plan hash is compared, as the brief's decision says. The digest is
  not compared because `plan.Digest` also binds line one and step metadata
  (plan/hash.go:33-34), which can change during the build after the approval.
- `plan.Store.Read` is used, not `WithHash`: `write` already holds `.plan.lock`
  (deliver.go:105), and the dry run must take no lock (docs/delivery.md:110).
- The production helper is named `nextState`, not `jwDa…`. I read the brief's
  "Helpers start with `jwDa`" as applying to the new test file in the shared
  `cli` test package, where all the `jwDa…` helpers are.
- A task that was approved, sent back to running, and then put in `waiting` at
  some other point before step ship (a question, say) is also delivered,
  because its approval is still current. The decision allows this: a waiting
  task whose plan approval is current.

## NOT VERIFIED on a live screen

Nothing. There is no UI change.

## Open items

- The app rigs still move the task to running between approve and deliver
  (app realplugin_seed_test.go:67). That keeps working; dropping it is up to the
  app.
- A deliver receipt in history.md can now say `from_state: waiting`,
  `to_state: waiting`. Before this lane, a deliver came only from running,
  queued, stuck or reworking. The brief's caller check searched the app for the
  refusal text and for reads of the state after approve, not for history readers
  that expect a deliver to come from `running`. Every plugin package that depends
  on internal/task passes. This is a heads-up for app history readers; the app's
  ey_history_event rig still goes through `running`.

DONE
