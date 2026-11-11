# C-gu-replan-after-reject

Base: d1c76719044999b8cb0bbff892c105e21801676b (main). Base check: `gkReplan` (internal/orchestrator/replan.go:30) acts only on `Status == Cancelled && ExitReason == "cancelled"`, so on base a Rejected Job with a done-edge waiter got no Job from any pass.

A Job closed Rejected (a control with status rejected and a close_reason) while Posted or Ready Jobs hold a done edge to it gets one orchestrator Job that re-plans. It reuses `gkWaiting`, `gkReplanned` and `gkNextJobID`; `replan.go` is not edited. The pass adds no refusal, gate, cap or limit, cancels nothing, changes no edge kind and counts no re-plans.

## Proof table

| # | Row / rule | Result |
|---|---|---|
| 1 | `go test -count=1 -v -run 'TestGuRjReplanAfterReject$' ./internal/orchestrator/` | HOLDS, exit 0, `--- PASS: TestGuRjReplanAfterReject`. Jobs on the Feed are exactly J-1 J-2 J-3; J-3 role orchestrator, `discovered_from` J-1, title `Re-plan after J-1 was rejected: J-2 wait on it`, A1 text holds `J-2` and `waiting for J-1, which was rejected`, no string starts with `job:`, `contract.Validate("job-record", body)` nil, and every field equals `ReplanRejectJobBody("J-3","J-1",["J-2"])`. Test: internal/orchestrator/gu_replan_reject_test.go:21. |
| 2 | `-run TestGuRjOnce` | HOLDS, exit 0, PASS. Four ticks after the reject: one job record with `discovered_from`, J-3 from J-1; J-2 not Cancelled (gu_replan_reject_test.go:81). |
| 3 | `-run TestGuRjNoDoneWaiter` | HOLDS, exit 0, PASS (`no_waiter`, `failed_edge`). Rejected J-1 with no waiter: one Job on the Feed. Rejected J-1 whose only waiter holds a `failed` edge: two Jobs, none new (gu_replan_reject_test.go:100). |
| 4 | `go test -count=1 -v -run TestGuRjBodyPassesRenderLint ./internal/cli/` | HOLDS, exit 0, PASS. `postFindings(h.env, path, ReplanRejectJobBody("J-7","J-2",["J-4","J-6"]))` has no error and `lint.HasErrors` is false; no `jobs/J-2`, `J-4`, `J-6` folder; `contract.Validate` nil (internal/cli/gu_replan_reject_render_test.go:17). |
| 5 | `go test ./internal/orchestrator/ ./internal/cli/ -run 'Replan\|GuRj' -count=1` (run unpiped; the earlier `-v` runs of rows 1 to 4 were read through a pipe) | HOLDS, exit 0, `ok` for both packages, which holds rows 1 to 4 as well since their tests match `GuRj`. All seven `TestGkReplan*`, the three `TestGqReplan*` and `TestGkReplanBodyPassesRenderLint`/`TestGqReplanBodyWithCausePassesRenderLint` pass, no gk or gq test file touched. |
| 6 | `-run TestGuRjSameTickAsWorkerCancel` | HOLDS, exit 0, PASS. `gkRig(t, []any{"J-1"})`, then J-3, J-4 after J-3, J-3 rejected, one tick: Jobs J-1..J-6, re-plan Jobs J-5 (from J-1) and J-6 (from J-3) (gu_replan_reject_test.go:130). |
| 7a | `git diff origin/main --stat` | HOLDS: replan_reject.go (new, 60), tick.go (+7, 0 deleted), gu_replan_reject_test.go (new), gu_replan_reject_render_test.go (new), plus this report. replan.go not edited. |
| 7b | Tests of changed packages and every importer. `go list -f` over Imports, TestImports, XTestImports of internal/orchestrator and internal/cli gives: cmd/grid, contract/formats, internal/account, internal/cli, internal/golden, internal/jobgraph, internal/mcp, internal/migrate, internal/orchestrator, internal/packaging, internal/question, internal/settings, lib. `go test -count=1` on all | HOLDS, exit 0, every package `ok` (cmd/grid has no test files). |
| 7c | `gofmt -l internal/orchestrator internal/cli` | HOLDS, exit 0, prints nothing. |
| 7d | `make lint` (`go vet ./...`) | HOLDS, exit 0. |
| R1 | Windows and macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| R2 | New tests fail without the change | HOLDS for the positive tests. With tick.go as on base (`git checkout HEAD~1 -- internal/orchestrator/tick.go`, then `git checkout HEAD -- internal/orchestrator/tick.go`): `go test -count=1 -run GuRj ./internal/orchestrator/` printed FAIL (output read through a pipe, so the exit code was not seen): `TestGuRjReplanAfterReject: Jobs on the Feed = [J-1 J-2], want [J-1 J-2 J-3]`; `TestGuRjOnce: want one re-plan Job J-3 from J-1, got map[] (Jobs [J-1 J-2])`; `TestGuRjSameTickAsWorkerCancel: want two re-plan Jobs, got map[J-5:...] (Jobs [J-1 J-2 J-3 J-4 J-5])`. On the real base the test files do not compile (no `ReplanRejectJobBody`). `TestGuRjNoDoneWaiter` is a negative test: it passes on base by nature. |
| R3 | Stale-snapshot trap is caught | HOLDS. With the `Load(s.Feed)` before `guReplanRejected` removed in tick.go (then `git checkout HEAD -- internal/orchestrator/tick.go`): `TestGuRjSameTickAsWorkerCancel: want two re-plan Jobs, got map[J-5:...] (Jobs [J-1 J-2 J-3 J-4 J-5])`, printed FAIL: the post that reused J-5 was dropped by the fold. `git status --short` is clean after. |
| R4 | No guard, gate, cap or hold on agents; no edit of an existing test; nothing deleted | HOLDS. The pass posts one Job record; no existing file edited except tick.go (7 lines added, 0 removed). |
| R5 | Commits | HOLDS: one-sentence message, no attribution lines, no personal paths, hosts or emails in the diff. |
| R6 | `*RealPlugin*` tests | Not named by the brief; none run, none claimed. |

