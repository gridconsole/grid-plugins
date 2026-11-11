# C-lt-readme-update-group

README's `## Commands` list now names `grid update`, and a new test checks every command in contract/commands.json is named there.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1 Red on base | HOLDS | `go test -count=1 -run TestLtReadme -v ./internal/cli/` with README unchanged, exit 1: `lt_readme_groups_test.go:47: README.md ## Commands does not name `grid update`` and `--- FAIL: TestLtReadmeNamesEveryCommand (0.00s)`; no other command named |
| 2 Walks whole set | HOLDS | test ranges over all of `contractdata.Commands` (internal/cli/lt_readme_groups_test.go:41). With `grid eval` temporarily removed from README.md:39, exit 1: `README.md ## Commands does not name `grid eval``. Edit reverted. |
| 3 README line | HOLDS | README.md:39 now has `grid update` with "project updates"; `git diff main -- README.md` is that one line |
| 4 Pass | HOLDS | `go test -count=1 -run 'TestLtReadme|TestGbDocs' -v ./internal/cli/` exit 0: `--- PASS: TestGbDocsVerbsAreImplemented`, `--- PASS: TestGbDocsSchemaLinksExist`, `--- PASS: TestLtReadmeNamesEveryCommand`, no FAIL |
| 5 Scope, lint, cross | HOLDS | diff names README.md, the new test and this report; `make lint` exit 0; `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| Packages changed + importers | HOLDS | `go test -count=1 ./internal/cli/` ok (38s); importers by go list: cmd/grid, lib: `go test ./lib ./cmd/grid` ok |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| RealPlugin tests | n/a | no row names one |
| Guards on agents | HOLDS | none added |

## What changed
- README.md:39: added `grid update`.
- internal/cli/lt_readme_groups_test.go: new `TestLtReadmeNamesEveryCommand`.

## NOT VERIFIED on a live screen
Nothing here is UI.
