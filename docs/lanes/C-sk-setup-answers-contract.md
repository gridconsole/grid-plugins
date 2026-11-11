# C-sk-setup-answers-contract

Base: 0e60a23a (clone head at start; brief named d0400870, which is an ancestor). The `Order` line asks for a rebase on C-sk-questions-ask-job; that lane is not in this clone, so the orchestrator rebases and reruns `make generate` at landing.

## Proof table

| Row / rule | Result |
|---|---|
| 1. New test `TestContractSetupAnswersListedInCommands` (internal/cli/contract_setup_answers_test.go) fails on base | HOLDS. On base 0e60a23a: `go test ./internal/cli -run TestContractSetupAnswers` exit 1: `contract_setup_answers_test.go:29: setup args = [], want [[project]]` and `:36: commands.json does not list --answers under setup; flags: map[--json:none --remove:none --sandbox:none --yes:none]`. Passes on the lane (exit 0). |
| 2. `make generate` changes only setup; inventory test passes without UPDATE | HOLDS. `make generate` exit 0; `git diff contract/commands.json` shows only `args: ["[project]"]` and the `--answers` (takes value) flag added under setup. `go test ./internal/cli -run '^TestContractCommandsParserInventory$' -count=1` exit 0. Probe: contract_inventory_test.go (`probeAt`, `key == "setup"` branch) runs each flag with prefix `setup` and `setup contractproject`, lists it if either knows it, `t.Fatalf` if the two disagree on `takes`; every probe still must exit 2 and `--__contract_unknown` still fails the test. The placeholder probe exited 2 for every flag (no fatal). Annotation at contract_inventory_test.go (`"setup": {args: []string{"[project]"}}`). |
| 3. contractGaps removed; packaging checks pass | HOLDS. `git grep -n "contractGaps\|setup  --answers" -- internal/packaging` prints nothing (exit 1). `go test ./internal/packaging -run 'TestShippedAgentsAndSkillsAreWellFormed|TestPluginFileChecksBiteOnBrokenFixture' -count=1` exit 0. On base commands.json with the allowlist removed it fails: `skills/setup/SKILL.md:35: ... uses flag --answers, which grid setup  does not have`. |
| 4. Bite check in internal/packaging/setup_answers_lane_test.go | HOLDS. `TestSetupAnswersSkillCheckBitesWithoutTheFlag` drops `--answers` from setup in memory, calls `checkPluginFiles` on the shipped skills, and gets `skills/setup/SKILL.md:35: ... uses flag --answers, which grid setup  does not have`; with the full surface it reports none for that file. Helper is `setupAnswersLaneProblems`. Fails on the base contract: `commands.json does not list --answers under setup`. |
| 5. `go test ./internal/cli/... ./internal/packaging/... ./internal/contract/... -count=1`; `make lint` | HOLDS. Exit 0 for `go test ./internal/cli/... ./internal/packaging/... ./internal/contract/... ./cmd/grid/... ./lib/... ./contract/... -count=1` (cli 61s, packaging, packaging/cmd, contract, lib, contract/formats all ok). `make lint` (go vet ./...) exit 0. |
| Importers of changed packages | The changes are test files plus the embedded `contract/commands.json`. Importers of `internal/cli`: `cmd/grid`, `lib`; both run above. Packages reading `contract` go through `internal/contract`, which was run; the whole-module test is the orchestrator's. |
| Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| New tests fail on base, pass on lane | HOLDS (rows 1 and 4). |
| No test edited to pass, none deleted | HOLDS. Only the `contractGaps` map and its use were removed from plugin_files_test.go (brief row 3 names this); no test function touched. |
| No guard/gate/cap on agents | HOLDS. None added. |
| Commits: one sentence, no attribution | HOLDS. One commit, no trailers. |
| guest_parity_test.go | Read, not edited. |

## What changed
- internal/cli/contract_inventory_test.go: for `setup` only, the probe runs with and without a project operand and unions the flags; `setup` args annotation is `[project]`.
- contract/commands.json: regenerated; setup gains `[project]` and `--answers` (value).
- internal/packaging/plugin_files_test.go: `contractGaps` and its use removed.
- New: internal/cli/contract_setup_answers_test.go, internal/packaging/setup_answers_lane_test.go.

## NOT VERIFIED on a live screen
Nothing here touches a UI. Rebase on C-sk-questions-ask-job and on lane-F-cli-drop-stubs (same two shared files) is for the orchestrator at landing: rerun `make generate`, never merge commands.json by hand.