## What changed

- internal/orchestrator/replan_reject.go (new): `Store.guReplanRejected` walks the Jobs in id order. For each Job with Status Rejected it takes `gkWaiting` (Posted or Ready Jobs with a bare or `done` edge to it); with at least one waiter and no `gkReplanned` item it posts one `job` record with id `gkNextJobID(snap)` and reloads the snapshot, so a second rejected Job in the same pass gets the next id. `ReplanRejectJobBody` (exported) calls `ReplanJobBody(id, rejected, "", waiting)` and sets the title to `Re-plan after <rejected> was rejected: <waiting> wait on it` and A1 text built from `jobgraph.DoneReason(rejected, contract.Rejected)`. Role orchestrator, `discovered_from` the rejected Job, no `job:` pointer. The sentence "each of ... is cancelled with a reason" keeps `cancelled`: it is what the waiters become, not the status of the target.
- internal/orchestrator/tick.go: after the `gkReplan` call, `snap, e = Load(s.Feed)` and `s.guReplanRejected(snap)`. The reload is needed: `gkReplan` reloads only its own copy, so `gkNextJobID` on the old snapshot returns an id `gkReplan` just used and the fold drops the post without an error (internal/contract/fold.go:138-140). R3 proves it.
- internal/orchestrator/gu_replan_reject_test.go (new): rows 1, 2, 3, 6; helper `guRjReject`.
- internal/cli/gu_replan_reject_render_test.go (new): row 4. It stops at lint and `contract.Validate`, as the brief asks; it does not post the body on a Feed or render it.

Once-only comes from the re-plan Job on the Feed, read from `snap.Items`; no memory field, file or counter was added. A Job rejected after its re-plan Job already exists gets no second one, also when the waiters changed.

## Notes for the reviewer

- `ids(snap)` sorts as strings (J-10 before J-2). The loop reloads after each post, so the order does not change any id chosen.
- tick.go is shared with C-gu-replan-job-dead-asks-owner and C-gu-replan-person-cancelled: each adds one call after this one; the later two rebase over a one-line conflict. If C-gq-replan-shows-cause lands first, `ReplanJobBody` keeps the cause argument and this file already passes `""`.

## NOT VERIFIED on a live screen

Nothing here has a UI row. Not run: a real binary posting a hand-written rejecting control and a live `grid job render` of the re-plan Job (`grid job render` of a re-plan body is covered for the cancel body by TestGkReplanBodyPassesRenderLint; the reject body differs only in title and A1 text, and row 4 lints it).
