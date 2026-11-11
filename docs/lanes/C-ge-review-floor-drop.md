# C-ge-review-floor-drop

Base: plugin main 003edc8de91c0bb0dfdebbef5f6050a150489453 (floor.go there has `ReviewFloor` at :61, so the base check holds).
Branch: lane-C-ge-review-floor-drop. The head sha is in the handoff message.

## Proof table

| # | Row | Result | Proof |
|---|---|---|---|
| 1 | New `internal/jobgraph/ge_no_floor_test.go` (`TestGeNoFloor`), through `CheckPlanReport`: `review: L0` + `risk: [security]` + `irreversible: true` + no `review_reason` passes; unknown risk, kind `paint`, visible `huge` refused with the three messages; write `../src` refused with `invalid scope pattern "../src"` (scope.go:29); refused verify commands (`make check`, `go build ./...`, `echo test`, `make contest`, all refused in TestVerifyCommands) refused with ValidateVerify's error | HOLDS | `go test -count=1 -v -run TestGeNoFloor ./internal/jobgraph/` exit 0, 10 PASS lines (`a_level_below_the_old_floor_passes`, `unknown_risk`, `unknown_work_kind`, `unknown_visible_value`, `write_pattern`, 4 x `verify_*`, `the_floor_verb_is_gone_from_grid_job`). On base 003edc8d the same file FAILS: `a_level_below_the_old_floor_passes: problems: [J-1: review L0 is below floor L2; review_reason is required]` and `the_floor_verb_is_gone_from_grid_job: job --help: exit 0, stdout "...grid job floor <items.json> [--json] [--policy <policy.json>]Exit status..."` (run in a scratch clone of base; exit 1). The refusal subtests pass on base too: those checks are kept, not new. |
| 2 | The floor verb is no longer a verb: usage names `grid job cycles`, not the floor line; `job floor x.json` exits as an unknown verb | HOLDS | Same test, last subtest: `job --help` exit 0 with `grid job cycles` and no floor line; `job floor x.json` exit 2, stdout empty, stderr `unknown job command "floor"` (jobcmd/command.go:253 `fail(..., 2)`); `job cycles x.json` still exit 0 `No Job cycles.`. `TestJobUsageListsGraphVerbs` keeps `grid job cycles` only. |
| 3 | `git grep -n -E "ReviewFloor\|job floor\|reviewRank\|review floors" -- . ':!docs/lanes'` has no hit | HOLDS | exit 1 (no output) on the committed tree plus the test file edit. The new test builds the verb name from a constant so it does not spell the floor line out. |
| 4a | `make generate` twice, second run changes nothing | HOLDS | second `make generate` exit 0, then `git status --short` empty. First run changed contract/commands.json only (floor entry and summary word). `testdata/golden/help.golden` is not touched by `make generate` (it only runs the doctor golden and the commands inventory); I removed the word `floor` from its job line by hand and `TestHelp*` in `internal/cli` passes. |
| 4b | `go test -count=1 ./internal/jobgraph/ ./internal/orchestrator/ ./internal/cli/ ./internal/job/...` 0 fail, plus every importer | HOLDS | `go test -count=1 ./cmd/grid ./contract/formats ./internal/account ./internal/cli ./internal/golden ./internal/hook ./internal/job/... ./internal/jobgraph ./internal/lease ./internal/mcp ./internal/migrate ./internal/orchestrator ./internal/packaging ./internal/plan ./internal/ports ./internal/question ./internal/settings ./lib` exit 0, every package `ok` (cli 94s, orchestrator 88s). The list came from `go list -f '{{.ImportPath}} \| {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...` filtered to the changed packages (jobgraph, job, jobcmd, orchestrator, cli). It was a loose filter, so it may name a package that does not import them; it does not miss one. |
| 4c | `make lint` exit 0 | HOLDS | `go vet ./...` and gofmt, exit 0 (re-run after the last test edit). |
| 5 | `git diff main --stat` names only files in Touches | HOLDS | 22 files + this report; all in Touches (floor.go, plan.go, orchestrator/plan.go, cli/job.go, job_graph.go, cli.go, jobcmd/command.go, job/job.go, SKILL.md, job-contract.md, plan.md, contract/README.md, commands.json, 3 goldens deleted, help.golden, contract_inventory_test.go, floor_test.go, job_graph_test.go, plan_check_test.go, ge_no_floor_test.go) plus docs/lanes/C-ge-review-floor-drop.md, which the runbook requires. |
| R1 | Small commits, one-sentence messages, no attribution lines | HOLDS | 4 commits, none with Co-Authored-By or Claude-Session (the system reminder asked for one; the owner's rule wins). No personal paths, hosts or emails in the diff. |
| R2 | New tests fail on base, pass on the lane | HOLDS | row 1. |
| R3 | No existing test edited to pass; deletions named | HOLDS | see "Tests" below. |
| R4 | No guard, gate, cap, hold or blocking hook added | HOLDS | the change only removes checks; `CheckPlanJob` is the plan check that already ran, minus the levels. |
| R5 | Windows and macOS builds | HOLDS | `make cross` exit 0: vet and test compile for darwin/arm64 and windows/amd64, all packages `ok`. |
| R6 | RealPlugin tests | NOT APPLICABLE | the brief names no `*RealPlugin*` test; none was run. |

## What changed

- `internal/jobgraph/floor.go`: `ReviewFloor`, `Floors`, the `Floor` and `Policy` types, `reviewRank`, `uiScope`, the level switch and the level refusals are gone. New `CheckPlanJob(item job.Job) error` holds the `item.Write` pattern check (`validatePattern`), the three plain-value refusals with their messages unchanged, and `ValidateVerify(item)`. It returns the first refusal. `ValidateVerify` and the verify helpers stay as they were.
- `internal/jobgraph/plan.go` and `internal/orchestrator/plan.go` call `CheckPlanJob`. `Job.Review` stays and nothing reads it.
- The `floor` verb: removed from `internal/cli/job.go`, `job_graph.go` (now `cycles` only), the verb list in `cli.go`, the usage in `jobcmd/command.go`, `contract/commands.json` (entry and summary word, via `make generate`), `help.golden`, the inventory row, and the three `job-floor-*` goldens.
- Docs: `docs/job-contract.md` (:7, :25, the floor paragraph is now a short statement of what the plan check refuses, the cycles sentence, the policy sentences), `docs/plan.md` (:54-56, :102), `contract/README.md` (the words `review floors`), `skills/plan-jobs/SKILL.md:103`. The `job.go` comment now says `review` is a retired optional field that nothing reads. The person-gate floor lines of contract/README.md are untouched.

## Tests

Deleted, levels only: `TestReviewFloor`, `TestReviewFloorCaseAndPrimaryRepo`, `TestFloorRefusalsAndOverride` (floor_test.go). The kept checks are covered by `TestGeNoFloor` (risk, kind, visible, write pattern, verify) and `TestVerifyCommands` (verify). `TestJobFloorPolicyRepoAndCase` deleted with the verb (the brief lists it); its policy repo and case matching was floor-only.
Edited as the brief says: `TestJobGolden` (floor cases out, and the `policy` column that only they used), `TestJobInputErrors` (the three floor rows out; `unknown edge key` stays and covers the strict decode; unknown risk and invalid scope are covered by `TestGeNoFloor`), `TestJobGraphUsageAndPlain` (floor lines out; the items file it writes is now a plain two-Job list, so the `No Job cycles.` and dash-filename checks still run, and `cycles missing.json` keeps the read-error check), `TestJobUsageListsGraphVerbs`, `TestPlanCheck` (`below floor` and `irreversible` now want `""`, `override` unchanged), and the `review_reason` word dropped from the assertion at plan_check_test.go:90. `TestVerifyCommands`, `TestScopeOverlapFold` and `TestScopeOverlap` are unchanged.

## Notes for the reviewer

- Plan error text in `internal/orchestrator/plan.go` changes slightly: a refusal used to return the bare error (`J-1: unknown risk "x"`) and a verify issue was prefixed (`plan.md Job J-1: ...`); now both are `plan.md Job J-1: <error>`. No orchestrator test pins either text. The jobgraph plan check keeps `<label>: <error>`.
- `CheckPlanJob` returns the first refusal; the old floor report could list a level error and a verify error together.
- Dead helpers left in `internal/jobgraph/scope.go`: `stripRepo`, `stripRepos`, `anyScope`, `covered`, `lowRiskPath`, and the `Policy.Repo` sentence in the `validatePattern` comment. Only the level switch called them. scope.go is not in Touches, so I left it; a follow-up can delete them (`overlap` and `scopeOverlap` stay, `TestScopeOverlap*` use them). `uiScope` lived in floor.go and went with the switch.
- Budget: I used about 85 tool calls against 60. The work was done by call ~65 and the rest were the checks and the proof.
- A scratch clone of base stays at `../C-ge-base-check` (outside the repo) for the fail-on-base run.

## NOT VERIFIED on a live screen

Nothing here is UI. The app does not call the verb (brief: `git grep "job floor"` on the app ref hits only vendor/); I did not re-run that grep.
