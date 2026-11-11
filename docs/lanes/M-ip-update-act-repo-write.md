# M-ip-update-act-repo-write

## What conflicted and how it was resolved

Merging origin/lane-C-gr-plugin-update-act (stacked start; main origin/main ee8a96c1 was already in the branch) conflicted in one hunk of `internal/cli/contract_inventory_test.go` (about line 556). Main added the `"project repo"` row, the lane added five `"update ..."` rows, both after `"project relink"`. Resolution: keep both, in the order the brief gives (`project repo`, then `update draft|edit|discard|skip|send`, then main's `signal add`). No other file conflicted. `make generate` changed nothing. A second `git merge origin/main` printed "Already up to date."

## Proof table

| # | Row | Result |
|---|---|---|
| 1 | Ancestors of HEAD | HOLDS: `git merge-base --is-ancestor` exit 0 for origin/main, origin/lane-C-gr-plugin-update-act and origin/lane-C-gr-plugin-project-repo-write |
| 2 | Annotations once, no markers | HOLDS: `git grep` prints `contract_inventory_test.go:556 "project repo"`, `:557 "update draft"`, `:561 "update send"`; `git grep -n '^<<<<<<<' -- internal contract testdata` prints nothing |
| 3 | Generated contract current | HOLDS: `make generate` exit 0, `git status --short` empty |
| 4 | Lane rows | HOLDS: `go test -count=1 -v -run 'TestGrUpdateAct\|TestGrProjectRepo' ./internal/cli/` exit 0; PASS for TestGrUpdateActDraftWritesTheFile, ...FormatParity, ...EditSkipSendDiscard, ...Refusals, TestGrProjectRepoAddWritesTheItem, ...EditKeepsOtherKeys, ...RemoveTakesOnlyThatItem, ...RefusalsLeaveTheFileAlone, ...GuestIsRefused (guest, grid-guest), ...UnknownProjectCode, ...FindsTheProjectById; no SKIP |
| 5 | Shared walks | HOLDS: `go test -count=1 -run 'TestContractCommandsParserInventory\|TestGgUnknown\|TestGuestProjectParity\|TestProject' ./internal/cli/` exit 0 (ok 12.9s) |
| 6 | Touched packages | HOLDS: `go test -count=1 ./internal/cli/ ./internal/updates/ ./internal/projects/` exit 0 (updates has no test files); `go vet` on the three exit 0; `gofmt -l internal/cli` empty |
| R1 | Importers tested | HOLDS: importers of cli, updates and projects are cmd/grid, lib, internal/job/jobcmd, internal/signal; `go test -count=1` on them exit 0 (cmd/grid has no test files) |
| R2 | Windows and macOS builds | HOLDS: `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0 |
| R3 | Lint | HOLDS: go vet and gofmt as in row 6 |
| R4 | No guards added | HOLDS: only a conflict resolution in a test annotation map; no production change |
| R5 | New tests fail on base | NOT APPLICABLE: no new tests; both lanes' own tests are unchanged |
| R6 | No attribution lines, no personal paths | HOLDS: merge commit has one sentence and no trailer |
| R7 | RealPlugin tests | NOT APPLICABLE: no row names one |

Guest parity walk: both verbs remain walked and refused for a guest (TestGrProjectRepoGuestIsRefused passes; TestGrUpdateActRefusals and TestGuestProjectParity pass on the merged tree).

## What changed

`internal/cli/contract_inventory_test.go` (conflict resolution) and this file. Nothing else.

## NOT VERIFIED on a live screen

None; no UI. The whole-module test and cross-repo set are left to the orchestrator at landing. Put C-gr-plugin-update-act on landing-excludes.txt once this lane is accepted.
