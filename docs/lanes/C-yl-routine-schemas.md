# C-yl-routine-schemas: output schemas for `grid routine list` and `routine add` with --json

Base: plugin main 103b915799ce05bb20c8d92fbf823650b37c5c25. Verdict: NOT DONE (one row, below).

Base check: `internal/cli/routine.go:114-121` (list: `entries, err := store.List()` then `writeJSON(env.Stdout, entries)` at :120) and `:150-158` (add: `store.AddWith(...)` then `writeJSON(env.Stdout, entry)` at :158). Neither `routine add` nor `routine list` had an `output:` annotation (`internal/cli/contract_inventory_test.go:529-530` on base), so the lane went ahead.

## Proof table

| # | Row | Result | Command / place |
|---|-----|--------|-----------------|
| 1 | Red on base | HOLDS | With the schemas, fixtures and annotations stashed (`git stash push -u -- contract internal/cli/contract_inventory_test.go`), `go test -count=1 -v -run 'TestYl' ./internal/cli/` exits 1: `yl_routine_schema_test.go:106: cmd-routine-list: unknown schema "cmd-routine-list": []` and `--- FAIL: TestYlRoutineListAddSchemas`; the other two TestYl tests fail on the missing fixture and schema file. |
| 1 | Green on lane | HOLDS | Same command, exit 0: `--- PASS: TestYlRoutineListAddSchemas`, `--- PASS: TestYlRoutineSchemasKeepAppFields`, `--- PASS: TestYlRoutineListAndAddDescribeOneEntry`. TestYlRoutineListAddSchemas runs as the app does (`--project alpha`): list on an empty project prints `[]`; add `morning` (--run); add `nightly` (--workflow, three limit flags); list; a last run written to the routine file; add `--replace` (keeps last_run, outcome, last_task); list again. Every stdout passes `contract.Validate`, and each of the four checked stdouts (`[]`, add morning, add nightly, list after) also equals its valid fixture (machine id masked), so the fixtures are output the plugin writes. |
| 2 | Required holds the app's fields | HOLDS | `TestYlRoutineSchemasKeepAppFields` (`--- PASS`, inside the TestYl run above): for each of name, every, run, machine, limits, scheduled, next, renaming it and dropping it in a copy of a list entry makes `Validate` fail, for cmd-routine-list and cmd-routine-add; a wrong type (every, scheduled, next, last_run, outcome, limits) fails; a top-level object fails for list and a top-level array fails for add; `limits` with a missing or extra key fails. |
| 3 | Three contract tests | HOLDS | `go test -count=1 -v -run 'TestFixtures\|TestContractCommandsRejectUnknownFields\|TestContractCommandsParserInventory' ./internal/contract/ ./internal/cli/` exit 0: `--- PASS: TestFixtures (1.57s)` (cmd-routine-add and cmd-routine-list included, 3 valid and 3 invalid fixtures each, list valid has 3), `--- PASS: TestContractCommandsRejectUnknownFields (0.14s)` with both new schemas, `--- PASS: TestContractCommandsParserInventory (4.25s)`. |
| 4 | No product change | HOLDS | `git diff main --stat` before the report: 17 files, all in Touches (commands.json, 12 fixtures, 2 schemas, contract_inventory_test.go, yl_routine_schema_test.go), plus the runbook's `docs/lanes/C-yl-routine-schemas.md` in the final commit. No production file. `git diff main -- contract/commands.json` is two lines: `"output": ""` becomes `"output": "cmd-routine-add.json"` (routine add) and `"cmd-routine-list.json"` (routine list). `make generate` changed no other file. |
| 5 | `go test ./internal/cli/` | HOLDS | `go test -count=1 ./cmd/... ./internal/... ./lib/...`: `ok .../internal/cli 276.753s`. |
| 5 | `go test ./internal/contract/` | NOT DONE | Fails: `--- FAIL: TestIySchemaRows`: `iy_schema_rows_test.go:47: schema cmd-routine-add.json has no row in the contract docs` and the same for `cmd-routine-list.json`. The test wants each schema linked as `](schemas/<name>)` from `contract/README.md` (`internal/contract/iy_schema_rows_test.go:34-38`, rows like `contract/README.md:199`). `contract/README.md` is not in this brief's Touches and row 4 forbids other files, so it is not edited. The two rows to add are in `/srv/gridwork/tmp/hand/C-yl-routine-schemas-readme-rows.txt`. NOT VERIFIED that the test is green with them (not run: that needs the edit). |
| 5 | `make lint` | HOLDS | exit 0 (`go vet ./...`, `gofmt -l .` empty). |
| 5 | `make cross` | HOLDS | exit 0; output has `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`, no FAIL. |
| rule | Importers' tests | HOLDS | `go list` importers of `internal/cli` or `internal/contract` (direct): cmd/grid, artifact, brain, braincmd, cli, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, question, session, settings, signal, task, updates, usage, where, workflow, lib. All ran in `go test -count=1 ./cmd/... ./internal/... ./lib/...` and are `ok`; the only FAIL in the whole run is TestIySchemaRows above. |
| rule | Plugin walk tests | HOLDS | `go test -count=1 -v ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `--- PASS: TestHsIpWalksEveryCommand` (walked 39 commands, 39 table entries), `--- PASS: TestIlGuestSiteList`; `go test -count=1 ./internal/job/lint/`: `ok 1.136s`. The app walk tests (make webui-test, TestFvArgvCollects, ...) belong to the app repo and do not apply to this plugin lane. |
| rule | *RealPlugin* tests | HOLDS (none) | No test of this lane is named *RealPlugin*; none was needed. |
| rule | New tests fail on base, pass on lane | HOLDS | Row 1 above, both runs pasted. |
| rule | No existing test edited or deleted | HOLDS | The only edit to an existing file is the two `output:` values in `internal/cli/contract_inventory_test.go:529-530`, as the brief says; no test deleted; `names` of TestContractCommandsApp untouched. |
| rule | No guard, gate, cap or hold on agents | HOLDS | None added; schemas and tests only. |
| rule | Public repo hygiene | HOLDS | No personal path, host or email in the files; the fixtures use the sample ULID `01ARZ3NDEKTSV4RRFFQ69G5FAW`. Commits carry no attribution lines. |

## What changed

- `contract/schemas/cmd-routine-list.json`: top-level array of entries. `contract/schemas/cmd-routine-add.json`: one entry. Entry = what `routine.Entry` writes as JSON (`internal/routine/store.go:18-38`): required name, every, run (empty for a workflow routine), machine, limits (routine_timeout, routine_max_turns, routine_budget, all required), scheduled, next (date-time string or null); optional workflow, last_run (date-time), outcome (running, success, failed), last_task. Every object has `additionalProperties: false`.
- `contract/fixtures/cmd-routine-list/` and `cmd-routine-add/`: valid and invalid fixtures. The populated list holds two routines, one with a last run: the harness writes it into the routine file through `brain.Store.EditConfig`, as `routine_test.go:36-41` does; `routine run`, `schedule` and `unschedule` are not called.
- `internal/cli/contract_inventory_test.go:529-530`: `output:` values; `contract/commands.json` regenerated by `make generate`.
- `internal/cli/yl_routine_schema_test.go`: `TestYlRoutineListAddSchemas`, `TestYlRoutineSchemasKeepAppFields`, `TestYlRoutineListAndAddDescribeOneEntry` (the two schemas describe one entry; a field added to one only fails). Fixture refresh: `UPDATE_CONTRACT_COMMAND_FIXTURES=1`, as the other cmd- fixtures.

## NOT VERIFIED

- `contract/fixtures/cmd-routine-list/valid/scheduled.json` (scheduled true, `next` a date-time string, outcome running) is hand-written from `internal/routine/store.go:115-135`: producing it needs the scheduler, which the brief leaves out. The string branch of `next` is therefore pinned by a fixture only, not by captured output.
- The app's decode (`internal/slices/routines/http.go:246-258`) is in the app repo, not in this clone; the field list (name, every, run, last_run, outcome, scheduled, next, last task) is taken from the brief. A field rename in the plugin now fails the plugin's tests; the app's side was not read.
- No live screen (Routines page).

## Open items

- Add the two README rows (file above) at landing, or re-brief the lane with `contract/README.md` in Touches; then `go test -count=1 ./internal/contract/` should be green.
- The other C-yj to C-yn lanes that add a `cmd-` schema will meet the same TestIySchemaRows failure.
