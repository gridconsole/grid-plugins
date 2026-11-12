# C-ve4-merge-verb-touches: C-ve-task-merge-verb with the verb inventories and help line in Touches

Base: origin/main (44c7de5b) plus the lane commits of C-ve-task-merge-verb (head before this report: 1c960ab9). No code changed in this lane: the brief only widens Touches to the six files the review named, so the commits stay as they are and this report is the only addition.

## Proof table

| # | Row / rule | Result | Command / place |
|---|---|---|---|
| 1 | `git merge origin/main`, name each file resolved | HOLDS | `git merge origin/main`: "Already up to date.", exit 0. No file resolved; origin/main has not moved since the lane's base. |
| 2 | Fix the review's reds, never weaken a row | HOLDS | The review's only reds were the Touches list. Nothing is red on the head: see rows 5 to 9. No test, row or assertion was changed. |
| 3 | Each inventory change only adds `merge` and removes nothing | HOLDS | `git diff origin/main...HEAD` on the six files: contract_inventory_test.go:471 `+"task merge"` annotation; fv_guest_task_test.go:176 `+` guest merge row; gg_unknown_task_walk_test.go:24 one line, `"task merge"` inserted between `task lane` and `task move`, every other entry kept; guest_parity_test.go:131 `+"task merge"`; he_mv_message_verbs_test.go:46 `+` heMvProse row for `internal/cli/cli.go:task help`. No line deleted except the re-wrapped ggTaskWalked line. |
| 4 | Help line (cli.go:135, help.golden, task-help.golden) | HOLDS, with a note | cli.go:135 now reads `new\|show\|list\|status\|lane\|merge, stop\|close\|reopen, history\|attach\|move (grid task help lists all)`; help.golden:11 is the same line; task-help.golden gains the `grid task merge` usage line. The top-level help no longer names decline, delete, rename, restore, summary; they stay in `grid task help` (ex_help_summary_verbs_test.go accepts the suffix form when the verbs do not fit on one line). |
| 5 | Brief row 1: ve_task_merge_test.go | HOLDS | `go test -count=1 -v -run TestVeMerge ./internal/cli/` exit 0: PASS TestVeMergeCleanMergeFromTheTaskBranch, TestVeMergeCleanMergeOnMainWithProject, TestVeMergeConflictAbortsAndLeavesTheCheckout, TestVeMergeDirtyCheckoutRefusedBeforeAnyMerge, TestVeMergeTaskWithoutBranch, TestVeMergeUsage; `ok internal/cli 1.269s`. |
| 6 | Brief row 2: `go test -count=1 -run TestHsIpWalksEveryCommand ./internal/cli/` | HOLDS | exit 0, `ok internal/cli 0.009s`. |
| 7 | Brief row 3: `go test -count=1 ./internal/cli/ ./internal/contract/ ./internal/deliver/...` | HOLDS | `./internal/deliver/...` does not exist in the plugin (deliver is internal/task/deliver.go), dropped as the brief allows. `go test -count=1 ./internal/cli/ ./internal/contract/ ...`: `ok internal/cli 63.986s`, `ok internal/contract 2.305s`, exit 0. |
| 8 | Tests of changed packages and every importer | HOLDS | Changed: internal/cli, internal/contract (test file), contract/commands.json. Importers of internal/cli (grep of the import path): internal/packaging, account, settings, mcp, braincmd, golden, jobgraph, question, lib, cmd/grid, contract/formats. One run, exit 0: contract ok, contract/formats ok, packaging ok, account ok, settings ok, mcp ok, braincmd ok, golden ok, jobgraph ok, question ok, lib ok, cmd/grid no test files. |
| 9 | Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `ok internal/cli 0.127s`. `go test -count=1 ./internal/job/lint/`: `ok internal/job/lint 0.153s`. Both exit 0. |
| 10 | `make lint` | HOLDS | exit 0; last line `go vet ./...` and nothing after it. |
| 11 | `make cross` | HOLDS | exit 0; last lines `cross: windows/amd64 test compile` then `ok github.com/gridconsole/grid-plugins/lib 0.001s`; darwin/arm64 stage the same, no FAIL. |
| 12 | New tests fail on base | HOLDS | With `internal/cli/task.go`, `internal/cli/inprocess.go` and `contract/commands.json` taken from origin/main: `internal/cli/ve_task_merge_test.go:70:9: undefined: taskMergeGit` (also :71:2, :75:21), `FAIL github.com/gridconsole/grid-plugins/internal/cli [build failed]`. Restored with `git checkout HEAD -- <the three files>`; `git status --short` empty. |
| 13 | No existing test edited to pass or deleted | HOLDS | Row 3: additions only. |
| 14 | No guard, gate, cap, hold or hook on agents | HOLDS | Nothing added in this lane; the verb applies no role rule. |
| 15 | *RealPlugin* tests | not applicable | None named by the brief. |
| 16 | Commits | HOLDS | One-sentence messages, no attribution lines, no personal paths. |

## What changed

Nothing in the code. The six files the review listed as outside Touches are now inside this lane's Touches; their diffs are as reviewed (rows 3 and 4). This report is the only new file.

## Open items (from the review, not part of this brief)

- deliver writes `base:` as a merge-base sha (internal/task/deliver.go:279), so the verb almost always merges into the main/master fallback.
- contract/README.md has no `grid task merge` section (outside Touches).
- A guest is not refused by the verb; the app route decides who may merge.

## NOT VERIFIED on a live screen

The Code tab Merge button of the app was not clicked. The verb's argv and JSON shape match the route's read, `{"ok":true,"merged":"<sha>"}`, which TestVeMergeCleanMergeOnMainWithProject compares on exact stdout.
