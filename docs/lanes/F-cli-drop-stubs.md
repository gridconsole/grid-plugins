# F-cli-drop-stubs

Plugin repo, lane branch `lane-F-cli-drop-stubs`. The stub verbs `views`, `ask`, `machine`, `deploy`, `sync` and the verb `ports` are removed. `env` stays: it has a live caller.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Callers outside each verb's own files | HOLDS | Command: `git grep -nE "\bgrid (env\|ports\|views\|ask\|machine\|deploy\|sync)\b"` over skills, hooks, agent-templates, docs, evals, golden and Go sources, exit 0. **env: one live caller, `skills/browser/SKILL.md:11` (`grid env up <job>`, `grid env down <job>`), so `env` stays and is named here.** ports: no caller outside the CLI, only history in `docs/lanes/L30.md`, `docs/lanes/D-plugin-skills-c.md:10`. views: `views/README.md:1` (prose, fixed). sync: `contract/formats.md:25` (prose for a spec-only manifest, reworded). ask, machine, deploy: none. hooks.json: none of the seven. `internal/packaging/layout_test.go:104` `TestSkillCommandsImplemented` checks skills only name implemented verbs. |
| 2. Verbs with no live caller are gone; packages deleted when unimported | HOLDS for the verbs, packages kept on purpose | `internal/cli/cli.go` verb table no longer has views, ask, machine, deploy, sync, ports. `internal/env` and `internal/ports` are NOT deleted: `internal/cli/environment.go:12,15` (`grid env`) imports both, and `internal/env/{env,config,process_unix,env_test}.go` imports `internal/ports`. `git grep -n 'internal/ports"'` and `git grep -n 'internal/env"'` show those only. `runPorts` is deleted from `internal/cli/environment.go`; `printPorts` stays because `runEnv` uses it. Dead "nil run = not built yet" branches removed from `Run` and `printHelp` (`cli.go`), as no stub is left. |
| 3. `grid <removed verb>` prints the unknown-verb help | HOLDS | `TestRemovedVerbsAreUnknown` (`internal/cli/cli_test.go`) runs views, ask, machine, deploy, sync, ports: exit 2, stderr `grid: unknown command "<v>". Run: grid help`, empty stdout, `Implemented(v)` false, help lists none. Fails on base (main `cli.go` and `environment.go`, exit 1): `grid views: exit 2, stderr "views: not built yet\n"`, `grid ports: exit 0, stdout "Usage: grid ports claim ..."`, `ports is still implemented`. Passes on the lane. |
| 4. Contract command inventory no longer lists them | HOLDS, no golden regen needed | `contract/commands.json`: entries views, ask, machine, ports, deploy and the top-level sync deleted by hand (the `question sync` subcommand is kept). `TestContractCommandsInventory` and `TestContractCommandsParserInventory` pass on the lane (`go test ./internal/cli/ -count=1`, exit 0), so no `UPDATE_CONTRACT_COMMANDS=1` regeneration is needed. Orchestrator: re-run it once if you want the file in the generator's exact bytes; I expect no diff. Parser annotations for `ports *` removed from `internal/cli/contract_inventory_test.go`. |
| 5. go test ./... and make lint | HOLDS for the packages I changed and their importers; whole-module test is the orchestrator's | See the commands below. No failing test, including the golden test. |
| Rule: new tests fail on base, pass on lane | HOLDS | Row 3. `TestEnvUpRefusesAJobInTwoTasks` is the old `TestPortsCommands` "multiple tasks" case moved to `env up`; it passes on base too, by design. |
| Rule: no existing test edited to pass; deleted tests named | HOLDS | `TestPortsCommands` deleted: behaviour removed by the owner's direction (`/srv/gridwork/briefs/needed.md:41`, "ports ... goes"; this brief). Its still-valid case (a Job id in two tasks is refused) is covered by `TestEnvUpRefusesAJobInTwoTasks`; the port claim, release and release-on-down are covered by `internal/ports/ports_test.go` and `TestEnvironmentCLI`, which now reads `ports.Store.List()` instead of `grid ports list`. `TestHelpListsEveryCommand` lost its "(not built yet)" assertion: that marker is removed with the last stub (same direction). `TestEnvironmentUsage` lost its `ports` argument rows (verb removed). |
| Rule: no guard, gate, cap, hold or blocking hook added | HOLDS | Diff only deletes verbs and adds one test. |
| Rule: Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Rule: lint | HOLDS | `make lint` (`go vet ./...`) exit 0. |
| Rule: tests of changed packages and importers | HOLDS | Importers of `internal/cli` from `go list` (imports, tests, external tests): cmd/grid, contract/formats, internal/cli, internal/golden, internal/mcp, internal/packaging, internal/question, internal/settings. `go test ./internal/cli/ -count=1` exit 0 (120s); `go test ./cmd/grid ./contract/... ./internal/golden ./internal/mcp ./internal/packaging ./internal/question ./internal/settings ./internal/env ./internal/ports -count=1` exit 0. |
| Rule: small commits, one sentence, no attribution lines | HOLDS | Two commits plus this report. |

## What changed

- `internal/cli/cli.go`: five stub rows and the `ports` row out of the verb table; no more nil-run handling.
- `internal/cli/environment.go`: `runPorts` removed; `grid env up|down` unchanged.
- Tests: `cli_test.go` (new `TestRemovedVerbsAreUnknown`), `environment_test.go`, `contract_inventory_test.go`, as above.
- `contract/commands.json`: six top-level entries removed.
- `contract/formats.md:25`, `views/README.md:1`: prose that named the removed verbs.

## Open items

- `grid env` stays only because `skills/browser/SKILL.md:11` calls it. `needed.md:53` says the browser for checker Jobs is dropped ("Claude in Chrome exists"). If the owner means the dev environment too, drop that line in the skill (outside this brief's touch list) and then remove `env`, `internal/env` and `internal/ports` in one follow-up; `internal/ports` is imported only by them.
- `docs/lanes/*.md` history still mentions the removed verbs (L30, L49, D-plugin-skills-c); left as history.

## NOT VERIFIED on a live screen

Nothing here is a UI row. I did not run the built `grid` binary; the unknown-verb answer is proved by the Go test only.
