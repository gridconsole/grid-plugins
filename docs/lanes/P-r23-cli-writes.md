# P-r23-cli-writes: ship brief written atomically, a repeated person cancel saves once

Repo plugin, branch lane-P-r23-cli-writes. Findings: U-chk-plugin-rev-cli rows 9 and 12.

## Proof table

| # | Row | Result |
|---|---|---|
| 1a | Row 9 confirmed on main | CONFIRMED. `internal/cli/deliver_person_job.go:139` on main is `os.WriteFile(path, …)` after `Lstat` returns "exists, leave it" (:135-137). On main `TestR23ShipBriefIsWrittenAtomically` prints `deliver_person_job.go writes with os.WriteFile, which is not atomic` and `does not write the brief with atomicfile.Write`. |
| 1b | Row 12 confirmed on main | CONFIRMED. On main, a person (dana, role owner, not the lease holder) runs `grid job cancel` twice: both exit 0 and print two ids. `TestR23RepeatedPersonCancelSavesOnce`: `the repeated cancel printed "01M7GJEW9K0Q53ZY9X234RSZDD", want the first request's id "01M7GJEW40RPH74BF3TNDMA1NA"`. |
| 2a | Row 9 fixed, test | `TestR23ShipBriefIsWrittenAtomically` (FAIL on main as pasted in 1a, PASS on head); `TestR23ShipBriefLeavesNoTempFile` (PASS on both, guards the atomic write leaving no temp file and a whole brief). The first is a source walk (go/parser) because the crash cannot be staged in-process; it fails on any `os.WriteFile`/`os.Create` in the file. `go test -count=1 ./internal/cli -run TestR23` on head: `ok`. |
| 2b | Row 12 fixed, tests | `TestR23RepeatedPersonCancelSavesOnce`, `TestR23RepeatedPersonCancelJSONNamesTheFirst` (both FAIL on main: `the repeat does not print the first request 01M7GJEWMKQHTANY25600JB0VJ as pending: {"id":"01M7GJEWNJHY0JC0T3WCRFRFN5",…`), PASS on head. `TestR23AnotherPersonsCancelIsStillSaved` pins that a second person's cancel is still saved (PASS on both). |
| 3a | go test of changed packages and importers | HOLDS. `go test -count=1 ./internal/orchestrator/... ./internal/cli/... ./internal/mcp/... ./internal/migrate/... ./cmd/grid/... ./lib/...` exit 0: orchestrator ok 532.872s, cli ok 327.263s, mcp ok 1.837s, migrate ok 43.018s, cmd/grid no test files, lib ok 104.788s. Importers from `go list`: orchestrator is imported by cli, mcp, migrate; cli by cmd/grid and lib; atomicfile is unchanged. |
| 3b | make lint | HOLDS. `make lint` exit 0 (`go vet ./...`, no output). |
| 3c | Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` : `ok ... 0.664s`. `go test -count=1 ./internal/job/lint/` : `ok ... 0.527s`. |
| 4 | Merge of origin/main before the last run | HOLDS. `git fetch origin && git merge origin/main`: `Already up to date.` |
| 5 | Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| 6 | New tests fail on base, pass on lane | HOLDS (see 1a, 1b, 2a, 2b). No existing test edited or deleted. |
| 7 | Guard, gate, cap or hook on agents | None added. The repeat check is only for a person of the team's own `grid job cancel` request. |
| 8 | Real-plugin tests, app walk tests | not run, not asked |
| 9 | Attribution lines | none in the commits |

## What changed

- `internal/cli/deliver_person_job.go`: `saveShipBrief` writes `jobs/<id>/brief.md` with `atomicfile.Write` (temp file, fsync, rename), as `internal/proof/proof.go` and `internal/feed/fold.go` write. A crash or full disk leaves no part-written brief for the next delivery to keep.
- `internal/orchestrator/person_write.go`: `PostPersonUnless`, for a cancel request, looks under `.orchestrator.lock` for an open request of the same person for the same Job (`openCancelRequest`: an unleased cancel control by that author, not yet answered by a task-lease record whose `re` is its id, written after the Job was last reopened — the filter `personCancels` uses). When one exists nothing is written, `skipped` is true and the record returned is the earlier one. `grid job cancel` prints its id (or its JSON, `pending: true`) and exits 0. Another person's request is still saved. The doc comment of `PostPersonUnless` says so.
- `internal/cli/r23_cli_writes_test.go`: the new tests.

## NOT VERIFIED on a live screen

Nothing here is a UI row. A real crash or full disk during the brief write was not staged; the test pins the write path, not the crash. The repeat prints the same stderr line `Saved. The cancel will be carried out on the next pass…` as the first press, since the CLI cannot tell a repeat from a first press without a change to the shared writer path.
