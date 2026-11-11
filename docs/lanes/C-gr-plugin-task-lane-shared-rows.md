# C-gr-plugin-task-lane-shared-rows: `task lane` in the three shared tables

Result: all seven rows HOLD. `go test -count=1 ./internal/task/ ./internal/cli/ ./internal/contract/` is green on the merged head.

## Proof table

| # | Row | Result | Proof |
|---|---|---|---|
| 1 | The three tests fail on the base with the quoted messages | HOLDS | Run on the branch before any edit (output below). Exit 1, three `--- FAIL` lines. |
| 2 | One new `task lane` row per table, no other row changed | HOLDS, with one extra line (see "Fourth edit") | Diff below. The merge touched none of the three files: `git diff origin/lane-C-gr-plugin-task-lane HEAD` on them shows only my lines. |
| 3 | Guest row asserts the refusal; unknown-task walk asserts `no_such_task` | HOLDS | fv_guest_task_test.go:174 (`exit 1`, `grid task lane: role "guest" may not move a task to a lane; a guest answers and comments`, `changed false`); passing `TestFvGuestTaskVerbs/lane`. The walk row is `"task lane"` in `ggTaskWalked` (gg_unknown_task_walk_test.go:24); `TestGgUnknownTaskWalk/task_lane` and `TestGgUnknownTaskWalkCoversTheContract` pass. `no_such_task` itself is asserted by `TestGrTaskLaneNoSuchTask` (gr_task_lane_test.go) and by the walk's generic unknown-task check, which ran for `task lane`. |
| 4 | commands.json is what `make generate` writes | HOLDS | `make generate` then `git diff --exit-code contract/commands.json`: exit 0, no output; `git status --short` clean after generate. |
| 5 | `go test -count=1 ./internal/task/ ./internal/cli/ ./internal/contract/` green | HOLDS | `ok .../internal/task 12.460s`, `ok .../internal/cli 69.106s`, `ok .../internal/contract 1.610s`. No `--- FAIL`. |
| 6 | origin/main and origin/lane-C-gr-plugin-task-lane are ancestors of HEAD | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0 (main = 91fadeff87525f5859978740e0b66ce51f375e48); same for `origin/lane-C-gr-plugin-task-lane`, exit 0. |
| 7 | No product file changed, no test deleted, skipped or weakened | HOLDS | See "Row 7 diff". |
| | `go build ./...` | HOLDS | exit 0 |
| | `make lint` | HOLDS | printed `go vet ./...`, exit 0 |
| | Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| | macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| | Importers of changed packages | PARTLY | Only test files in `internal/cli` changed, no product code, so no importer sees different code. I ran the three named packages. `go list` of direct importers of task, cli and contract: cmd/grid, artifact, brain, braincmd, cli, contract, feed, guard, hook, intent, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, question, session, settings, signal, task, usage, where, workflow, lib. The whole-module test is the orchestrator's. |
| | No guard on agents | HOLDS | No product file touched. |
| | Attribution lines | HOLDS | None in my commit messages. |

## Row 1: failing output (before any edit)

```
--- FAIL: TestFvGuestTaskVerbs (0.04s)
    fv_guest_task_test.go:280: grid task lane is in contract/commands.json but in neither fvGuestVerbs nor fvGuestReadVerbs
--- FAIL: TestGgUnknownTaskWalkCoversTheContract (0.00s)
    gg_unknown_task_walk_test.go:127: contract members not walked or skipped: [task lane]; walked or skipped but not in the contract: []
--- FAIL: TestGuestProjectParity (8.35s)
    guest_parity_test.go:350: missing guest parity command: task lane
FAIL
```

## Row 2: the diff (three files, against the task lane head)

```
fv_guest_task_test.go   + {"lane", []string{"task","lane","T-01","doing","--project","example"}, 1, `grid task lane: role "guest" may not move a task to a lane; a guest answers and comments`, false},
gg_unknown_task_walk_test.go   "task move", ...  ->  "task lane", "task move", ...   (one name added to ggTaskWalked)
guest_parity_test.go   + "task lane": {"{task}", "doing"},                       (guestParityCommands, between task history and task list)
guest_parity_test.go   + case "task lane": want = guestParityOutput{1, "", "grid task lane: role \"guest\" may not move a task to a lane; a guest answers and comments\n"}
```

Merge hunks: none in these three files. The merge of origin/main brought other lanes' changes (listed in row 7).

### Fourth edit

The parity table needs a second line beyond the one row. With only the row added, `TestGuestProjectParity/task_lane/*/invited` failed with "invited guest still gets unknown answer": `guestParityEarlyAnswer` lists the verbs whose guest refusal comes before the project is resolved, and `task lane` was not in it. I added a `case "task lane"` next to `task new|close|reopen|stop|move`, with the refusal the product gives. The generic case could not be reused because the lane message reads `move a task to a lane`, not `lane a task`. This is the same kind of line as the neighbours' and weakens nothing.

## Walk: walked, not skipped

`task lane` is in `ggTaskWalked`. The walk drove it with its default operand and the unknown task answered as the other task verbs do (`TestGgUnknownTaskWalk/task_lane` PASS). No skip entry was needed.

## Row 7 diff

`git diff origin/main HEAD --stat` is exactly the task lane work plus my three test files, 13 files:
contract/commands.json (the lane's entry, identical to what `make generate` writes), docs/lanes/C-gr-plugin-task-lane.md, internal/cli/cli.go, task.go, task_lane.go, internal/task/lane.go, contract_inventory_test.go (one annotation), gr_task_lane_test.go (new), the two golden files, and my three test files (+1, +1 changed, +3).
No product file beyond the task lane lane's own was changed by this lane. My two commits touch only the three test files.

`git diff origin/lane-C-gr-plugin-task-lane HEAD --stat` is larger, because it includes origin/main's changes brought in by the merge. It shows two deletions, in internal/cli/gg_unknown_project_walk_test.go (-13) and internal/land/land_test.go (-4). Both come from origin/main, not from this lane: `git diff origin/lane-C-gr-plugin-task-lane origin/main --stat -- <those two files>` shows the same 17 deletions. No test was deleted, skipped or edited by this lane.

## Commits

1. `Add the task lane row to the unknown-task walk, guest verb and guest parity tables.` This commit alone left `TestGuestProjectParity` red (the fourth edit was missing); a chained command committed before I had seen the run.
2. Merge of origin/main (no conflicts, none in list tables).
3. `Name the guest refusal of task lane in the parity early-answer table.` Fixes the red above.

The merged head is green. Only the first commit alone is not.

## What is NOT VERIFIED on a live screen

Nothing here changes a screen. The task page's state menu and the app's reading of `card.lane` remain unchecked, as in the task lane report.
