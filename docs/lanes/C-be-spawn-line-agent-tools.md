# C-be-spawn-line-agent-tools: the worker's spawn line carries the tools the role's agent file names

Base: `7a4bc0c2d6c9a1ef1c7bec89e38f9bca07767c29` (the brief quotes `2221cb86`; the line references in the brief still matched). Base check: `git grep -n "Effort" -- internal/orchestrator/stream.go` printed only the `Model`/`Effort` comment, field and fill (stream.go:105, 108, 273, 275); `spawnLine` had no tools field.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. TestBeSpawnCarriesCheckerTools | HOLDS | internal/orchestrator/be_spawn_tools_test.go:18. It posts J-1 role checker and J-2 role builder, sets `s.Tools`, and finds the literal `"tools":["Read","Grep","Glob","Bash","mcp__claude-in-chrome__*","mcp__Claude_Browser__*"]` in the raw output (one tools key in all; J-2 has the nine keys). Base (new test files only, `go test -count=1 -v -run 'TestBe\|TestStreamSpawn' ./internal/orchestrator/ ./internal/cli/`): build failed, `be_spawn_tools_test.go:22:4: s.Tools undefined (type *Stream has no field or method Tools)` (also :53, :69), `FAIL github.com/gridconsole/grid-plugins/internal/orchestrator [build failed]`. Lane: `--- PASS: TestBeSpawnCarriesCheckerTools`. |
| 2. TestBeWorkerSpawnToolsFromAgentFile | HOLDS | internal/cli/be_worker_spawn_tools_test.go:30. checker answers the six tools of agents/checker.md:6 in order (also under the role name `grid-checker`); builder, orchestrator and curator hold `Skill`; writer answers nil. Base: `be_worker_spawn_tools_test.go:14:14: undefined: workerSpawnTools`, `FAIL .../internal/cli [build failed]`. Lane: `--- PASS: TestBeWorkerSpawnToolsFromAgentFile`. |
| 3. TestBeBrainAgentToolsWin | HOLDS | internal/cli/be_worker_spawn_tools_test.go:46. Home `knowledge/agents/grid-checker.md` with `tools: [Read, mcp__claude-in-chrome__*]` answers exactly those two. Lane: `--- PASS: TestBeBrainAgentToolsWin`. Also passing: TestBeProjectBrainAgentToolsBeatHome (project level before home), TestBeBrainAgentWithoutToolsAnswersNil, TestBeBrainAgentOfRoleWithoutCatalogFile, TestBeAgentToolsForms (string, list, flow list, repeats, empties, map, number, mixed list, duplicate key, no front matter). |
| 4a. `go test -count=1 -v -run 'TestBe\|TestStreamSpawn' ./internal/orchestrator/ ./internal/cli/` | HOLDS | exit 0. `--- PASS:` for TestBeSpawnCarriesCheckerTools, TestBeSpawnEmptyToolsKeepsTheNineKeys, TestBeSpawnToolsErrorGoesOutWithoutTools, TestStreamSpawnCarriesTheRoleModel, **TestStreamSpawnWithoutAModelKeepsTheNineKeys**, TestStreamSpawnSurvivesBrokenSettings, TestStreamSpawnsEveryTask, TestBeWorkerSpawnToolsFromAgentFile, TestBeBrainAgentToolsWin, TestBeProjectBrainAgentToolsBeatHome, TestBeBrainAgentWithoutToolsAnswersNil, TestBeBrainAgentOfRoleWithoutCatalogFile, TestBeAgentToolsForms; `ok .../internal/orchestrator`, `ok .../internal/cli`. |
| 4b. `go test -count=1 ./internal/orchestrator/ ./internal/cli/` | HOLDS | exit 0 (run together with the importers below): `ok .../internal/orchestrator 233.621s`, `ok .../internal/cli 117.532s`. |
| 5a. `make lint` | HOLDS | `make lint >/dev/null 2>&1 && echo LINT-EXIT-0` printed LINT-EXIT-0 (`go vet ./...`). `gofmt -l internal/orchestrator internal/cli` printed nothing. |
| 5b. `make cross` | HOLDS | `make cross >/dev/null 2>&1 && echo CROSS-EXIT-0` printed CROSS-EXIT-0 (Windows and macOS builds). |
| 5c. `git diff 7a4bc0c2 --stat` names only Touches files | HOLDS | internal/cli/be_worker_spawn_tools_test.go, internal/cli/worker.go, internal/orchestrator/be_spawn_tools_test.go, internal/orchestrator/stream.go; 294 insertions, 0 deletions. The one other path is this report, which the runbook asks for. contract/commands.json is unchanged. |

