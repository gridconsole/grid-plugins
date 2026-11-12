# C-yg-lease-where-schemas

Output schemas for `grid lease status task --json` and `grid where --json`.

Verdict: NOT DONE. Rows 1 to 4 hold. Row 5 holds except one test, `TestIySchemaRows` in `./internal/contract/`: the two new schemas need a link each in `contract/README.md`, which is outside the brief's Touches and which row 4 forbids. The fix is two lines, below under Open items.

Base: plugin main `103b915799ce05bb20c8d92fbf823650b37c5c25`. Code head: `9ecfcb05` (test commit `f3de5028`, then schemas `9ecfcb05`); this report is committed after it.

## Base check

- `internal/cli/task_lease.go:145-149` writes `struct{ lease.Tenure; Process string "json:\"process\"" }{tenure, state}`; `:123` builds `lease.Tenure{Lease: feed.Lease{Lease: "task"}}` for a task never leased.
- `internal/cli/cli.go:402-404` writes `writeJSON(env.Stdout, res)`, a `where.Result` (`internal/where/where.go:68-78`).
- Neither had an `output:`: `contract_inventory_test.go` on base `"where": {}` (:427) and `"lease status": {args: []string{"task"}}` (:491). Not stopped.

## Choices

- **Lease: fields written out, not `$ref`.** The validator checks a `$ref` target against the whole value first (`internal/contract/schema.go:191-199`), and each `oneOf` branch of `lease.json` has `additionalProperties: false`, so `process` would fail as an unknown property. `cmd-lease-status.json` therefore lists the nine fields itself, and `TestYgLeaseStatusFieldsMatchLeaseJSON` fails when its property names or required list differ from the task branch of `lease.json` plus `process`, or when any property other than `fence` and `since` differs from `lease.json`'s. `lease.json` is unchanged.
- **`fence` and `since` are looser than `lease.json`.** A task never leased answers `fence: 0` and `since: ""` (task_lease.go:123, seen in the red run below); `lease.json` requires fence 1 or more and a date-time. While `holder` is a string, an `allOf` if/then requires fence 1 or more, pid, pid_start and a non-empty since, as `lease.json` does.
- **`epoch` is not in the schema.** The app's `leaseEpoch` (app `8f9b806d`, `internal/kernel/worker/epoch.go:34-55`) takes an `epoch` string only when present and otherwise needs gen and fence, which the plugin writes. The app's own test says the same (`fw_worker_reads_realplugin_unix_test.go:152`: "lease status task has no `epoch` key; leaseEpoch takes gen.fence (epoch.go, by design)"). So no field the app needs is missing, and no plugin output changed.
- **where:** nine fields, all always written (no omitempty): project, id, rule (the seven `where.Rule` values), folder, record, repos, keys, task, task_path. A task or project without a value writes an empty string, so those stay `type: string`.

## Proof table

