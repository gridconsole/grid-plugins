# C-eo-recall-guard-drop: the plan guard stops reading shell text for a brain recall line

Base: bb3c1340 (the brief's d528178a lines all matched what I read: plan.go:20-75, recall.go whole; `recallGate` was present). Lane commit: see the bundle head.

## Proof table

| Row | Result |
|---|---|
| 1. Every deleted function and case listed with file:line on base | HOLDS, list below |
| 2. Bash `node plan-hash.mjs <task>/plan.md && echo "plan ready" >> <task>/history.md` on a held plan with no recall line returns an empty decision at block | HOLDS: `TestPlanGuardBashNoRecallRead`, internal/guard/dy_recall_guard_test.go:23. On base: `go test -run 'TestPlanGuardBashNoRecallRead\|TestPlanGuardEditPlanReadyNoRecallRead\|TestPlanGuardOtherRulesStillDeny' ./internal/guard/` exit 1, `dy_recall_guard_test.go:37: decision {... Behavior:deny Reason:plan-hash.mjs is not run on .../plan.md: plan.md has no brain recall line. ...}, want empty`. On the lane: exit 0 |
| 3. Edit of history.md adding the plan ready line returns no deny with no recall line | HOLDS: `TestPlanGuardEditPlanReadyNoRecallRead`, dy_recall_guard_test.go:42 (also a Write of the same line). On base: exit 1, `dy_recall_guard_test.go:52: decision {... Behavior:deny Reason:.../history.md is not changed to add a plan ready line: plan.md has no brain recall line. ...}, want empty`. On the lane: exit 0 |
| 4. Folder-name check and one role fence still deny at block | HOLDS: `TestPlanGuardOtherRulesStillDeny`, dy_recall_guard_test.go:65 (folder name `tasks.` as in "tasks folder odd sibling"; reviewer writing in the repo as in "reviewer remains inside projects"); passes on base and lane |
| 5. `git grep -n -E "recallGate\|recallHeld\|recallMissing\|writesTo\|planHash\|shellReady" -- internal/guard` | HOLDS: empty, exit 1 (no match) |
| 6a. `git diff main --stat` | HOLDS with one addition: the files in Touches plus the new test file, plus `golden/known-differences.txt` (+1 line, see below). `main` here is the lane's base |
| 6b. `go test -count=1 ./internal/guard/... ./internal/task/... ./internal/hook/... ./internal/golden/...` | HOLDS after the golden line: guard ok, task ok, hook ok, golden ok, exit 0 |
| 6c. `make lint` | HOLDS exit 0. The target is `go vet ./...` and `gofmt -l .`; it does not run an unused-name linter. Unused names were checked by hand: `historyAlias`, `addsMark`, `planReady`, and the `plan` import are gone, and the package compiles and vets |
| 6d. `GOOS=windows go vet ./...` | HOLDS exit 0 |
| macOS build | HOLDS: `GOOS=darwin go build ./...` exit 0 |
| Importers tested | HOLDS: `go test -count=1 ./internal/guard/... ./internal/task/... ./internal/hook/... ./internal/golden/... ./internal/braincmd/... ./internal/job/... ./internal/land/... ./internal/signal/... ./internal/packaging/... ./internal/cli/...`. All ok. The first full run failed `TestApproveRecordWorkerPassMovesTheReceiptOfATaskOnlyRead` under load; it passed alone and in a second `go test -count=1 ./internal/cli/...` (exit 0). That test does not touch the guard |
| No guard, gate, cap or hold added | HOLDS: only deletions in internal/guard and internal/task; the Bash path of `evaluatePlan` now falls through to the non-file-tool return (plan.go:31) |
| New tests in one new file | HOLDS: internal/guard/dy_recall_guard_test.go |
| No existing test edited to pass | HOLDS: only deletions of the cases named below |
| No attribution lines, no personal paths | HOLDS |

## Deleted on base bb3c1340 (file:line)

Functions and vars:
- internal/guard/recall.go whole file (115 lines): `recallGate` :41, `recallHeld` :33, `capture` :25, `writesTo` :94, const `recallMissing` :23, regexps :12-21 (`planHash`, `shellReady`, `printFlag`, `redirectToFile`, `otherWrite`, `teeWord`, `teeArgs`, `indirectWrite`, `singleQuoted`, `literalPath`).
- internal/guard/plan.go: Bash branch :31-33; plan ready deny :56-62; `historyAlias` :16; `addsMark` :119-143; the `plan` import.
- internal/guard/context.go: `planReady` :144.

Test cases:
- `TestRecallBash`, internal/guard/plan_edges_test.go:10-50 (15 commands, each with a missing and a present recall line).
- Row "plan ready needs recall", internal/guard/guard_test.go:189-196.
- No other case in internal/guard expects a recall deny (`git grep -n -i recall -- 'internal/guard/*_test.go'` on base: only those two plus `TestRecallLineForms`, which tests `plan.HasRecall` and stays because `grid plan` still asks for the line).

What still covers plan-guard: `TestPlanHeaderRules` (guard_test.go:95; folder alias, odd sibling, planner, curator, brain, reviewer and checker rows), `TestPlanGuardRemovedApprovalLineDenies` (planguard_warn_test.go:33), and the three new tests. Shell reading for people scope is in internal/guard/home.go, untouched.

## What changed
- `evaluatePlan` returns an empty decision for a Bash call, as for any non-file tool; a history.md write that adds the plan ready line is no longer denied for a missing recall line.
- Comments at internal/guard/context.go:159 and internal/task/hold.go:137 and docs/settings.md (plan-guard row, and the "other plan rule" sentence) drop the brain-recall words.
- `golden/known-differences.txt` +1 line: the recorded case `plan-guard/bash-recall` expected the legacy recall deny, and the golden corpus compares against that recording, so row 6 could not pass without it. This file is not in Touches; it is the precedent used in commit 85a1d976. The reason cites `docs/lanes/L160.md:33` because the checker accepts only `L<n>.md` cites. Orchestrator: keep it, or replace with deleting the recorded case.

## NOT VERIFIED on a live screen
Nothing here is UI. A live Claude Code session running a Bash `plan-hash.mjs` was not run (no claude in the sandbox).

## Open items
- `plan.HasRecall` and internal/plan/write.go:85 stay (owner question in INDEX.md).
- Merge order per INDEX.md for docs/settings.md; my lines (:115, :139-140) do not overlap the other lanes' lines.
