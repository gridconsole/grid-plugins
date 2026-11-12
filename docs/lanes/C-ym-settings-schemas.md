# C-ym-settings-schemas: output schemas for skill list, skill enable, agent set, claude settings show and set, agent budget show

Base: plugin main 103b9157. On base the six commands had `"output": ""` in `contract/commands.json`, so no schema named them.
Code head before this doc: 8f278086 (commits f30f9709 schemas, fixtures, test; 8f278086 README rows and gofmt). The doc commit comes after it.

**Verdict: DONE for the schemas; two OWNER items stay open (below). The README row (was NOT DONE) is CLOSED BY C-yh2-schema-readme-rows.** Five schemas, fixtures, annotations and the test hold. Two things the brief names cannot hold, and the brief says a field the app reads that the plugin does not write stops the lane NOT DONE (rows 2 and the budget fields below). Nothing in the plugin's output was changed.

## What the app reads (app main 8f9b806d), by site

The brief lists the fields before `required` is written. Checked at the pinned sha with `git show 8f9b806d:<path>`:

| Command | App site | What it reads from the plugin's stdout |
|---|---|---|
| skill list | `internal/slices/settings/skill_usage.go:102-107` | the whole stdout as `map[string]bool`, then `skills[name]` (:107). No object fields. |
| skill list | `skill_usage.go:71-73` | `name`, `projects`, `agents` are the app's own HTTP reply, not plugin output. |
| skill list | `skill_usage.go:160-163`, `internal/slices/agentreads/http.go` `pluginInstall` | `plugins[].installPath` and `version` are Claude Code's `installed_plugins.json`, not plugin output. |
| skill enable | `internal/slices/settings/skill_enable.go:106-111` | nothing: `output` is dropped, the reply is built from the request body. |
| agent set | `save_agent.go:82-97`, `internal/slices/agentaccounts/override.go:46` | nothing: `output` is dropped (`_, err := h.run(...)`). |
| claude settings show | `internal/slices/claudesettings/http.go:94` relays raw; `internal/kernel/webui/app/pages/settings/agents-settings-json.mjs:33-110`, `agents.mjs:90` | `path`, `state`, `readAt`, `mainModel.value/window`, `rows.<key>.now/inForce/setBy/blocked`, `codex.exists/path`. Shape documented in `client.mjs:730-731`. |
| claude settings set | `http.go:144-148` relays raw; `client.mjs:732` | `{ path, key, from, to, written }`. |
| agent budget show | `agentreads/http.go:80` relays raw; `agents-budget.mjs:29,37,55,60,63,69` | `agents`, `presets` (:29), `env` (:55), `undo` (:63), `note.state` (:69), `blockedAdvisor` (:37), `mainWindow` (:60). |

## Proof table

