# C-gu-replan-job-dead-asks-owner

Base: lane-C-gu-replan-after-reject (stacked; main d1c76719 plus the reject re-plan pass). Base check: on base, `gkReplanned` (internal/orchestrator/replan.go:69-76) counts a re-plan Job as done for good, so a re-plan Job that ends Cancelled or Rejected leaves the waiters of the first dead Job waiting and no pass posts anything again.

A re-plan Job (role orchestrator, `discovered_from` a Job D) that ends Cancelled or Rejected while Posted or Ready Jobs still hold a done edge to D gets one person Job for the task owner (`Store.Owner`). The pass posts person Jobs only, never a re-plan Job. It adds no refusal, gate, cap or limit on agents, cancels nothing and counts nothing; replan.go is not edited.

## Proof table

| # | Row / rule | Result |
|---|---|---|
| 1 | `go test ./internal/orchestrator/ -run GuDp -count=1 -v`, `TestGuDpWorkerCancelledReplan` | HOLDS, exit 0, `--- PASS: TestGuDpWorkerCancelledReplan`. `gkRig(t, []any{"J-1"})`, one tick gives J-3 (assigned in that same tick), `guDpAck` and `guDpExit(.., "cancelled")` for J-3, one tick: Jobs J-1..J-4, one person Job J-4: role Person, kind ask, intent.owner `owner`, skills [], discovered_from J-3, title `Decide how J-2 continue: re-plan Job J-3 of J-1 ended cancelled`, A1 holds J-1, J-3 and `J-2 (waiting for J-1, which was cancelled)`, `contract.Validate("job-record", body)` nil, every field equals `ReplanOwnerJobBody(...)`. internal/orchestrator/gu_replan_owner_test.go:148, helpers :14, :32, :95. |
| 2 | `TestGuDpControlRejectedReplan` | HOLDS, exit 0, PASS. J-3 closed by the control `{"target":"J-3","status":"rejected","reason":"r","close_reason":"r"}` through `f.post`: one person Job J-4, title ends `ended rejected` (gu_replan_owner_test.go:161). |
| 3 | `TestGuDpOnce` | HOLDS, exit 0, PASS. Four more ticks: Jobs J-1..J-4, one person Job (gu_replan_owner_test.go:172). |
| 4 | `TestGuDpNoWaiterLeft` (`waiter_cancelled_first`, `no_owner`) | HOLDS, exit 0, PASS. J-2 cancelled by a control before J-3 ends: Jobs J-1..J-3, no person Job. `f.s.Owner = ""`: same, and `f.tick` (which fails on an error) returns none. It is a negative test and passes on base by nature (see R2) (gu_replan_owner_test.go:187). |
| 5 | `go test ./internal/orchestrator/ -run 'Replan\|GuDp\|GuRj' -count=1 -v` | HOLDS. Every `TestGkReplan*` (7), `TestGqReplan*` (3), `TestGuRj*` (4) and `TestGuDp*` (6) printed PASS, `ok`; no gk, gq or gu file touched. The whole-package run (211s) passed before the last test was added. The pipe `\| grep` hid the exit code of the `-v` runs; the unpiped package runs in 7b printed `ok`. |
| 6 | `TestGuDpChainEndsAtPerson` | HOLDS, exit 0, PASS. J-1, J-2 after J-1, J-1 rejected by a control; one tick gives J-3 (base pass); J-3 closed cancelled by a control; next tick gives one person Job J-4 (A1 holds `J-2 (waiting for J-1, which was rejected)`); four more ticks leave exactly J-1..J-4, with `discovered_from` Jobs J-3 and J-4 only: no second re-plan Job, no second person Job (gu_replan_owner_test.go:235). |
| 7a | `git diff origin/main --stat` | HOLDS, besides the base lane's files: replan_owner.go (new), tick.go (+7 over the base lane's tick.go, 0 deleted), gu_replan_owner_test.go (new), this report. replan.go not edited. |
| 7b | Tests of changed packages and importers. `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...` gives as importers of internal/orchestrator: internal/cli, internal/migrate (and itself); of those, cmd/grid, contract/formats, internal/account, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test -count=1` on orchestrator, cli, migrate, cmd/grid, then on the other nine | HOLDS, exit 0, every package `ok` (orchestrator 211.9s, cli 121.4s, migrate 23.3s; cmd/grid has no test files). |
| 7c | `gofmt -l internal/orchestrator/` and `make lint` (`go vet ./...`) | HOLDS, exit 0, nothing printed. |
| R1 | Windows and macOS builds: `make cross` (vet and test compile for darwin/arm64 and windows/amd64) | HOLDS, exit 0, every package `ok`. |
| R2 | New tests fail on base | HOLDS for the positive tests. With the `guReplanAskOwner` call taken out of tick.go (`git checkout -- internal/orchestrator/tick.go` after): `TestGuDpWorkerCancelledReplan: want one person Job J-4, got map[] (Jobs [J-1 J-2 J-3])`; `TestGuDpControlRejectedReplan: ... got map[] (Jobs [J-1 J-2 J-3])`; `TestGuDpOnce: ... got map[] (Jobs [J-1 J-2 J-3])`; `TestGuDpChainEndsAtPerson: ... got map[] (Jobs [J-1 J-2 J-3])`; FAIL (read through a pipe, exit code not seen). `TestGuDpNoWaiterLeft` is a negative test: it passes on base by nature. |
| R3 | Stale-snapshot trap | HOLDS. With the `Load(s.Feed)` before `guReplanAskOwner` removed (then `git checkout -- internal/orchestrator/tick.go`): `TestGuDpSameTickAsRejectReplan: Jobs on the Feed = [J-1 J-2 J-3 J-4 J-5 J-6], want [J-1 .. J-7]`: the person Job reused J-6 and the fold dropped it without an error. With the reload it passes. |
| R4 | No guard, gate, cap or hold on agents; no existing test edited; nothing deleted | HOLDS. One Job post per dead re-plan Job; tick.go gains 7 lines, none removed. |
| R5 | Commits and public repo | HOLDS: one-sentence messages, no attribution lines, no personal path, host or email in the diff. |
| R6 | `*RealPlugin*` tests | Not named by the brief; none run, none claimed. |

## What changed

- internal/orchestrator/replan_owner.go (new): `Store.guReplanAskOwner` returns at once for an empty (or blank) `Store.Owner` (comment says so; `park` in events.go refuses the same case with an error, this pass skips so a tick never fails for it). It walks `snap.Items`; for an item with role orchestrator (EqualFold) and a `discovered_from` D, whose folded status is Cancelled or Rejected, with `gkWaiting(snap, D)` not empty and no person Job (`IsPerson`) discovered_from that item, it posts one person Job with id `gkNextJobID(snap)` and reloads the snapshot. `ReplanOwnerJobBody` (exported, like `ReplanRejectJobBody`) starts from `ReplanJobBody` and sets role `Person`, kind `ask`, skills `[]`, intent.owner the owner, the title, one A1 row (names D, the re-plan Job, the status it ended and each waiter with `jobgraph.DoneReason(D, status of D)`), `discovered_from` the re-plan Job, and `posts_back` `[]`. No `job:` pointer.
- internal/orchestrator/tick.go: after `guReplanRejected`, `snap, e = Load(s.Feed)` then `s.guReplanAskOwner(snap)`. The reload is needed (R3).
- internal/orchestrator/gu_replan_owner_test.go (new): rows 1 to 4 and 6, plus `TestGuDpSameTickAsRejectReplan` for R3; helpers `guDpAck`, `guDpExit`, `guDpClose`, `guDpPeople`, `guDpRig`, `guDpWantOwnerJob`.

Once-only comes from the person Job on the Feed, read from `snap.Items`; no memory field, file or counter was added.

## Notes for the reviewer

- The brief says J-3 is "assigned by the next tick". It is assigned by the same tick that posts it (Ready is computed after the re-plan passes, tick.go:104). `guDpRig` therefore acks J-3 after one tick; a second tick changed nothing (tests passed with and without it).
- `guReplanAskOwner` does not check D's status; `ReplanOwnerJobBody` passes it to `DoneReason`, which says "to be accepted" for a D that is neither Cancelled nor Rejected. Only a hand-posted orchestrator Job can have such a D; the passes that post re-plan Jobs act on Cancelled and Rejected Jobs only.
- A person Job that is itself cancelled or rejected is the next lane (C-gu-replan-person-cancelled): this pass does nothing for it.
- Two re-plan Jobs of the same D, both dead, each get one person Job (each is a distinct `discovered_from`).

## NOT VERIFIED on a live screen

No UI row. Not run: a real binary posting the hand-written control and the Feed read by the app; `grid job lint` or `grid job render` of the person Job body (the body passes `contract.Validate("job-record")`; the base lane's internal/cli/gu_replan_reject_render_test.go is the template if the reviewer wants a lint row, but the brief's Touches names no cli file). Whether the app shows kind `ask` Jobs that have no question record behind them.