## Rules

| Rule | State | Proof |
|---|---|---|
| Tests of changed packages and every importer | HOLDS | Importers from `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...` filtered to internal/orchestrator and internal/cli: cmd/grid, internal/cli, internal/mcp, internal/migrate, lib. `go test -count=1 ./internal/orchestrator/ ./internal/cli/ ./cmd/grid/ ./internal/mcp/ ./internal/migrate/ ./lib/` exit 0: orchestrator ok, cli ok, cmd/grid no test files, mcp ok, migrate ok 12.077s, lib ok 41.525s. |
| Plugin walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` `ok .../internal/cli 0.750s`; `go test -count=1 ./internal/job/lint/` `ok .../internal/job/lint 0.670s`. |
| No existing test edited or deleted | HOLDS | `git diff 7a4bc0c2 --stat` shows 0 deletions and no existing test file. |
| No guard, gate, cap or hold on agents | HOLDS | The stream and `workerSpawnTools` report what the agent file says: no filter, no allowlist, no added or removed tool. An error costs the line its tools key and one Diag line, never the Job (be_spawn_tools_test.go `TestBeSpawnToolsErrorGoesOutWithoutTools`). |
| *RealPlugin* tests, mismatch proof | n/a | No row names either. |
| Commit rules | HOLDS | One commit `dbef5963`, one-sentence message, no attribution line. No personal path, host or email in code, tests or this report. |

## What changed

- `internal/orchestrator/stream.go`: `spawnLine.Tools []string \`json:"tools,omitempty"\``; `Stream.Tools func(taskDir, role string) ([]string, error)` beside `Model`; the fill follows the model fill. An error is one Diag line (`... no tools for role "checker": <err>`) and the line goes out without tools. A Stream with no `Tools` callback, or one that answers nothing, prints the nine keys (TestStreamSpawnWithoutAModelKeepsTheNineKeys, TestBeSpawnEmptyToolsKeepsTheNineKeys).
- `internal/cli/worker.go`: `workerSpawnTools(env, dir, role)` set on the Stream beside `Model`. The agent is the one the Instructions use: the brain agent for the role (`brain.Store{...}.AgentFor(role)`, project then home), else `gridplugins.AgentFor(contract.NormalizeRole(role))`. A brain agent with no `tools:` answers nil; the catalog file is never laid under it. `beAgentTools` reads `tools:` as a comma-separated string or a list: entries trimmed, empties and repeats dropped, order kept; any other shape is no value.
- Finding that shaped the reader: `agents/builder.md` (and `agents/curator.md`) have a `: ` inside the `description:` plain scalar, which is not strict YAML, so `mdfile.Load` of the whole front matter fails and row 2's "builder holds Skill" would have answered nil. `beToolsFragment` therefore parses only the lines of the one top-level `tools:` key (the key line and the indented or `- ` lines under it). Brain agents have already passed the strict check in `AgentFor`, so for them it reads the same value.

## Notes for the backlog

- `agents/builder.md:3` and `agents/curator.md:3` are not strict YAML; a copy of either into the brain would be skipped by `AgentFor` and flagged by `grid brain check`. Not touched here (outside Touches).
- A comma inside parentheses in a string-form `tools:` (`Bash(git log, git diff)`) is split at the comma; the brief asks for a plain comma split and no shipped agent file has one. A list form keeps such an entry whole.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run, as the brief says: `claude`, a live `grid worker next` against the app. The app side (`C-be-agentbuild-agent-tools`) decides how the `tools` key is passed.

## Verdict

DONE
