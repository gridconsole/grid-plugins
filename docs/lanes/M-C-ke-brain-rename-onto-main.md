# M-C-ke-brain-rename-onto-main

Carries lane C-ke-brain-rename onto plugin main 21f02736.

## What changed

- `git merge origin/main` into the lane: clean, no conflict, no file edited by hand in the merge.
- Orchestrator note 11 Nov 19:16: `TestHeMvMessagesNameRealSubs` failed because `rename` was not a sub of `brain` in `contract/commands.json`. That file is generated from the parsers (`make generate`), so `rename` had to become a parser route first. Three small commits, all on top of the merge:
  - `internal/braincmd/command.go`: `rename` added to the usage verb list, one usage entry, and a dispatch to `runRename` (the lane's own verb). The brief of C-ke-brain-rename left this to WAIT-C-ke-mount; the orchestrator note makes it necessary here.
  - `internal/cli/cli.go` (help summary), `testdata/golden/help.golden`, `testdata/golden/brain-help.golden`, `internal/cli/contract_inventory_test.go` (one new table entry `brain rename`), `internal/cli/guest_parity_test.go` (one new table entry and one new `case "brain rename"`, same shape as drop). These are the walk tests that read the verb list; each is an added entry, no existing entry or test edited or removed.
  - `contract/commands.json`: regenerated with `make generate` (exit 0), 38 lines changed: the `rename` sub and the brain summary.
- Heads-up: the four tests in `internal/cli` that failed after wiring (`TestExHelpSummariesNameEveryVerb`, `TestGgUnknownProjectWalk`, `TestGuestProjectParity`, `TestRemainingTaskOperandHelp/brain`) now pass. The two goldens were edited by hand to match the printed help, because `UPDATE_GOLDEN=1` cannot be set from here; `TestRemainingTaskOperandHelp` compares them byte for byte and passes.

## Proof table

| Row | Result |
|---|---|
| 0. `go test -count=1 -run TestHeMvMessagesNameRealSubs ./internal/contract/` | HOLDS, exit 0 (failed before the wiring, with `"rename" is not a sub of grid brain in contract/commands.json` at ke_rename.go:13 and :22) |
| 1. `git merge-base --is-ancestor origin/main HEAD` | HOLDS, exit 0 (was 1 before the merge) |
| 1. `git diff origin/main --stat` names only lane files | HOLDS, with the three wiring additions above: the lane's four new files and its lane doc, plus command.go, cli.go, the two goldens, commands.json and two cli test tables (listed above) |
| 2. C-ke-brain-rename row 1: `go test -count=1 -v -run 'TestKeRnStore' ./internal/brain/` | HOLDS, exit 0, `--- PASS: TestKeRnStore` |
| 2. row 2: `TestKeRnRefuse` | HOLDS, exit 0, `--- PASS: TestKeRnRefuse` (13 subtests) |
| 2. row 3: `TestKeRnHalfWay` | HOLDS, exit 0, `--- PASS: TestKeRnHalfWay` |
| 2. row 4: `go test -count=1 -race -run TestKeRnRace ./internal/brain/` | HOLDS, exit 0, no race report; `--- PASS: TestKeRnRace` |
| 2. row 5: `go test -count=1 -v -run TestKeRnCommand ./internal/braincmd/` | HOLDS, exit 0, `--- PASS: TestKeRnCommand` (usage, roles_refused, failures, roles_allowed, json_result, origin) |
| 2. row 6: `git diff main --stat` names only the four new files | CHANGED BY DESIGN: the orchestrator note requires `rename` in commands.json, which needs the wiring files above. `make lint` and `make cross` below. |
| Package tests: `go test -count=1 ./internal/brain/ ./internal/braincmd/ ./internal/cli/ ./internal/contract/ ./lib/` | HOLDS, exit 0, all ok. Importers by `go list`: braincmd is imported by cli; cli by cmd/grid and lib. cmd/grid has no test files. |
| Walk: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0, ok |
| Walk: `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0, ok |
| Walk tests of the app (webui-test, TestFvArgvCollects, slices Imports) | not applicable: plugin repo |
| `make lint` | HOLDS, exit 0 |
| `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| No guard, gate, cap or hold added on agents | HOLDS: no such code added; the role check in `runRename` is the lane's own brief, same as drop |
| Tests that fail on base, pass on the lane | the lane's own `TestKeRn*` are new files (no base); `TestHeMvMessagesNameRealSubs` failed on the merged tree before the wiring (quoted in row 0) and passes now |
| Public repo hygiene | HOLDS: no personal paths, hosts or emails added |

## NOT VERIFIED on a live screen

Nothing here is UI. The CLI is exercised in process only; the mount of `grid brain rename` in the app (WAIT-C-ke-mount) is not part of this lane.
