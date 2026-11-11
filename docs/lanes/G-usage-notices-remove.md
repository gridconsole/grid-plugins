# G-usage-notices-remove

Removes usage thresholds, usage notices and the task/workspace budget read from the plugin.
Owner's direction: simplify.md row 1 ("Drop the holds and caps. Keep usage as a number the window shows"),
needed.md row 1 and R5 ("Usage notice and cost notice, get rid of that").

## Proof table

| Row | Result |
|---|---|
| 1. Inventory, remove or keep | HOLDS. Below. |
| 2. `usage threshold ...` / `usage budget ...` exit 2; help does not name them; post/show unchanged | HOLDS. `TestUsageThresholdAndBudgetVerbsRemoved` (11 argument sets, exit 2, `unknown command`, no `usage/` folder written); `TestTokenHelp` (help names none of threshold, budget, PERCENT, notice, tokens_per_task, workspace.md) plus `testdata/golden/usage-help.golden`. `usage-post-lead`, `usage-show-empty`, `usage-show-totals` goldens and `TestTokenUsageCLI`, `TestUsageCLI` post/show cases pass with their expectations unchanged. |
| 3. Post across old 50/80/100 writes no notice, prints no usage-notice line | HOLDS. `TestUsagePostAtOldThresholdsWritesNoNotice`: dollars, minutes, tokens (task budget in old thresholds.json) and tokens (workspace.md `tokens_per_task`), posting 49/50/79/80/99/100/150 percent of a 100 budget; no `usage-notice` in output, empty stderr, empty `notices`/`warnings`, no usage-notice feed records, totals still recorded. |
| 4. Old budget / thresholds / `usage_notice_thresholds` read without error and are ignored | HOLDS. `TestUsageIgnoresOldBudgetsAndThresholds`: valid, sparse v2, unreadable, invalid-version thresholds.json; valid, invalid, no-front-matter and unclosed workflow.md; `workspace.md` default; show and post exit 0, empty stderr, old file byte-identical afterwards. `TestSpentBudgetDoesNotDenyAnAgentStart` (hook) uses an old-format budget file. |
| 5. contract/README.md; help golden and command inventory | HOLDS for README (`contract/README.md:314-322`), `contract/commands.json` (budget and threshold subcommands and the summary removed) and `testdata/golden/usage-help.golden`. `make golden` was NOT run: env-var-prefixed commands (`UPDATE_CONTRACT_COMMANDS=1`, `UPDATE_GOLDEN=1`) are refused in the sandbox, so `commands.json` and the two usage goldens were edited by hand and are checked by `TestContractCommandsParserInventory`, `TestContractCommandsInventory` and `TestUsageGolden`, which pass. The orchestrator should re-run `make golden` at landing. |
| 6. Tests and lint | HOLDS. `go test ./internal/cli/... ./internal/usage/... ./internal/workflow/... ./internal/hook/... ./internal/golden/... ./contract/...` exit 0 (all `ok`); `make lint` exit 0. The importers `internal/feed`, `internal/task` and `cmd/grid` were not run in this session (call budget): NOT DONE, left for the orchestrator's whole-module run. |
| No guard, gate, cap, hold or hook added | HOLDS. Only removals; nothing added to any agent path. |
| New tests fail on base | The new tests are the three above plus the removed-verbs test. On base, `usage threshold show` / `usage budget show` exit 0 (not 2), a post across 80 percent prints `usage-notice` and writes a feed record, and the help names the verbs, so all four fail there. Not re-run on a base checkout in this session (call budget). |
| Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| `make lint` | HOLDS. exit 0 (`go vet ./...`). |
| No attribution lines, no personal paths | HOLDS. Commit messages are one sentence with no trailer. |

## Inventory (row 1)

Removed:
- `internal/cli/usage.go`: `threshold`/`budget` dispatch, `runUsageConfig`, `runWorkspaceBudgetShow`, the usage-notice print loop, `usageWorkflowWarning`, the help lines (threshold, budget, defaults, per-unit precedence, workspace read). Reason: the verbs and the notice line are the removed feature.
- `internal/cli/cli.go:137`: command summary no longer says "set budgets and notice thresholds".
- `internal/usage/notice.go`: Configuration, Change, NoticeThresholds, settings, Config, Configure, readSettings, saveConfig, postNotices, tokenCrossing, tokenBudget. Kept: `ParseTokens`, moved to `usage.go` (still used by `--tokens`).
- `internal/usage/workspace.go`: `WorkspaceBudgetReadAllowed/WriteAllowed`, `WorkspaceTokens`, `SetWorkspaceTokens`. No caller left (the app ops that used it are removed by E-verdict-1-app-limit).
- `internal/usage/usage.go`: Store fields `Writer`, `UsageNoticeThresholds`, `WorkflowWarning`, `beforeSave`; the `s.notices` call in `Post`.
- `internal/workflow/usage.go`: `UsageNoticeThresholds` and its list parser, with their test file.
- `contract/README.md`, `contract/commands.json`: documentation and inventory of the removed verbs and key.
- Tests deleted, all of removed behaviour (owner's direction above): `internal/usage/{notice,inheritance,config_compat,identity,workspace}_test.go`, `internal/workflow/usage_test.go`, `internal/cli/usage_{budget_gate,inheritance,workflow,workspace_gate}_test.go`, and in `internal/cli/usage_test.go` the threshold/budget/workspace/notice tests; five goldens (`usage-post-notices`, `usage-threshold-*`). Post/show behaviour is still covered by `usage_test.go` in both packages.
- Existing tests edited only where they named removed things: `internal/cli/contract_inventory_test.go`, `internal/cli/guest_parity_test.go` (command tables), `internal/workflow/policy_file_test.go` (dropped the `UsageNoticeThresholds` call, gate assertions kept), `internal/hook/usage_number_test.go` (budget now written as an old-format file), `internal/usage/usage_test.go` (two `Config()` calls dropped).

Kept:
- `internal/usage/fold.go`, `Store.Post/Show`: usage as a number.
- `PostResult.Notices` / `Warnings`: kept in the post JSON, always empty, so the `usage-post-lead` golden and any reader of the answer keep their shape.
- `usage-notice` feed type, schema, fixtures and `internal/contract/schema.go:426`, `internal/feed` tests: old feeds still hold these records and must still read. Dropping the type is a separate contract change.
- `internal/cli/budget.go`, `internal/settings/budget.go`: the agent budget switch, not in scope.
- `loop-detected` `threshold` field (contract/schemas/loop-detected.json): unrelated.

## Commands

- `go test ./internal/cli/... ./internal/usage/... ./internal/workflow/... ./internal/hook/... ./internal/golden/... ./contract/...`: see result line at the end of this file.
- Packages that import the changed ones (go list): cmd/grid, internal/cli, internal/feed, internal/task, internal/usage, internal/workflow; hook and cli tests also import usage.
- `GOOS=windows CGO_ENABLED=0 go build ./...`: exit 0. `GOOS=darwin CGO_ENABLED=0 go build ./...`: exit 0. `make lint`: exit 0.

## NOT VERIFIED on a live screen

Nothing here is UI. The app's token_limit ops are not touched; the orchestrator checks that none of them still call `grid usage budget` after E-verdict-1-app-limit lands.
