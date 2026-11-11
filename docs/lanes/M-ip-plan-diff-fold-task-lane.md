# M-ip-plan-diff-fold-task-lane

## What conflicted and how it was resolved
`git merge origin/lane-C-he-plan-diff-feed-fold` (the lane had not landed yet) conflicted in
`internal/cli/gg_unknown_task_walk_test.go`, one hunk in `ggTaskWalked`. Main added `"task lane"` before
`"task move"`; C-he dropped `"plan diff"` and `"feed fold"`. Resolution keeps both: `"task lane"` present,
`"plan diff"` and `"feed fold"` absent. Everything else auto-merged. `make generate` changed no file.
A second `git merge origin/main` said "Already up to date".

## Proof table
| # | Row | Result |
|---|---|---|
| 1 | Ancestors | HOLDS: `git merge-base --is-ancestor` exit 0 for origin/main, origin/lane-C-he-plan-diff-feed-fold, origin/lane-C-gr-plugin-task-lane-shared-rows |
| 2 | Walk list | HOLDS: grep for `"plan diff"`/`"feed fold"` in the walk test and commands.json prints nothing; `internal/cli/gg_unknown_task_walk_test.go:24` is the one `"task lane", "task move"` line; no `<<<<<<<` markers |
| 3 | Generated contract current | HOLDS: `make generate` exit 0, `git status --short` empty |
| 4 | Lane rows | HOLDS: `go test -count=1 -v -run 'TestHePf\|TestGrTaskLane\|TestFvGuestTaskVerbs\|TestGgUnknownTaskWalk' ./internal/cli/` ok. PASS lines: `TestHePfRemovedSubs`, `TestGrTaskLaneMovesAndKeepsTheRest`, `TestGrTaskLaneRefusals`, `TestFvGuestTaskVerbs`, `TestGgUnknownTaskWalkCoversTheContract` ("contract members: 80, walked: 80, skipped: 0"), `TestGgUnknownTaskWalk`, `TestGgUnknownTaskWalk/task_lane`; no SKIP |
| 5 | C-he's other rows | HOLDS: ok for `-run 'TestPlan\|TestFeed\|...\|TestContractCommandsParserInventory' ./internal/cli/ ./internal/plan/`; `go test -count=1 ./internal/contract/` ok; `TestGoReadmeSubverbsInCommandsJSON\|TestSkillCommandsImplemented` ok in contract and packaging |
| 6 | Touched packages | HOLDS: `go test -count=1 ./internal/cli/ ./internal/plan/ ./internal/task/` ok; `go vet` of the three exit 0; `gofmt -l internal/cli` empty |
| R | Importers' tests | HOLDS: ok for approvefix, job/..., jobgraph, land, orchestrator, proof, braincmd, guard, hook, signal, lib, cmd/... (importers of cli, plan, task from `go list`) |
| R | Lint, Windows, macOS | HOLDS: `make lint` exit 0; `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| R | New tests | No new tests; the lane only resolves a merge conflict. |
| R | RealPlugin tests | Not needed by any row. |
| R | No guard/gate on agents | HOLDS: none added |

## What changed
Merge commit with the conflict resolved in the walk test. No other file.

## NOT VERIFIED on a live screen
Nothing here has UI.
