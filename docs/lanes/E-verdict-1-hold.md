# E-verdict-1-hold: drop the usage hold and the per-task token caps; usage stays a number

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. `usageHold` and its call gone; history and recall run on every PreToolUse no guard denied | HOLDS | `internal/hook/hook.go:253-256` calls `history` and `recall` directly; `usageHold` is deleted. New `TestSpentBudgetDoesNotDenyAnAgentStart` (`internal/hook/usage_number_test.go`) drives `Run` at a spent budget: Agent and Task starts are not denied and the Bash PreToolUse history snapshot is written. On base it failed: `usage_number_test.go:58: Agent start denied at the budget: {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny",...is at its usage budget (dollars: 10 spent of a budget of 10)...` On the lane: `go test ./internal/hook` exit 0. |
| 2. `usage.Hold`, `HoldsTool`, `DefaultHoldSettings` removed; `grid usage` numbers unchanged | HOLDS | `internal/usage/hold.go` deleted (with `HoldPolicy`, `HoldSettings`, `HoldBudget`, `HoldSummary`, `HoldDecision`, `Store.HoldPolicy`). `go build ./...` exit 0. `go test ./internal/usage ./internal/cli` exit 0; the existing `TestUsageGolden` (show, post, notices goldens) and `TestUsageCLI` show/post rows pass unchanged. |
| 3. `usage budget set/clear` removed from verbs and help; plain unknown-verb message | HOLDS | `internal/cli/usage.go` `runUsageConfig` accepts only `show` for `budget`; help (`usageHelp`, `testdata/golden/usage-help.golden`) lists only `grid usage budget show [--workspace]`. New `TestUsageBudgetSetAndClearAreUnknownVerbs` (`internal/cli/usage_budget_gate_test.go`) and `TestUsageCLI/budget-set-removed`, `budget-clear-removed`: exit 2, stderr `usage budget: unknown command "set"` / `"clear"`, also with `--workspace`. These fail on base (set exits 0). `contract/commands.json` lost the two entries (hand-edited, `TestContractCommandsParserInventory` and `TestGuestProjectParity` pass). |
| 4. Deleted tests named, with the test that still covers usage numbers | HOLDS | See "Deleted tests" below. Usage numbers stay covered by `TestUsageGolden`, `TestUsageCLI`, `TestTokenNoticeCLI`, `TestTokenBudgetCLI` and the whole of `internal/usage` (`notice_test.go`, `workspace_test.go`, `usage_test.go`), all untouched. |
| 5. go test ./... and make lint: 0 fail | PARTLY | `make lint` (`go vet ./...`) exit 0. Whole-module `go test ./...` not run by me (the orchestrator runs it at landing). Changed packages `./internal/hook ./internal/usage ./internal/cli` exit 0. |
| Importers of changed packages tested | HOLDS | `go list` importers of usage/hook/cli: `cmd/grid` (no test files), `internal/cli`, `internal/packaging`. `go test ./cmd/grid ./internal/packaging ./internal/hook ./internal/usage ./internal/cli` all ok. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| New tests fail on base, pass on lane | HOLDS | Hook test failing line pasted in row 1; the CLI unknown-verb tests exit 0 on base for `budget set`, so they fail there. |
| No guard, gate, cap, hold or blocking hook added | HOLDS | Only removals, plus tests. |
| No attribution lines; no personal paths | HOLDS | Commits are one sentence with no trailer. |
| Edit only the named packages | HOLDS with note | Beyond the named files I had to touch, for the removed verbs: `internal/cli/usage_test.go`, `usage_workflow_test.go`, `usage_inheritance_test.go`, `contract_inventory_test.go`, `guest_parity_test.go`, `internal/hook/hold_hook_test.go` (deleted), `testdata/golden/*`, `contract/commands.json`, `internal/usage/usage.go` (the `HoldPolicy` field). All pin or generate the removed behaviour. `internal/cli/budget.go` is untouched. |

## What changed

- `internal/hook/hook.go`: `usageHold`, its call and the `usage` import are gone.
- `internal/usage/hold.go`, `hold_test.go`: deleted; `Store.HoldPolicy` field removed.
- `internal/cli/usage.go`: `budget set|clear` (task and `--workspace`), `parseBudgetWrite`, `usageBudgetWrite` and the workspace write path are gone; `runWorkspaceBudgetShow` replaces `runWorkspaceBudgetGated`. `budget show [--workspace]` and all `threshold` verbs stay.
- Tests that set a budget as a fixture now do it through `usage.Store.Configure` (`setTaskBudget`).

## Deleted tests

- `internal/usage/hold_test.go` (all of it: `TestHoldPolicySeam`, `TestHoldPolicySeesTheSpendAndTheBudget`, the HoldsTool and Hold-at-budget cases): the hold is removed by this lane (verdicts-11nov v-1). Usage numbers: the rest of `internal/usage`.
- `internal/hook/hold_hook_test.go` (all `TestHold*`): replaced by `TestSpentBudgetDoesNotDenyAnAgentStart`.
- `internal/cli/usage_budget_gate_test.go`: `TestUsageBudgetSetAndClear`, `TestUsageBudgetSyntaxError`; replaced by `TestUsageBudgetSetAndClearAreUnknownVerbs`.
- `internal/cli/usage_workspace_gate_test.go`: `TestUsageWorkspaceBudgetCallerRole` (set/clear role policy); `TestUsageWorkspaceBudgetShowStaysOpen` kept.
- `internal/cli/usage_test.go`: `TestWorkspaceBudgetMutationCLI`, `TestWorkspaceBudgetRefusalsCLI`, `TestWorkspaceBudgetStoreErrorsCLI` (workspace writes removed); `TestWorkspaceBudgetReplaceablePoliciesCLI` renamed `...PolicyCLI` and cut to the read policy; `TestTokenConfigurationSelectors` cut to `threshold clear --tokens`; cases `budget-over-100`, `zero-budget`, `multiple-budgets` replaced by the two unknown-verb cases; `TestTokenInvalidCLI` lost its `budget` arm; `TestTokenBudgetCLI` lost its set/clear steps (show-only steps kept); golden steps `usage-budget-set` (now a store-side setup) and `usage-budget-clear` dropped, with `testdata/golden/usage-budget-set.golden` and `usage-budget-clear.golden`.
- `internal/cli/usage_inheritance_test.go`: the `budget` first-save arm (threshold and notice arms kept).
- `TestWorkspaceBudgetRolesCLI` now covers `show` only.

## Open items

- `contract/README.md` (about lines 322-366) still describes `usage budget set|clear`. NOT DONE: out of the Touches list and I ran out of calls; it needs a docs pass.
- `usage.Store.SetWorkspaceTokens`, `usage.Change.BudgetTokens` and `Change.Budget` stay in `internal/usage` (notice tests and old tasks' stored budgets use them); nothing in the CLI calls the setters now. A later lane can remove them.
- Decision for the orchestrator: I kept `grid usage budget show` and the threshold notices, since the brief names only set/clear. With no way to set a budget, notices fire only for a workspace `tokens_per_task` or a budget an older task already stores.
- `contract/commands.json` was edited by hand because the regenerate command (`UPDATE_CONTRACT_COMMANDS=1 ...`) is outside my shell allow list; the parser inventory test confirms it matches.

## NOT VERIFIED on a live screen

Nothing in this lane has a UI. Not run: a live session that starts an agent for a task past its budget (the hook test drives `Run` with real hook input instead).
