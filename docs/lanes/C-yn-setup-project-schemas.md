# C-yn-setup-project-schemas

Base: plugin main `103b915799ce05bb20c8d92fbf823650b37c5c25`. On that sha `runProject` (internal/cli/projects.go:68-75) starts

```
func runProject(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "usage: grid project add|remove|list|scan|relink|repo")
		return 2
	}
	verb := args[0]
	switch verb {
	case "repo":
```

and none of the six annotations (contract_inventory_test.go:552 project add, :553 remove, :555 scan, :557 repo, :580 project-md set, :581 setup) had an `output:`.

Verdict: NOT DONE. Everything in the brief is built and green except acceptance row 5. `go test ./internal/contract/` has one red test, `TestIySchemaRows`, and its fix is a file outside the brief's Touches (`contract/README.md`). The lane does not touch that file; see row 5.

## What the app reads (app main `8f9b806d`, read with `git show`; lines as the brief names them)

| Command | App site | Fields read | Schema `required` |
|---|---|---|---|
| `setup --json`, `setup --yes --json` | internal/slices/setup/machine.go:26, :51 (`wfMachineResult`, `runMachine`) | `path` (non-empty), `added` (not null), `present` (not null), `written` | path, added, present, written |
| `project scan <root> --json` | internal/slices/setup/http.go:148-160 (`scan`, `found` struct) | `name`, `path`, `rule`, `registered`, `error` | name, path, rule, registered (`error` optional: the plugin writes it only for a row that failed under --apply, projects.go:512) |
| `project add [path] --name N --json` | internal/slices/setup/http.go:266-272 (`add`, `entry` struct) | `name` (non-empty) | name, id, folders, repos, keys (all always written, projects.go:46-58) |
| `project remove <p> --json` | internal/slices/setup/remove_project.go:91 | none; only the exit code | name, id, folders, repos, keys (what the plugin always writes) |
| `project-md set <p> <k> <v> --json` | internal/slices/projectsettings/project_settings_write.go:105 | none; only the exit code | project, key, value |
| `project repo add\|edit\|remove --json` | internal/slices/projectsettings/project_repos_write.go:96-106 (`reply`) | `state` (empty or `applied`), `item` (raw, `null` read as none) | state (enum `applied`); item optional, requires `id` |

Every plugin field the app reads is written by the plugin: no NOT DONE row for a missing field. Writers: runSetup internal/settings/claude.go:306 (machine form) and internal/cli/settings.go:257 (project form), runProject projects.go:143 and :172, projectRepo project_repo.go:110, runProjectMD projectmd.go:41.

Two shape decisions, stated plainly:
- `setup` is one route with two forms. `cmd-setup.json` is a `oneOf`: the Claude settings result the app reads, and `grid setup <project> --answers <file> --json` (`{project, answers}`, tested too, fixture `answers.json`).
- `project repo` `item` allows keys beyond id, where, remote and branch: the plugin keeps every key of an existing repos.json item on edit (internal/projects/repos.go:18-21), so `additionalProperties: false` there would reject real output.

## Proof table

