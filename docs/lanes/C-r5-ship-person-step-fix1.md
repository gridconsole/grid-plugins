# C-r5-ship-person-step-fix1: the tests that pinned "deliver posts no person Job" now expect it

Base: lane-C-r5-ship-person-step (head ed4efe64), on main 921ca9fb. Repo: plugin. The bundle carries the base lane's commits (f9633e40, ed4efe64) and this lane's test commit (aa594bbf). The base lane's report is docs/lanes/C-r5-ship-person-step.md. Its NOT DONE verdict (:3) and its rows :16-20 (6 FAIL, importers, lint, builds, merge and walk tests not run) are closed by rows 1-4 and the builds row below.

Direction for the test edits: orchestrator ruling, 14 Nov 22:55 UTC, option (a) of docs/lanes/C-r5-ship-person-step.md:31-34, written in this lane's brief. The ruling changes deliver's behaviour (end review 2212Z M1, default of board question q-ship-person-step), so tests that pinned "deliver posts only the curator Job", or an exact Feed or stderr without the person Job, now expect the person Job. Only that expectation changed. The other assertions in each test are as they were.

## Proof table

| Row | Status | Evidence |
| --- | --- | --- |
| 1. TestErDeliverFromAJobWithNoProposalPostsNothing | HOLDS, renamed | Renamed to **TestErDeliverFromAJobWithNoProposalPostsOnlyThePersonJob** (er_deliver_from_job_test.go:147); the old name is in its comment. Old: `after != before` -> `the Feed holds 2 Jobs, was 1` (FAIL). New: `after == before+1`, `curator_job` still empty, and the new Job is the open approve person Job, the same id as `person_job` in `--json`. `--- PASS`. Only docs/lanes files named the old test (`grep -rn PostsNothing`). |
| 1. TestJwKhNothingPlantedTakesTheFirstID | HOLDS | jw_keeper_held_recheck_test.go:238 (it failed at :237 before a comment line was added). Old want `["J-1 curator counted"]` -> FAIL `job records = ["J-1 curator counted" "J-2 Person counted"]`. New want adds `"J-2 Person counted"`. `--- PASS`. |
| 1. TestJwKhHeldIDPlantedBeforeTheLockIsSkipped | HOLDS | jw_keeper_held_recheck_test.go:192 and :213. Old wants `[J-1 builder held, J-2 curator counted]` and `[J-1 builder counted, J-2 curator counted]` -> FAIL `... "J-3 Person counted"], want [...]` at :192 and :213. New wants add `"J-3 Person counted"`. `--- PASS`. |
| 1. TestDeliverKeeperJobPostsOnceWhileOpen | HOLDS | deliver_keeper_job_test.go:143. Old: the curator Job posted after J-1 is cancelled is `J-2` -> FAIL `a cancelled curator Job must not block the next: "J-3" <nil>`. J-2 is the first delivery's person Job, so the new expectation is `J-3`. `--- PASS`. |
| 1. TestDeliverStillDeliversWhenTheCuratorJobCannotBePosted | HOLDS | deliver_keeper_job_test.go:172 (it failed at :171). Old: one stderr line, the curator line -> FAIL in both subtests, e.g. `stderr: "deliver: the curator Job was not posted: stale lease\ndeliver: the person Job was not posted: stale lease\n"`. New: two lines, curator then person. Exit 0, state `waiting`, stdout and "no Job posted" are unchanged. The name is still true. `--- PASS` (no-epoch, taken-by-someone-else). |
| 2. TestRemainingTaskOperands: cause | HOLDS: the person Job, not a base bug | FAIL in the deliver id, folder, folder-name and relative subtests: `expected JSON success: {...} deliver: the person Job was not posted: GRID_MEMBER is required`. The fixture env has no GRID_MEMBER, so `orchestratorWriter` refuses (internal/cli/orchestrator.go:668). deliver prints the line and still delivers (deliver.go:105-114). The base lane meant this, the same as the curator Job (docs/lanes/C-r5-ship-person-step.md:39, :48), and option (b), a silent deliver, was not chosen. So I fixed the test, not the code. |
| 2. TestRemainingTaskOperands: update | HOLDS | remaining_task_operand_test.go:122-133. For `deliver`, stderr must be exactly `deliver: the person Job was not posted: GRID_MEMBER is required\n`. Every other command still needs empty stderr, and every command still needs valid JSON. `--- PASS`. |
| 3. TestApproveAfterDeliveryRecordsPlanApproval and approve_no_gate_test.go unchanged and passing | HOLDS | `git diff --stat origin/main..HEAD -- internal/cli/approve_no_gate_test.go internal/cli/delivery_test.go` is empty, exit 0. `go test -count=1 -v ./internal/cli -run '...TestApproveAfterDeliveryRecordsPlanApproval\|TestApproveGateFlagIsUnknown'`: `--- PASS` for both, exit 0. |
| 4. `go test` of internal/cli | HOLDS | `go test -count=1 ./internal/cli`: `ok ... internal/cli 30.601s`, exit 0. This includes the base lane's new tests: TestDeliverPostsShipPersonJob, TestRedeliverPostsNoSecondShipPersonJob, TestApproveAnswersShipPersonJob, TestBuiltInFlowShowsShipPersonStep, and the edited built-in row of TestNhWfShowTaskSources. |
| 4. Importers of internal/cli | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep 'grid-plugins/internal/cli\b' \| cut -d' ' -f1` gives cmd/grid, contract/formats, internal/account, internal/braincmd, internal/cli, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test -count=1` over all of them except internal/cli (run above): each `ok` (cmd/grid has no test files), exit 0. The hand-edited fixture contract/fixtures/cmd-workflow-show/valid/task-built-in.json is named only by internal/cli tests (`grep -rln cmd-workflow-show --include=*_test.go`), and those pass. The packages that walk the fixtures also pass: `go test -count=1 ./contract/... ./internal/contract/...` gives `ok` for contract, contract/formats and internal/contract, exit 0. |
| 4. `make lint` | HOLDS | `make lint` (`go vet ./...`), exit 0. |
| 4. Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `--- PASS` for both, ok, exit 0. `go test -count=1 ./internal/job/lint/`: ok, exit 0. |
| 4. `git fetch origin && git merge origin/main` before the last run | HOLDS | `Already up to date.` origin/main is 921ca9fb and an ancestor of HEAD (`git merge-base --is-ancestor`, exit 0). The last runs above are on head aa594bbf. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0. |
| New tests fail on base | HOLDS | This lane adds no new test. The base lane's FAIL lines are in docs/lanes/C-r5-ship-person-step.md:15. Its missing one, TestBuiltInFlowShowsShipPersonStep, I have now run on base: `git checkout 921ca9fb -- internal/cli/nh_workflow_show_task.go` gives `deliver_person_job_test.go:129: built-in steps [{Name:Run ...} {Name:Check ...}], want Run, Check, then the person step` / `--- FAIL`. After `git checkout HEAD -- internal/cli/nh_workflow_show_task.go`, `git status --short` is empty and the test is `ok`. |
| Existing tests edited only by direction; none deleted | HOLDS | The six tests above, by the 22:55 ruling. One rename, old name noted. No test was deleted. |
| Small commits, one-sentence messages, no attribution | HOLDS | aa594bbf (tests) and this report's commit. No attribution lines. |
| No guard, gate, cap, hold or blocking hook | HOLDS | This lane changes tests only. The base lane's Job is a request (docs/lanes/C-r5-ship-person-step.md:21). |
| Base lane rows 1-3b (person Job at delivery, built-in flow, approve answers it) | HOLDS (unit) | The rows and evidence are in docs/lanes/C-r5-ship-person-step.md:9-14. The tests they name pass in the internal/cli run above. |
| App walk tests (webui-test, TestFvArgvCollects, route walks, slices Imports) and realplugin guest tables | not run, not asked | Plugin lane. Those are the app repo's. |
| Whole-module `go test ./...` and cross-repo set | not run, not asked | The orchestrator runs them at landing. |

