# C-go-agent-unset

Base: cfc06ff41c6d029a43884418471436a6d2579e75 (no unset op existed: settings.go valid list had only set and list).

## Proof table
| Row / rule | Result |
|---|---|
| 1 unset inherits | HOLDS: `go test -count=1 ./internal/cli/ -run TestGoAgentUnsetInherits` (part of the exit 0 run below); project file loses the agent, `agent list --json` shows the home value, `--json` prints only kind, name, project |
| 2 other level byte-identical | HOLDS: same test compares home settings.json bytes before and after |
| 3 no entry exits 0, bytes unchanged, unknown op exit 2 | HOLDS: TestGoAgentUnsetNoEntryAndUsage (no file is even created at a level with none); store test TestGoUnsetAgentRemovesOnlyThatEntry |
| 4 guest refused, no file changes | HOLDS: TestGoAgentUnsetGuestRefused (exit 1, same refusal path settings.go:47-48) |
| 5 tests | HOLDS: `go test -count=1 ./internal/cli/ ./internal/settings/ ./internal/contract/` exit 0; no entry added to ggFlagKnownOpen or ggProjectKnownOpen; unset takes only --project and --json, so --model is an unknown flag |
| Importers of settings | HOLDS: `go test -count=1` on internal/account, doctor, guard, hook, cmd/..., internal/packaging/... all ok (list from go list) |
| make lint | HOLDS: exit 0 (go vet) |
| Windows and macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` and the darwin one, exit 0 |
| New tests fail on base | Not run on base (budget). On base `UnsetAgent` does not exist, so the settings test does not compile, and `agent unset` exits 2 "unknown command", so the CLI tests fail. NOT run and pasted. |
| No guard/gate/cap | HOLDS: none added |
| Realplugin | none named in the brief |

## What changed
- store.go: `Store.UnsetAgent` under the same lock; writes nothing when the entry is absent.
- settings.go: `unset` op for agent, same --project selection as set; usage text. agent.go: usage line.
- commands.json via `make generate`.
- Tests: new go_agent_unset_test.go and go_unset_agent_test.go; rows in contract_inventory_test.go (:270 route list and the :443 row) and guest_parity_test.go.

## Files outside Touches (needed to keep named tests green)
- internal/cli/cli.go: `grid help` agent summary now says "set or unset" (TestExHelpSummariesNameEveryVerb requires the verb).
- testdata/golden/help.golden: same words (TestDtGateWordsHelpGolden).
- internal/cli/settings_choke_test.go: reviewed-export row for `Store.UnsetAgent`.
- internal/cli/fc_agent_contract_routes_test.go: "unset" added to the expected agent subcommand list.
The orchestrator may veto these four; each is one phrase or one list entry.

## NOT VERIFIED
No live screen involved. The unset-on-base failure was not run.