| Row | Result | Command and output |
|---|---|---|
| 1 red on base | HOLDS | Test file alone at `f3de5028`: `go test -count=1 -v -run 'TestYg' ./internal/cli/` exit 1: `yg_lease_where_schema_test.go:120: cmd-lease-status rejects stdout: unknown schema "cmd-lease-status"` with stdout `{"lease":"task","holder":null,"fence":0,"gen":0,"attempt":null,"since":"","pid":null,"pid_start":null,"process":"free"}`; `yg_lease_where_schema_test.go:250: cmd-where rejects stdout: unknown schema "cmd-where"`. Every case failed: `--- FAIL: TestYgLeaseStatus`, `/free`, `/held`, `/app_fields`, `/every_field`, `/process`, `/released`, `TestYgLeaseStatusFieldsMatchLeaseJSON`, `--- FAIL: TestYgWhere`, `/task`, `/no_task`, `/no_project`, `/app_fields`, `/every_field`, `/rules`. |
| 1 green | HOLDS | `go test -count=1 -v -run 'TestYg' ./internal/cli/` exit 0 (`ok  .../internal/cli 2.753s`): `--- PASS: TestYgLeaseStatus/free` (a task never leased), `/held` (taken with `lease take task --hold --json`), `/released`; `--- PASS: TestYgWhere/task` (`where --json --task <id>`), `/no_task` (`where --json` in the project folder), `/no_project`. Each stdout passes `contract.Validate` (`ygLwValid`). |
| 2 required holds the app's fields | HOLDS | Same run: `--- PASS: TestYgLeaseStatus/app_fields/rename_holder`, `/rename_gen`, `/rename_fence`; `--- PASS: TestYgWhere/app_fields/rename_folder`, `/rename_task_path`. Each subtest first checks the unrenamed copy validates, so it fails on base (it did). `/every_field/rename_*` passes for all nine lease fields and all nine where fields. |
| 3 fixtures, unknown fields, inventory | HOLDS | `go test -count=1 -v -run 'TestFixtures\|TestContractCommandsRejectUnknownFields\|TestContractCommandsParserInventory' ./internal/contract/ ./internal/cli/` exit 0: `--- PASS: TestFixtures` (cmd-lease-status 3 valid and 3 invalid, cmd-where 2 and 3), `--- PASS: TestContractCommandsRejectUnknownFields` with `/cmd-lease-status` and `/cmd-where`, `--- PASS: TestContractCommandsParserInventory`. |
| 4 no product change | HOLDS | `git diff main --stat`: 16 files, all in Touches (commands.json, 11 fixtures, 2 schemas, contract_inventory_test.go, yg_lease_where_schema_test.go), plus this report. `git diff main -- contract/commands.json`: two hunks, `"output": ""` to `"output": "cmd-where.json"` (:75) and to `"output": "cmd-lease-status.json"` (:1729), nothing else. No production file. `make generate` changed no file besides commands.json (`git status --short` after it). |
| 5 cli and contract tests | NOT DONE for one test | `go test -count=1 ./internal/cli/` `ok` (294s). `go test -count=1 ./internal/contract/` exit 1: `--- FAIL: TestIySchemaRows`: `schema cmd-lease-status.json has no row in the contract docs`, `schema cmd-where.json has no row in the contract docs`. Every other test in the package passes. The test is `internal/contract/iy_schema_rows_test.go:36`; it needs `](schemas/<name>)` in `contract/README.md`. |
| 5 lint | HOLDS | `make lint` exit 0 (`go vet ./...`, `gofmt -l .` empty). |
| 5 cross | HOLDS | `make cross` exit 0: `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile`, no FAIL. |
| Importers | HOLDS except the one test | `go list` of importers of `internal/cli`, `contract` and `internal/contract` (29 packages): cmd/grid, artifact, brain, braincmd, cli, contract, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, question, session, settings, signal, task, updates, usage, where, workflow, lib. `go test -count=1` on all: `ok` for each except `internal/contract` (the one test above); cmd/grid and updates have no test files. |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` `ok`; `go test -count=1 ./internal/job/lint/` `ok`. The app walk tests are not this repo's. |
| RealPlugin tests | not named by a row | None run; no test of this lane needs the built plugin. |
| Attribution, paths | HOLDS | Commit messages carry no attribution lines. Fixtures use `/work/example` and `/grid/...`, no host paths, hosts or emails. |

## What changed

- `contract/schemas/cmd-lease-status.json`, `cmd-where.json` (new).
- `contract/fixtures/cmd-lease-status/{valid,invalid}/` (free, held, released; holder renamed, process missing, held without pid) and `contract/fixtures/cmd-where/{valid,invalid}/` (task, no-task; folder renamed, task_path missing, unknown rule).
- `internal/cli/contract_inventory_test.go`: the two `output:` values; `contract/commands.json` regenerated by `make generate`.
- `internal/cli/yg_lease_where_schema_test.go` (new): `TestYgLeaseStatus`, `TestYgLeaseStatusFieldsMatchLeaseJSON`, `TestYgWhere`; helpers `ygLw*`.

## Open items

1. **NOT DONE: `TestIySchemaRows`.** Add these two rows to the cmd- table of `contract/README.md` (next to the rows at :185-198), and add `contract/README.md` to the Touches of this brief:
   `| grid lease status output | [cmd-lease-status.json](schemas/cmd-lease-status.json) | internal/cli/task_lease.go:145 |`
   `| grid where output | [cmd-where.json](schemas/cmd-where.json) | internal/cli/cli.go:403 |`
   I did not edit the file: it is not in Touches and row 4 names the Touches list. The test reads only for the `](schemas/<name>)` text (iy_schema_rows_test.go:29), so those two lines are enough; I did not run it with them. The other lanes that add a `cmd-` schema (C-yh, C-yi, C-yj to C-yn) meet the same test.
2. The app is not touched. Its read of these two outputs is shown only through its source lines above; the cross-repo set is the orchestrator's.

## NOT VERIFIED on a live screen

Nothing here is a screen. No live check needed.

NOT DONE
