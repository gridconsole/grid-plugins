# M-ju-job-order-person-list-onto-main

`job order` and `person list` removal (C-he-job-order-person-list) reaches main together with the two main rows that still called the removed verbs.

## What conflicted and how it was resolved

Nothing conflicted. `git merge origin/lane-M-ip-plan-diff-fold-task-lane` merged clean (ort); `git merge origin/main` merged clean, including `ggTaskWalked` in internal/cli/gg_unknown_task_walk_test.go. The step 4 re-fetch and `git merge origin/main` printed `Already up to date.`

M-ip-plan-diff-fold-task-lane is already in origin/main (`git diff --name-only origin/main...origin/lane-M-ip-plan-diff-fold-task-lane` is empty), so that merge brought in the stale part of the branch only.

## What changed

- lib/ha_question_parity_test.go:229: the row `list owner` (exit 0, `"people"`) is now `list removed` (exit 2, `"code":"invalid_argument"`). lib and binary still have to give the same answer (haQsSame is untouched).
- internal/cli/inprocess.go: deleted `"order"` from the `job` subs and `"fold"` from the `feed` subs. gofmt applied. Nothing else in the table changed.
- Everything else is the carried C-he-job-order-person-list lane.

## Proof table

| # | Row | Result |
|---|---|---|
| 1 | Main and carried lanes in the branch | HOLDS. `git merge-base --is-ancestor <ref> HEAD` exit 0 for origin/main, origin/lane-C-he-job-order-person-list, origin/lane-C-he-plan-diff-feed-fold, origin/lane-M-ip-plan-diff-fold-task-lane. `git grep -n -e '^<<<<<<<' -e '^>>>>>>>' -- internal contract lib testdata` prints nothing. |
| 2 | Removed verbs walked and called nowhere; `task lane` walked | HOLDS. `git grep -n -E '"plan diff"\|"feed fold"\|"job order"\|"person list"' -- internal/cli/gg_unknown_task_walk_test.go internal/cli/guest_parity_test.go internal/cli/contract_inventory_test.go contract/commands.json` prints nothing. `git grep -n '"task lane", "task move"' -- internal/cli/gg_unknown_task_walk_test.go` prints `internal/cli/gg_unknown_task_walk_test.go:24:	"task lane", "task move", ...`. `git grep -n -E '"person", "list"' -- lib` prints only `lib/ha_question_parity_test.go:229`. |
| 3 | The lanes' own rows on the merged tree | HOLDS. `go test -count=1 -v -run 'TestHeJp\|TestHePf\|TestGrTaskLane\|TestGgUnknownTaskWalk\|TestPerson\|TestGmJob\|TestGmPlan\|TestGuestProjectParity\|TestContractCommandsParserInventory\|TestContractCommandsInventory' ./internal/cli/` exit 0: `--- PASS: TestHeJpRemovedSubs (0.25s)`, `--- PASS: TestHePfRemovedSubs (0.01s)`, `--- PASS: TestGrTaskLaneMovesAndKeepsTheRest`, `TestGgUnknownTaskWalkCoversTheContract` (79 walked, 0 skipped). The only SKIP in the run is TestPersonalCaseVariantTaskPathDarwin (Darwin only). `go test -count=1 ./internal/orchestrator/ ./internal/plan/` exit 0 (`ok ... orchestrator 132.246s`, `ok ... plan 0.133s`). |
| 4 | `person list` row pins the removal | HOLDS. `go test -count=1 -v -run TestHaQsPerson ./lib/` exit 0: `--- PASS: TestHaQsPerson (0.62s)`. The rows of TestHaQsPerson are not t.Run subtests, so there is no separate `list removed` sub-result line; the row name is in the failure message. With the old row put back in the working tree only: `ha_question_parity_test.go:245: list owner: exit 2, want 0; stdout "{\"error\":{\"code\":\"invalid_argument\",\"message\":\"person: unknown command \\\"list\\\"\"}}\n"` and `--- FAIL: TestHaQsPerson`, exit 1. Then `git checkout -- lib/ha_question_parity_test.go`; `git status --short` was clean. |
| 5 | Package tests, generated files | HOLDS. `go test -count=1 ./internal/cli/ ./internal/contract/ ./internal/packaging/ ./lib/ ./cmd/grid/` exit 0 (cli 91.153s, contract 2.915s, packaging 13.414s, lib 34.774s, cmd/grid no test files). Packages importing the changed ones (go list): internal/cli is imported by cmd/grid and lib; lib is imported by nothing. `go test -count=1 -v -run TestHsIpWalksEveryCommand ./internal/cli/` exit 0: `--- PASS: TestHsIpWalksEveryCommand (0.00s)` ("walked 39 commands, 39 table entries"). `make generate` exit 0, then `git diff --exit-code contract/commands.json testdata/golden` exit 0 and `git status --short` empty. `make golden` exit 0 (`ok .../internal/golden 0.610s`). TestGoReadmeSubverbsInCommandsJSON and TestSkillCommandsImplemented are in the passing internal/contract and internal/packaging runs. |
| 6 | Lint, cross, nothing else changed | HOLDS. `make lint` (go vet ./...) exit 0. `make cross` (darwin/arm64 and windows/amd64 vet and test compile) exit 0. `git diff --name-only origin/main HEAD` lists the 12 files of `git diff --name-only origin/main...origin/lane-C-he-job-order-person-list`, plus lib/ha_question_parity_test.go and internal/cli/inprocess.go (this report is added in the next commit); `git diff --name-only origin/main...origin/lane-M-ip-plan-diff-fold-task-lane` is empty. |

Rules:

| Rule | Result |
|---|---|
| Edited only the named files | HOLDS, see row 6. |
| No test deleted, skipped or weakened | HOLDS. The one row changed (lib/ha_question_parity_test.go:229) keeps the call and now pins its removal, as step 2 of the brief says. |
| No guard, gate, cap or hold on agents | HOLDS. No such code added. |
| Commits: one sentence, no attribution lines | HOLDS. |
| Windows and macOS builds | HOLDS, `make cross` exit 0. |
| `*RealPlugin*` tests | Not named by any row; none run. |

## NOT VERIFIED on a live screen

Nothing here has a UI. The whole-module test and the cross-repo set are left to the orchestrator at landing.