| Row | Result | Command / place |
|---|---|---|
| 1. Red on base | HOLDS | Run of the new test file on the base tree earlier in this session: `cmd-skill-list: unknown schema "cmd-skill-list"`, and the same `unknown schema` line for `cmd-settings-receipt`, `cmd-claude-settings-show`, `cmd-claude-settings-set`, `cmd-agent-budget-show`. The base output was not saved to a file, so the test line numbers are not pasted here. To reproduce: add a worktree at 103b9157, copy `internal/cli/ym_settings_schema_test.go` into it and run the row-1 command. |
| 1. Green on lane | HOLDS | `go test -count=1 -v -run 'TestYm' ./internal/cli/` exit 0: `--- PASS: TestYmSettingsSchemas/skill_list`, `/skill_enable`, `/agent_set`, `/claude_settings_show`, `/claude_settings_set`, `/agent_budget_show` (each with a basic and a populated run, `--json`, harness home, `HOME` and `CLAUDE_CONFIG_DIR` under `t.TempDir()`), `--- PASS: TestYmSkillListSchemaNamesTheCatalog`. Each stdout passes `contract.Validate`. |
| 2. receipt `name` | HOLDS | `read: {"name"},{"kind"},{"project"}` in `TestYmSettingsSchemas` (`ym_settings_schema_test.go`, cases `skill enable`, `agent set`): a renamed and a retyped copy fail Validate. |
| 2. budget `agents`, `presets` | HOLDS | case `agent budget show`, `read: {"agents"},{"presets"},{"env"},{"undo"},{"note","state"}`: renamed and retyped copies fail Validate. |
| 2. skill list `installPath` | OWNER item, not a red of C-yh2 (recorded by C-yh2-schema-readme-rows, nothing changed): `grid skill list --json` writes a name-to-boolean map (`internal/cli/settings.go:137`); `installPath` comes from Claude's own `installed_plugins.json` (app `skill_usage.go:160-163`), so the brief row was wrong. Detail: The skill list stdout has no `installPath` or `version` (`settings.go:137` writes `doc.Skills`, a name-to-boolean map). The app reads `installPath` from Claude's `installed_plugins.json` (`skill_usage.go:160-163`), not from this command. The test renames the real fields instead: `run-job` and `task` (`skills[name]`, `skill_usage.go:107`). No field was invented. |
| Budget show `blockedAdvisor`, `mainWindow` | OWNER item, not a red of C-yh2 (recorded by C-yh2-schema-readme-rows, nothing changed): the app reads both (`agents-budget.mjs:37`, `:60`); the plugin's `BudgetShowResult` (`internal/settings/budget.go:388-397`) never writes them. The owner decides: the page reads `rows.advisorModel.blocked`, or the plugin writes the two fields. Detail: | Read at `agents-budget.mjs:37` (`data.blockedAdvisor ?? false`) and `:60` (`hooks.mainWindow?.() ?? data.mainWindow`). `BudgetShowResult` (`internal/settings/budget.go:388-397`) writes `note, presets, codexPresets, agents, applied, closest, custom, env, undo` only. Not in the schema, not made optional. Effect today: the page's advisor checkbox is never disabled by the budget read, and `data.mainWindow` is always unset (`agents.mjs:90` supplies the window from `claude settings show` `mainModel.window`, which is written). The owner picks: the page reads `rows.advisorModel.blocked` (it already does at `agents-settings-json.mjs:46`), or the plugin writes the two fields. |
| 3. Fixtures, unknown fields, inventory | HOLDS | Final proof run on 8f278086 (`go test -count=1 -v` over `./internal/cli/ ./internal/contract/`) printed `--- PASS: TestFixtures`, `--- PASS: TestContractCommandsRejectUnknownFields`, `--- PASS: TestContractCommandsParserInventory`, and `--- PASS: TestIySchemaRows`. |
| 4. `git diff main -- contract/commands.json` adds only `output` values | HOLDS | 6 lines `"output": ""` -> `"output": "<schema>.json"` (skill list, skill enable, agent set, claude settings show and set, agent budget show), made by `make generate`. |
| 4. `git diff main --stat` names only Touches | CLOSED BY C-yh2-schema-readme-rows (was NOT DONE; the README rows are carried by that repair lane, whose Touches name `contract/README.md`) | One file outside Touches: `contract/README.md` (+5 rows). `TestIySchemaRows` (`internal/contract/iy_schema_rows_test.go:47`) failed with `schema cmd-... has no row in the contract docs` for the five new schemas; it is a repo-wide walk test the runbook requires green. It is its own commit (8f278086): revert it if the orchestrator prefers to add the rows at landing, and `TestIySchemaRows` is then red until it does. |
| 5. `go test ./internal/cli/ ./internal/contract/` | HOLDS | Final proof run on 8f278086: `ok` for `internal/cli` and `internal/contract`, 0 failures. Before the README commit, `internal/contract` had one failure, `TestIySchemaRows` (see row 4). |
| 5. `make lint` | HOLDS | Failed once with `gofmt needed: internal/cli/ym_settings_schema_test.go`; after `gofmt -w` it passed. The exit code was not printed on its own: the `&&` chain went on to `make cross`, which only runs when lint exits 0. |
| 5. `make cross` | HOLDS, exit code inferred | Printed the four cross steps (darwin/arm64 and windows/amd64, each `vet` and `test compile`) with no `make:` error line. The exit code itself was not read. |
| Importers of `internal/contract` and `internal/cli` | PARTLY RERUN | One `go test -count=1` run over the packages that import them (cmd/grid and the internal packages) was ok except `TestIySchemaRows`, which 8f278086 fixed. That full list was not rerun after 8f278086; the commit changes README text and the gofmt of the new test only, but it is not proven here. |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. The app-side walk tests do not apply to the plugin repo. |
| `*RealPlugin*` tests | not applicable | No row of this brief names one. |
| No guard, gate or cap added | HOLDS | No production file changed; the diff is schemas, fixtures, one test file, the inventory annotations, `commands.json` and the README rows. |

## What changed

- `contract/schemas/cmd-skill-list.json`: the 12 skills of `internal/settings/catalog.go:30`, each boolean and required, `additionalProperties: false`. `TestYmSkillListSchemaNamesTheCatalog` fails when a skill is added to the catalog without a schema line.
- `contract/schemas/cmd-settings-receipt.json`: `{kind, name, project}` for skill enable and agent set (the same object, `settings.go:169`).
- `contract/schemas/cmd-claude-settings-show.json`, `cmd-claude-settings-set.json`, `cmd-agent-budget-show.json`: every field the plugin writes, nullable values as `anyOf` with a null branch (the checker only honours a string `type`), the five Claude rows required.
- Fixtures: valid basic and populated runs from the real CLI, invalid renamed and retyped copies. Written by `make test UPDATE_YM_FIXTURES=1 "GOFLAGS=-count=1 -run=TestYmSettingsSchemas"`; without the variable the test writes nothing.
- `contract_inventory_test.go`: six `output:` values. `contract/commands.json` regenerated.
- `internal/cli/ym_settings_schema_test.go` (new): `TestYmSettingsSchemas`, `TestYmSkillListSchemaNamesTheCatalog`.

## Notes for the backlog

- `skill disable`, `agent unset` and `hook level` write the same receipt but carry no annotation (the brief listed enable and set only).
- A hand-edited `settings.json` that names a skill outside the catalog would make `skill list` output fail the closed schema; the app reads only `skills[name]`, so it would not notice.

## NOT VERIFIED on a live screen

The Skills, Agents and budget pages were not opened; the schemas are checked against the plugin's own stdout on a harness home, not against a running app relay.
