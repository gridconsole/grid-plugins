# M-C-ke3-train-onto-main: the brain write train onto plugin main

Verdict: DONE. Every row holds.

## Proof table

| Row | Status | Command / place |
|---|---|---|
| 1. Both ancestors of HEAD | HOLDS | `git merge-base --is-ancestor origin/main HEAD && git merge-base --is-ancestor origin/lane-C-lo-plugin-brain-write-anyone HEAD && echo both-ok` printed `both-ok`, exit 0. |
| 2. `go test ./internal/contract/ -run 'TestHeMv\|TestExHelp' -count=1` | HOLDS | `ok  github.com/gridconsole/grid-plugins/internal/contract 0.260s`, exit 0. Before the test-file edit below it failed: `he_mv_message_verbs_test.go:209: internal/cli/cli.go:148: `grid brain help`: "help" is not a sub of `grid brain` in contract/commands.json`. |
| 3. `go test ./internal/braincmd/ ./internal/cli/ ./lib/ -count=1` | HOLDS | `ok .../internal/braincmd 0.921s`, `ok .../internal/cli 30.474s`, `ok .../lib 5.327s`, exit 0. |
| 4. `make lint` | HOLDS | `go vet ./...` then gofmt check, no output, exit 0. |
| commands.json lists `write` under `brain` | HOLDS | `contract/commands.json:3181` (`"write"`, args `<topic>/<name>`), inside the `brain` block that starts at :2617. `make generate` (doctor golden and `TestContractCommandsParserInventory` with UPDATE_*) changed no file, so the merged files were already current. |
| help golden carries ke3's pointer line | HOLDS | `testdata/golden/help.golden:24`: `brain  show/search/topics/index/propose/accept/keep/decide/check/write (grid brain help lists all)`; `TestGoldenCorpus` passes (`ok .../internal/golden`). |
| Nothing dropped | HOLDS | `git diff --diff-filter=D --stat origin/main HEAD` is empty: no file deleted. No test edited to pass (see the one addition below). |
| Packages of the change and their importers | HOLDS | Changed: braincmd, cli, contract (test only), lib. Importers by `go list`: cli imports braincmd; cmd/grid and lib import cli. `go test ./cmd/grid/ ./internal/contract/ ./internal/golden/ ./internal/braincmd/ ./lib/ -count=1`: all ok (cmd/grid has no test files), exit 0. |
| Windows and macOS builds | HOLDS | `make cross`: vet and test-compile of every package for darwin/arm64 and windows/amd64, all `ok`, exit 0. |
| Walk: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `ok .../internal/cli 0.040s`. |
| Walk: `go test -count=1 ./internal/job/lint/` | HOLDS | `ok .../internal/job/lint 0.070s`. |
| New tests fail on base, pass on lane | n/a | This lane adds no test. The tests it carries come from the five carried lanes. |
| No guard, gate, cap, hold or blocking hook | HOLDS | None added. |
| No attribution lines, no personal paths | HOLDS | Commits carry one sentence each and no trailers. |
| RealPlugin tests | n/a | No row names one. |

## What changed

1. `git merge origin/main` (d839c45d): one conflict, `lib/lib.go`, in the package doc list "These stay with the host process". Main deleted the bullet "the user name (USER, and the system account) that names a new task's owner (internal/task)", and the train left that bullet untouched, so I took main's list (the train changed other bullets in the same region, and main's rewrite of them is the one that was kept). `contract/commands.json` and `internal/cli/guest_parity_test.go` auto-merged.
2. `git merge origin/lane-C-lo-plugin-brain-write-anyone`: merged with no conflict. It touches `docs/decisions/D-ie-brain-write-decision.md`, `internal/braincmd/ke_write.go`, `ke_write_test.go`, a new `lo_write_anyone_test.go`, and its lane report.
3. One addition to an existing test file, a stated departure (dfe4d970): `internal/contract/he_mv_message_verbs_test.go` gets the entry `"internal/cli/cli.go:brain help"` in `heMvProse`. ke3's brain summary at `internal/cli/cli.go:148` ends with "(grid brain help lists all)". On plugin main `TestHeMvMessagesNameRealSubs` reads that as `grid brain help` and "help" is not a sub in commands.json, because the dispatcher answers help itself (`internal/braincmd/command.go:128`). Three sibling `brain help` entries already use the same list. The ke3 pointer line stays as the brief requires; the walk is not weakened for any other message. No assertion was removed.

## Open items

- The three older `brain help` entries in `heMvProse` cite `command.go:112`, which is now :128. Their text is stale but the test passes; left alone as outside the brief.

## NOT VERIFIED on a live screen

Nothing in this lane has a UI. Not run here: `make test` over the whole module and the cross-repo set, which the orchestrator runs at landing.
