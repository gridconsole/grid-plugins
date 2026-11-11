# C-gk-replan-after-cancel

Base: 2d92aa42fcbf20c2491c6ca34558fcac9d56a911 (main). Base check: `git grep -n -i 'replan\|re-plan' -- internal/orchestrator ':!*_test.go'` printed nothing, so the loop posted no Job after a cancel on base.

A Job whose worker wrote a worker-exit `cancelled` while Posted or Ready Jobs hold a done edge to it gets one orchestrator Job that re-plans. The loop adds no refusal, gate, cap or limit, cancels nothing, changes no edge kind and counts no re-plans.

## Proof table

| # | Row / rule | Result |
|---|---|---|
| 1 | `go test -count=1 -v -run 'TestGkReplanAfterCancel$' ./internal/orchestrator/` | HOLDS, exit 0, `--- PASS: TestGkReplanAfterCancel`. Checks: Jobs on the Feed are J-1 J-2 J-3; J-3 role orchestrator, skills `["plan-jobs"]`, `discovered_from` J-1, links empty, no `review` key, verify path `task:task.md`, A1 text holds `J-2` and `waiting for J-1, which was cancelled`, no string starts with `job:`, `contract.Validate("job-record", body)` nil, and every field equals `ReplanJobBody("J-3","J-1",["J-2"])`. |
| 2 | `-run TestGkReplanOnce` | HOLDS, exit 0, PASS. Four ticks after the exit: one job record with `discovered_from` J-1, J-2 not Cancelled. |
| 3 | `-run TestGkReplanAfterRestart` | HOLDS, exit 0, PASS. Exit and hand-posted Cancelled control (`Re` the exit), no tick between; one tick of a Store copy posts J-3, the second posts none. |
| 4 | `-run TestGkReplanNotForControlCancel` | HOLDS, exit 0, PASS. J-1 Cancelled by control only, exit reason empty, two ticks, no job record has `discovered_from`. |
| 5 | `-run 'TestGkReplanNoWaiter\|TestGkReplanFailedEdge\|TestMoves'` | HOLDS, exit 0, all PASS. `TestMoves/cancelled` (orchestrator_test.go:123) passes unchanged. |
| 6 | `go test -count=1 -v -run TestGkReplanBodyPassesRenderLint ./internal/cli/` | HOLDS, exit 0, PASS. No `jobs/J-2`, `J-4`, `J-6` folder; `postFindings` has no error and `lint.HasErrors` is false; `contract.Validate` nil; the body is posted on the task Feed under the task lease; `grid job render <path> J-7 --json` exits 0 and `jobs/J-7/brief.md` exists. |
| 7a | `git diff main --stat` | HOLDS: gk_replan_render_test.go, gk_replan_after_cancel_test.go, replan.go, tick.go (3 lines), plus this report. Nothing else. |
| 7b | `go test -count=1 ./internal/orchestrator/ ./internal/jobgraph/ ./internal/cli/ ./internal/migrate/` | HOLDS, exit 0, four `ok`. |
| 7c | `gofmt -l internal` | HOLDS, exit 0, prints nothing. |
| 7d | `make lint` (`go vet ./...`) | HOLDS, exit 0. |
| R1 | Packages that import the changed ones (`go list -f` over Imports, TestImports, XTestImports) | importers of internal/orchestrator: internal/cli, internal/migrate (both tested above). Importers of internal/cli: cmd/grid, contract/formats, internal/account, internal/golden, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test -count=1` on them: exit 0, all `ok` (cmd/grid has no test files). |
| R2 | Windows and macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| R3 | New tests fail without the change | HOLDS for the positive tests. With the one `s.gkReplan(snap)` call removed from tick.go (then `git checkout -- internal/orchestrator/tick.go`): `go test -count=1 -run TestGkReplan ./internal/orchestrator/` exit 1: `TestGkReplanAfterCancel: Jobs on the Feed = [J-1 J-2], want [J-1 J-2 J-3]`; `TestGkReplanOnce: want one re-plan Job J-3 from J-1, got map[] (Jobs [J-1 J-2])`; `TestGkReplanAfterRestart: after one tick want one re-plan Job J-3, got map[] (Jobs [J-1 J-2])`; `TestGkReplanNamesEveryDoneWaiter: want one re-plan Job J-6, got map[] (Jobs [J-1 J-2 J-3 J-4 J-5])`. On the real base the test files do not compile (no `ReplanJobBody`). `TestGkReplanNotForControlCancel`, `TestGkReplanNoWaiter` and `TestGkReplanFailedEdge` are negative tests: they pass on base too, by nature. |
| R4 | Render test is not vacuous | HOLDS. With `"links": []any{"job:" + cancelled}` put into the body for one run (then `git checkout -- internal/orchestrator/replan.go`): `TestGkReplanBodyPassesRenderLint: render would refuse the re-plan Job: [{Rule:job.record ... $.links[0]: expected object}]`, exit 1. `git status --short` is clean. |
| R5 | No guard, gate, cap or hold on agents; no edit of an existing test; nothing deleted | HOLDS. The change posts one Job record; no existing test file edited; no line removed (450 insertions, 0 deletions). |
| R6 | Commits | HOLDS: one-sentence messages, no attribution lines, no personal paths, hosts or emails in the diff. |
| R7 | `*RealPlugin*` tests | Not named by the brief; none run, none claimed. |

## What changed

- internal/orchestrator/replan.go (new): `Store.gkReplan` walks the Jobs in id order. For each Job with Status Cancelled and ExitReason `cancelled` it collects the Posted or Ready Jobs with a done edge to it (an empty kind or kind `done`; a gate edge, a `failed` edge and a `children-done` edge are not collected). With at least one waiter, and no item with role `orchestrator` and `discovered_from` that Job, it posts one `job` record with `s.Feed.Post` under the task lease and reloads the snapshot, so a second cancelled Job in the same pass gets the next id. `gkNextJobID` copies the counting rule of internal/cli `nextJobID`. `ReplanJobBody` (exported) builds the body: fields of `firstJob` except the ones the brief lists; no `review` key; no `job:` pointer; one acceptance row A1 with the text of the brief.
- internal/orchestrator/tick.go: one call, `s.gkReplan(snap)`, after the reload and before `s.proposals`. `advance` and the `cancelled` case are not edited. `proposals` runs on the snapshot from before the post; the reload that follows it picks the new Job up, and the new id is top level, so it cannot collide with a `J-<n>.<k>` child.
- internal/orchestrator/gk_replan_after_cancel_test.go (new): the tests of rows 1 to 5, plus `TestGkReplanNamesEveryDoneWaiter` (not in the brief): a bare-id waiter and an explicit `done` waiter are named in the title; a `failed` waiter and a waiter on another Job are not.
- internal/cli/gk_replan_render_test.go (new): row 6.

Once only comes from the re-plan Job record on the Feed, read from `snap.Items`; no memory field, file or counter was added.

## Notes for the reviewer

- An explicit `done` edge needs a `why` in the job-record schema; a bare id does not. The tests use both.
- A person Job (`IsPerson`) that waits on the cancelled Job counts as a waiter; it is Posted or Ready and holds the edge.
- A re-plan Job that is itself cancelled by its worker, with nothing waiting on it, posts nothing. If Jobs wait on it, it gets its own re-plan Job.
- Budget: the lane used about 63 tool calls against 60 and made no advisor call; the last calls were the report, the commit and the bundle.

## NOT VERIFIED on a live screen

- No live run: no real worker cancelled a Job and no real orchestrator Job started from the posted body. Row 6 proves lint and `job render` of the body; it does not prove an agent follows A1.
- The wording of the title and of A1 was not read by an agent.
