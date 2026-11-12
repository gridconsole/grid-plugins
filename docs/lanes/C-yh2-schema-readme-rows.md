# C-yh2-schema-readme-rows: land C-yh, C-yi, C-yj, C-yk, C-yl and C-ym with their contract README rows

Base: plugin main after `git merge origin/main`, stacked on lane-C-yg2-schema-readme-rows (fd62c325). No production Go file changed; only test files, embedded contract data and docs.

## What changed

- `git merge origin/main`: clean, no conflict.
- Merged in turn: `origin/lane-C-yh-account-schemas` (clean), `-yi-workflow-schemas`, `-yj-brain-schemas` (clean), `-yk-task-write-schemas` (clean), `-yl-routine-schemas` (clean), `-ym-settings-schemas`.
- Files resolved by hand:
  - `internal/cli/contract_inventory_test.go` (yi merge): kept the `output:` of `project-md set` and `setup` (from yg2/yn) and of `workflow show` and `workflow write` (from yi).
  - `contract/README.md` (ym merge): kept every row of both sides (account x5, agent budget show, approve, brain x4, claude settings x2, setup, skill x2, settings receipt); `grid approve` appeared on both sides and is kept once.
- `contract/commands.json` was not edited by hand: after the merges I took `origin/main`'s copy and ran `make generate`; the result equals the merged file (`git diff HEAD -- contract/commands.json` empty), 161 `"output"` keys.
- Added the four missing README rows, in the form of the existing rows: `grid routine add output` (routine.go:158), `grid routine list output` (routine.go:120), `grid workflow show output` (workflow.go:74), `grid workflow write output` (workflow.go:176). The line numbers are checked against `writeJSON` calls in the source.
- Lane reports: NOT DONE rows of yh, yi, yl, yn rewritten as CLOSED BY; yj and yk deviation notes marked closed; ym's README row closed and its two open items recorded as OWNER items (below), nothing changed for them.

## Proof table

| Row | Result | Command and result |
|---|---|---|
| contract tests | HOLDS | `go test -count=1 ./internal/contract/` -> `ok  github.com/gridconsole/grid-plugins/internal/contract  1.151s` (TestIySchemaRows green for the C-yg, C-yn and six lane schemas) |
| named tests | HOLDS | `go test -count=1 -run 'TestYh\|TestYi\|TestYj\|TestYk\|TestYl\|TestYm\|TestFixtures\|TestContractCommandsRejectUnknownFields\|TestContractCommandsParserInventory' -v ./internal/cli/ ./internal/contract/`: `--- PASS:` for TestContractCommandsRejectUnknownFields, TestContractCommandsParserInventory, TestYhAccountStdoutValidates, TestYhAccountListIsStatus, TestYhAccountRequiredFields, TestYiWorkflowShowSchema, TestYiWorkflowWriteSchema, TestYiWorkflowRequiredFields, TestYjBrainSearch, TestYjBrainChanges, TestYjBrainWrite, TestYjBrainDrop, TestYkTaskWriteSchemas, TestYlRoutineListAddSchemas, TestYlRoutineSchemasKeepAppFields, TestYlRoutineListAndAddDescribeOneEntry, TestYmSettingsSchemas, TestYmSkillListSchemaNamesTheCatalog, TestFixtures; last lines `ok .../internal/cli 5.997s`, `ok .../internal/contract 0.223s` |
| cli package | HOLDS | `go test -count=1 ./internal/cli/` -> `ok  github.com/gridconsole/grid-plugins/internal/cli  97.780s` |
| make generate clean | HOLDS | `make generate` ran; `git status --short` empty afterwards (commands.json unchanged from the merge) |
| lint | HOLDS | `make lint > /dev/null 2>&1 && echo LINT_EXIT_0` printed `LINT_EXIT_0` |
| cross | HOLDS | `make cross > /dev/null 2>&1 && echo CROSS_EXIT_0` printed `CROSS_EXIT_0` (Windows and macOS vet and test compile; its output also listed `ok` for the other packages) |
| importers | HOLDS | Changed packages: `internal/cli` (test files only) and `contract` data. `go list` importers of `internal/cli` or `internal/contract`: cmd/grid, artifact, brain, braincmd, cli, contract, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, question, session, settings, signal, task, updates, usage, where, workflow, lib. `make cross` ran the tests of the module and printed no FAIL (exit 0); `internal/cli`, `internal/contract` and `internal/job/lint` were also run on their own above and below. |
| walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` -> `ok  .../internal/cli 0.111s`; `go test -count=1 ./internal/job/lint/` -> `ok  .../internal/job/lint 0.289s`. The app-side walk tests do not apply to this repo. |
| Touches | HOLDS | `git diff origin/main --name-only`, outside `contract/schemas` and `contract/fixtures`: contract/README.md, contract/commands.json, docs/lanes/C-y{g,g2,h,i,j,k,l,m,n}-*.md, internal/cli/contract_inventory_test.go, internal/cli/y{g,h,i,j,k,l,m,n}_*_schema_test.go. All are the Touches of the six briefs, yg2's, or contract/README.md and the six reports. No production file. |
| new tests fail on base | n/a | This lane adds no test; the lane tests were red on base as their own reports show (unknown schema). |
| *RealPlugin* tests, mismatch proof | n/a | No row names one. |
| no guard/gate on agents | HOLDS | Nothing of that kind added. |

## OWNER items (C-ym, recorded, nothing changed)

1. Skill list `installPath`: `grid skill list --json` writes a name-to-boolean map (`internal/cli/settings.go:137`). `installPath` comes from Claude's own `installed_plugins.json` (app `skill_usage.go:160-163`). The brief row was wrong; the lane's test renames the real keys instead.
2. Budget show `blockedAdvisor` and `mainWindow`: the app reads them (`agents-budget.mjs:37`, `:60`); the plugin's `BudgetShowResult` (`internal/settings/budget.go:388-397`) never writes them. The owner decides between a page change and a plugin change. This is a product gap, not a test fix.

## NOT VERIFIED

Nothing here has a screen. The app-side reads were not re-opened in this lane; the six lane reports hold those checks.

Verdict: DONE
