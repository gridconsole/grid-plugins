# C-yg2-schema-readme-rows

Finishes C-yg-lease-where-schemas and carries C-yn-setup-project-schemas: one `contract/README.md` row for each of the eight new `cmd-` output schemas, so `TestIySchemaRows` passes.

Verdict: DONE.

## Merges

- `git merge origin/main`: clean, no conflicts, no file resolved.
- `git merge origin/lane-C-yn-setup-project-schemas`: clean, no conflicts, no file resolved. The lane now carries both C-yg and C-yn.

## The failure, before the fix

`go test -count=1 ./internal/contract/` on the merged lane (before the rows) exit 1:

```
--- FAIL: TestIySchemaRows (0.00s)
    iy_schema_rows_test.go:47: schema cmd-lease-status.json has no row in the contract docs
    iy_schema_rows_test.go:47: schema cmd-project-add.json has no row in the contract docs
    iy_schema_rows_test.go:47: schema cmd-project-md-set.json has no row in the contract docs
    iy_schema_rows_test.go:47: schema cmd-project-remove.json has no row in the contract docs
    iy_schema_rows_test.go:47: schema cmd-project-repo.json has no row in the contract docs
    iy_schema_rows_test.go:47: schema cmd-project-scan.json has no row in the contract docs
    iy_schema_rows_test.go:47: schema cmd-setup.json has no row in the contract docs
    iy_schema_rows_test.go:47: schema cmd-where.json has no row in the contract docs
FAIL	github.com/gridconsole/grid-plugins/internal/contract
```

## Proof table

| Row | Result | Command and output |
|---|---|---|
| README rows, `TestIySchemaRows` | HOLDS | Eight rows in the cmd- table of `contract/README.md`, alphabetical, in the form of the rows there (commit `21a4cda8`): lease status (`internal/cli/task_lease.go:146`), project add (`projects.go:172`), project-md set (`projectmd.go:41`), project remove (`projects.go:172`), project repo (`project_repo.go:110`), project scan (`projects.go:143`), setup (`settings.go:257`; `internal/settings/claude.go:306`), where (`cli.go:407`). Each cited line is a `writeJSON` call, checked with grep on the merged tree; the lease and where lines are 146 and 407, not the 145 and 403 the C-yg report gave, because main moved them. `go test -count=1 ./internal/contract/` exit 0: `ok .../internal/contract`. |
| C-yg row 1 and 2 (TestYg) | HOLDS | `go test -count=1 -v -run 'TestYg\|TestYn' ./internal/cli/` exit 0: `--- PASS: TestYgLeaseStatus`, `TestYgLeaseStatusFieldsMatchLeaseJSON`, `TestYgWhere`. The red-on-base lines are in docs/lanes/C-yg-lease-where-schemas.md row 1 (`unknown schema "cmd-lease-status"`); nothing there changed. |
| C-yn rows 1 and 2 (TestYn) | HOLDS | Same run: `--- PASS: TestYnSetup`, `TestYnProjectScan`, `TestYnProjectAdd`, `TestYnProjectRepo`, `TestYnProjectMDSet`, `TestYnProjectRemove`. |
| C-yg and C-yn row 3 | HOLDS | `go test -count=1 -v -run 'TestFixtures\|TestContractCommandsRejectUnknownFields\|TestContractCommandsParserInventory' ./internal/contract/ ./internal/cli/` exit 0: `--- PASS: TestFixtures`, `--- PASS: TestContractCommandsRejectUnknownFields`, `--- PASS: TestContractCommandsParserInventory`. |
| C-yg and C-yn row 4, no product change | HOLDS | `git diff origin/main --stat -- . ':!docs'`: only `contract/schemas/cmd-*.json` (8), `contract/fixtures/cmd-*/` , `contract/commands.json`, `internal/cli/contract_inventory_test.go`, `internal/cli/yg_lease_where_schema_test.go`, `internal/cli/yn_setup_project_schema_test.go`, plus `contract/README.md`. No production file. `git diff origin/main -- contract/commands.json` has eight added `"output": "cmd-....json"` lines and eight removed `"output": ""` lines, nothing else. `make generate` then `git status --short` empty. |
| C-yg and C-yn row 5 tests | HOLDS | `go test -count=1 ./internal/cli/ ./internal/contract/` exit 0: `ok .../internal/cli 193.933s`, `ok .../internal/contract 2.507s`. |
| `make lint` | HOLDS | `make lint` exit 0 (`go vet ./...`); `gofmt -l .` printed nothing. |
| Windows and macOS builds | HOLDS | `make cross` exit 0: `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile`, every package `ok`. |
| Importers | HOLDS | `go list` of packages importing `internal/cli`, `contract` or `internal/contract` (31): cmd/grid, contract, contract/formats, artifact, brain, braincmd, cli, internal/contract, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, question, session, settings, signal, task, updates, usage, workflow, lib, where. `go test -count=1` on all (cli and contract above): every line `ok` or `no test files`, no FAIL. |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 `ok`; `go test -count=1 ./internal/job/lint/` exit 0 `ok`. The app walk tests are not this repo's. |
| RealPlugin tests | not named by a row | None run; no row here needs the built plugin. |
| No guard, gate, cap or hold | HOLDS | Only a docs table and a lane report changed. |
| Attribution, paths | HOLDS | Commit messages carry no attribution lines. The README rows hold repo-relative paths only. |
| C-yg report NOT DONE rows rewritten | HOLDS | `docs/lanes/C-yg-lease-where-schemas.md`: the Verdict line, row 5, the Importers row, open item 1 and the final line now read CLOSED BY C-yg2-schema-readme-rows. No other text changed. |

## What changed

- `contract/README.md`: eight rows.
- `docs/lanes/C-yg-lease-where-schemas.md`: NOT DONE rows rewritten.
- `docs/lanes/C-yg2-schema-readme-rows.md`: this report.
- Everything else is the two merges (main, C-yn). No test was edited, deleted or skipped.

## Open items

- `docs/lanes/C-yn-setup-project-schemas.md` row 5 still says NOT DONE for the same test. The brief named only the C-yg report, so I left it. The test it names now passes on this lane.

## NOT VERIFIED on a live screen

Nothing here is a screen.

DONE
