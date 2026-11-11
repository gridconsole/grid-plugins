# C-gq-plan-jobs-replan-text

| Row | Result |
|---|---|
| 1 TestSkillAndOrchestratorNameOnlyContractCommands | HOLDS: `go test ./internal/job/lint/ ./internal/plan/ -count=1` exit 0 |
| 2 TestSkillExampleLintsClean, TestOrchestratorNamesTheOrphanKinds | HOLDS: same run, exit 0 |
| 3 internal/plan/anchor_skill_forms_test.go:31 | HOLDS: same run, exit 0 |
| 4 `go test ./internal/job/lint/ ./internal/plan/ -count=1` | HOLDS: exit 0 |
| Packages importing the changed ones | The change is a markdown file only; the tests that read it (lint, plan, contract, packaging) ran: `go test ./internal/job/lint/ ./internal/plan/ ./internal/contract/ ./internal/packaging/ -count=1` exit 0 |
| Lint | HOLDS: `go vet ./internal/job/lint/ ./internal/plan/` exit 0 |
| Windows and macOS builds | HOLDS: `GOOS=windows go build ./...` exit 0, `GOOS=darwin go build ./...` exit 0 |
| New tests fail on base | No new test: the brief names the existing tests, which prove the commands the section names. |
| No guard, gate or cap added | HOLDS: text only. |
| Commands named match the contract and the CLI | `grid job cancel` needs `--reason` and `--job` (internal/cli/orchestrator.go:392-397); `--person <name>` at :81. The text uses those forms. |

## What changed

skills/plan-jobs/SKILL.md: new section "## Re-plan Job" (6 steps) before "## Run": read the cancelled Job and its exit_code, keep (replacement posted, waiter's `after` repointed) or cancel each waiter, person Job for a human-only step, read the tree after.

## NOT VERIFIED on a live screen

Nothing needs a screen. Not run: whole-module test and cross-repo set (orchestrator at landing).