| # | Row | Result |
|---|---|---|
| 1 | Red then green | HOLDS. Red on base: with `contract/schemas` stashed (`git stash push -u -- contract/schemas`), `go test -count=1 -v -run 'TestYn' ./internal/cli/` exit 1: `yn_setup_project_schema_test.go:160: cmd-setup rejects the real output: unknown schema "cmd-setup"`, `:211: cmd-project-scan rejects the real output: unknown schema "cmd-project-scan"`, `:260 ... "cmd-project-add"`, `:287 ... "cmd-project-repo"`, `:306 ... "cmd-project-md-set"`, `:319 ... "cmd-project-remove"`, six `--- FAIL`. Green on the lane head, same command, exit 0: `--- PASS: TestYnSetup`, `TestYnProjectScan`, `TestYnProjectAdd`, `TestYnProjectRepo`, `TestYnProjectMDSet`, `TestYnProjectRemove`. Each runs the real writer with --json in a temp home (HOME, CLAUDE_CONFIG_DIR and GRID_HOME under `t.TempDir()`), setup with --yes, scan over a root holding one registered and one new checkout (the test asserts registered true and false), and each stdout passes `contract.Validate`. |
| 2 | Required holds the app's fields | HOLDS. In the same six tests `ynSpRequire` renames every key of the "Fields read" column in a copy of the stdout (scan `registered` and `name`, add `name`, and the rest) and fails if Validate still accepts it; `--- PASS:` lines as in row 1. |
| 3 | Fixtures, unknown fields, inventory | HOLDS. `go test -count=1 -v -run 'TestFixtures\|TestContractCommandsRejectUnknownFields\|TestContractCommandsParserInventory' ./internal/contract/ ./internal/cli/` exit 0: `--- PASS: TestFixtures`, `--- PASS: TestContractCommandsRejectUnknownFields` (subtests for all six new schemas pass), `--- PASS: TestContractCommandsParserInventory`. |
| 4 | No product change | HOLDS. Files changed: six schemas, their `contract/fixtures/<schema>/{valid,invalid}/` files, the `output:` values in contract_inventory_test.go, contract/commands.json, internal/cli/yn_setup_project_schema_test.go, this doc. `git diff main -- contract/commands.json` is six lines `"output": ""` to `"output": "cmd-….json"` (project add :4690, remove :4734, repo :4770, scan :4790, project-md set :4830, setup :4964). No production file. Fixtures are written from the real output by `UPDATE_YN_FIXTURES=1` with the temp folder, ids and hashes fixed; they hold no personal path. |
| 5 | Tests, lint, cross | NOT DONE for one test; the rest holds. `go test -count=1 ./internal/cli/` ok (218s). `go test -count=1 ./internal/contract/` FAILS: `iy_schema_rows_test.go:47: schema cmd-project-add.json has no row in the contract docs`, and the same for cmd-project-md-set, cmd-project-remove, cmd-project-repo, cmd-project-scan, cmd-setup. `TestIySchemaRows` wants each schema linked as `](schemas/<name>)` in contract/README.md (iy_schema_rows_test.go:29, :37); that file is not in Touches and row 4 forbids it, so the lane does not edit it. The fix: six rows in contract/README.md next to line 183, in the form `\| grid setup output \| [cmd-setup.json](schemas/cmd-setup.json) \| internal/cli/settings.go:257 \|`, for cmd-setup, cmd-project-scan (projects.go:143), cmd-project-add and cmd-project-remove (projects.go:172), cmd-project-repo (project_repo.go:110), cmd-project-md-set (projectmd.go:41). Any lane that adds a cmd- schema meets the same test. `make lint` exit 0 (`go vet ./...`). `make cross` exit 0 (windows/amd64 vet and test compile, darwin). |
| R1 | Importers tested | HOLDS. The changed packages are `contract` (embedded data) and `internal/cli` (test files). Direct importers of `contract` (`go list -f '{{.ImportPath}} {{.Imports}} {{.TestImports}} {{.XTestImports}}' ./...`, filtered): contract, internal/braincmd, internal/cli, internal/contract, internal/hook, internal/job/lint, internal/packaging. `go test -count=1` on those plus ./contract/... ./internal/packaging/... ./cmd/grid/: all ok except internal/contract (row 5). |
| R2 | Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok 0.454s. `go test -count=1 ./internal/job/lint/` ok 0.925s (in the importer run). |
| R3 | No guard, gate, cap or hold on agents | HOLDS. Nothing in the lane touches agent behaviour: schemas, fixtures, one test file, annotations. |
| R4 | No *RealPlugin* row | HOLDS. The brief names none; the tests call `Run` in process. |
| R5 | Commits | HOLDS. Three commits (schemas and fixtures with annotations, the test file, this doc), one-sentence messages, no attribution lines. |

## What changed

- `contract/schemas/cmd-setup.json`, `cmd-project-scan.json`, `cmd-project-add.json`, `cmd-project-remove.json`, `cmd-project-repo.json`, `cmd-project-md-set.json`.
- Fixtures under `contract/fixtures/<schema>/`: at least two valid and two invalid each (scan also `failed.json` with the optional `error`; setup also `answers.json`; repo also `edited.json`).
- `output:` on the six lines of contract_inventory_test.go; `contract/commands.json` regenerated by `make generate`.
- `internal/cli/yn_setup_project_schema_test.go`: `TestYnSetup`, `TestYnProjectScan`, `TestYnProjectAdd`, `TestYnProjectRepo`, `TestYnProjectMDSet`, `TestYnProjectRemove`, helpers `ynSp*`.

## NOT VERIFIED on a live screen

Nothing in this lane has a screen. The app's own tests against these schemas are not run here (cross-repo set is the orchestrator's).

## Open items

- `contract/README.md`: six rows (row 5). Add the file to Touches, or apply the rows at landing.
- `project list`, `project relink`, `project-md get` still have no `output:`; not in this brief.
