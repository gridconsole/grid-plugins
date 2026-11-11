# C-fh-agent-contract-routes

Base: b17389b791d078adc7377e41457ed8e54a7526c2. Code head: f97e3f5f (this report is committed after it).

Base check: `git grep -n 'paths\["agent"\]' internal/cli/contract_inventory_test.go` gave `internal/cli/contract_inventory_test.go:267:	paths["agent"] = []string{"list", "set"}` (no check, sync or budget). internal/cli/ex_help_summary_verbs_test.go exists on the base.

## Proof table

| Row | Result |
|---|---|
| 1 TestFcAgentRoutesInContract | HOLDS. `go test -count=1 ./internal/cli/ -run 'TestContractCommands\|TestFcAgent\|HelpGolden\|TestEx'` exit 0. Fails on base (contract restored to base): `fc_agent_contract_routes_test.go:78: agent subcommands in contract = [list set], want [budget apply budget show budget undo check list set sync]` |
| 2 TestFcAgentAppReadIsNamed | HOLDS, same run. Fails on base: `fc_agent_contract_routes_test.go:102: contract/commands.json lacks "agent budget show", which the app calls (internal/slices/agentreads/http.go:75)`. The test comment cites http.go:75 (fc_agent_contract_routes_test.go:94-95). |
| 3 TestFcAgentBudgetExits | HOLDS, same run: `agent budget nope` and `agent budget apply` exit 2; `agent budget --help` exits 0 and prints the budget.go:24 usage. This test also passes on base: it pins behaviour that already existed, so it is not a fail-on-base test. |
| 4 `go test -count=1 ./internal/cli/ -run 'TestContractCommands\|TestFcAgent\|HelpGolden\|TestEx'` | HOLDS, exit 0 (`ok .../internal/cli 4.628s`). |
| 5 `git diff main --stat` and `make lint` | HOLDS. Diff names only contract/commands.json, internal/cli/cli.go, internal/cli/contract_inventory_test.go, internal/cli/fc_agent_contract_routes_test.go, testdata/golden/help.golden (5 files, 229 insertions, 4 deletions), taken before this report was added. `make lint` exit 0 (go vet clean, gofmt prints nothing). |
| Tests of changed packages and importers | HOLDS. Packages: internal/cli, contract (embeds commands.json), plus importers from go list: cmd/grid, lib, contract/formats, internal/braincmd, internal/contract, internal/hook, internal/packaging, internal/packaging/cmd. `go test -count=1` on all: ok (internal/cli 54.693s; cmd/grid and contract have no test files). |
| Windows and macOS builds | HOLDS. `make cross` (vet and test compile for darwin/arm64 and windows/amd64): output holds no FAIL or error lines. The exit code itself was not captured: a second run to print `$?` was refused. |
| commands.json regenerated, not hand-edited | HOLDS. `make generate` ran clean (exit 0 on each step); diff adds the seven agent routes and the new summary. |
| help.golden by hand from the failing diff, not UPDATE_GOLDEN | HOLDS. Edited the agent line only, copied from the failing `TestDtGateWordsHelpGolden` diff. (`make generate` itself runs UPDATE_GOLDEN=1 for `TestDoctorGolden` only; it changed no file.) |
| No guard, gate, cap, hold or blocking hook | HOLDS. None added. |
| No existing test edited or deleted | HOLDS. contract_inventory_test.go changed only at the agent paths line and five annotation entries; nothing deleted. |
| Commits | One commit, one-sentence message, no attribution lines. |
| RealPlugin tests | NOT NEEDED. No row names one. |

## What changed

- internal/cli/contract_inventory_test.go: `paths["agent"]` is check, list, set, sync, budget show, budget apply, budget undo, with a comment on why budget routes are listed by hand; annotations for the five new routes (`agent budget apply` takes `<economic|balanced|high>`).
- contract/commands.json: regenerated with `make generate`.
- internal/cli/cli.go and testdata/golden/help.golden: agent summary is now "check agent readiness; sync templates; set models and efforts; budget; list settings" (the ex verbs test passes).
- internal/cli/fc_agent_contract_routes_test.go: TestFcAgentRoutesInContract, TestFcAgentAppReadIsNamed, TestFcAgentBudgetExits.

## NOT VERIFIED on a live screen

Nothing here is a UI row. The app's real call to `grid agent budget show --json` was not run (no app, no curl); only the contract and the CLI exit codes are proven.

## Notes

- The brief limits the diff to five files; this report is a sixth, as the runbook requires.
- `agent budget show` etc. have `"output": ""` in the contract, like the other agent routes; no output schema file exists for them.
