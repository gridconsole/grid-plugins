# P-brain-readers-fix2

Shows the checks P-brain-readers-fix1 left unshown (review STUCK 07:04). Branch `lane-P-brain-readers-fix2`, from `lane-P-brain-readers-fix1` (cbb0c0bb), merged with origin/main (clean merge, no conflicts). V-number V13.

## What changed

Nothing in code. fix1's code and tests were already green; this lane re-ran every check on the merged head and pastes them below. The only new file is this doc. No fix was needed, so nothing outside fix1's Touches was edited.

Per the orchestrator ruling (07:18), no fail-on-base row is required and no second worktree was made.

## Rows

| Row | Result | Proof |
|---|---|---|
| 1a. fix1 row 1: walk test over the 2.2 reader table | HOLDS | `TestPbRdReaderTableWalk` in `internal/braincmd/pb_rd_walk_test.go:126`. `go test -count=1 -v -run 'TestPbRdReaderTableWalk\|TestPbRdSettingsGlobCoversRoot' ./internal/braincmd/ ./internal/settings/` exit 0: `--- PASS: TestPbRdReaderTableWalk (0.10s)`, `ok .../internal/braincmd 0.427s`. It parses the 2.2 table from the doc (fails under 25 files), fails on a table file with no entry, on a Go code line still building a `knowledge/` path, and on a missing new-layout marker |
| 1b. fix1 row 2: settings glob covers `<Root>/**` | HOLDS | `TestPbRdSettingsGlobCoversRoot` in `internal/settings/pb_rd_readers_test.go`, same run, exit 0: `--- PASS: TestPbRdSettingsGlobCoversRoot` with PASS for `brain_folder_under_the_user_home`, `brain_folder_outside_the_user_home`, `ClaudeBlock_carries_the_brain_rules`, `key_absent`, `key_naming_the_Grid_home_itself`; `ok .../internal/settings 0.019s` |
| 2. `go test` for every changed package and every importer | HOLDS | `go test -count=1 ./...` exit 0, every package `ok` (no FAIL line). Changed packages: braincmd 6.3s, guard 2.3s, job 3.0s, land 1.1s, orchestrator 267.8s, people 0.03s, projectenv 0.4s, routine 1.7s, settings 3.2s, workflow 7.2s. Importers (cli, hook, jobcmd, task, feed, mcp, runner, migrate, doctor, account, updates, lease, ports, jobgraph, lint, lib, cmd/grid) are in the same run, all ok. The whole module is a superset of the `go list` importer set, so no package is missing |
| 2a. `TestHsIpWalksEveryCommand`, `TestIlGuestSiteList` | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0, `ok .../internal/cli 0.128s` |
| 2b. `internal/job/lint` | HOLDS | `go test -count=1 ./internal/job/lint/` exit 0, `ok 0.385s` |
| 3a. `make lint` | HOLDS | exit 0 (`go vet ./...`, no output) |
| 3b. `make cross` | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package `ok` |
| 4. Fix needed to go green | HOLDS | none needed; nothing red |
| Merge of origin/main | HOLDS | `git fetch origin && git merge origin/main`: clean merge before the last proof run (the 76 files it brought are contract schemas, fixtures, and `P-loop-schemas`, which this lane does not touch) |
| `make webui-test`, guest walk tables, `make realplugin` | not run, not asked | plugin repo; the brief names none |
| Tests fail on base | not run, the brief forbids it | orchestrator ruling 07:18: no fail-on-base row, no second worktree |
| Whole-module test, cross-repo set | not run, not asked | the orchestrator runs them at landing (the module-wide `go test ./...` above was run anyway) |

## Outside Touches

None. This lane adds only `docs/lanes/P-brain-readers-fix2.md`. The files fix1 edited outside Touches (`agent-templates/grid-library.md`) stay as fix1's lane doc describes.

## Open items

Same as fix1's lane doc: reads prefer `grid/`, writes still go to `knowledge/` (P-brain-format's), and `braincmd/cb_author_origin.go` keeps `ValidName` for generated ids.

## NOT VERIFIED

Nothing needs a live screen.
