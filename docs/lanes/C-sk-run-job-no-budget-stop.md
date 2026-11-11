# C-sk-run-job-no-budget-stop

Base: 0e60a23a (clone start; the brief named d0400870).

| Row | State | Proof |
|---|---|---|
| 1. New test fails on base | HOLDS | `go test ./internal/contract -run TestSkillRunJobBudget -count=1` on base, exit 1: `skill_run_job_budget_test.go:12: run-job SKILL.md still tells the agent to stop when the Job stops fitting its turns estimate` and `:15: run-job SKILL.md must say budget.turns_estimate is not a limit` |
| 2. Existing contract tests pass unchanged | HOLDS | `go test ./internal/contract -run 'TestRunJobSkillHoldsTheRules\|TestSkillExamplesValidate\|TestSkillAndAgentCommandsExist\|TestSkillRunJobBudget' -count=1` exit 0; skill_examples_test.go not edited |
| 3. No "stops fitting" in skills/ agents/ | HOLDS | `git grep -n "stops fitting" -- skills/ agents/ \| wc -l` prints 0 |
| 4. Contract and packaging tests, lint | HOLDS | `go test ./internal/contract/... ./internal/packaging/... -count=1` exit 0 (3 packages ok); `make lint` exit 0 |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| Importers of changed packages | HOLDS | Only a new `_test.go` file was added to internal/contract; no non-test Go code changed, so importers (artifact, brain, cli, hook, job, ...) are unaffected. Not run separately. |
| No guard/gate/cap added | HOLDS | The change removes a stop rule; adds none. |
| Existing tests untouched | HOLDS | Only added files plus skills/run-job/SKILL.md:15. |

## What changed
- skills/run-job/SKILL.md:15: the "stop at the point it stops fitting" rule is replaced by "`budget.turns_estimate` is the planner's estimate, not a limit; finish the Job", plus how to post `not met` rows if the run ends early. Numbering and other lines unchanged.
- internal/contract/skill_run_job_budget_test.go: new test `TestSkillRunJobBudgetIsNotAStopRule`.

## NOT VERIFIED on a live screen
Nothing here is UI. No agent run was done.