## What changed in this lane

- internal/cli/er_deliver_from_job_test.go: Row 4 test renamed, and it expects one person Job.
- internal/cli/jw_keeper_held_recheck_test.go: the job-record lists include the person Job.
- internal/cli/deliver_keeper_job_test.go: the next curator id is J-3, and stderr has two lines.
- internal/cli/remaining_task_operand_test.go: deliver's stderr is the one person-Job line.
- docs/lanes/C-r5-ship-person-step-fix1.md (this file).

No production code changed in this lane.

## Open items (none blocks this lane)

- TestDeliverWithNoProposalsPostsNoJobAndPrintsAsBefore passes, but its name is now stale: a delivery posts the person Job, and the test only counts curator Jobs. It is not in this brief, so I did not edit it.
- A deliver with no GRID_MEMBER or no task lease epoch (a person's terminal) posts no person Job and prints one stderr line on every delivery. The base review asked whether round 5's deliver was in that state. Still open.
- `person_job` in `deliver --json` has no contract or schema ruling yet (base review note).
- The base lane's other open items still stand (docs/lanes/C-r5-ship-person-step.md:44-52): LA1 `grid land` after the answer, the app's Judge flow, the dry run not listing the person Job, and the stale docs/delivery.md:149-157.

## NOT VERIFIED

- Nothing on a live screen. The lane has no UI.

DONE
