# C-ke3-brain-help-line

Base: lane-C-ke2-mount-finish (ee600fa7). Orchestrator decision 21:52, option (b), as a general rule.

## Proof table

| Row | Result | Proof |
|---|---|---|
| Step 1: quote the failure on base | HOLDS | `go test -count=1 ./internal/cli/ -run TestExHelp` exit 1: `ex_help_summary_verbs_test.go:100: help line for brain is 122 columns: "  brain       show/search/topics/index/propose/accept/keep/decide/supersede/check/changes/revert/drop/restore/write/rename"` |
| 1. `go test -count=1 ./internal/cli/` passes | HOLDS | exit 0: `ok  github.com/gridconsole/grid-plugins/internal/cli 30.356s` |
| 2. Mutant: plan drops `migrate`, keeps the pointer | HOLDS | cli.go plan summary set to `write, check, items, hash, diff (grid plan help lists all)`: `--- FAIL: TestExHelpSummariesNameEveryVerb ... ex_help_summary_verbs_test.go:91: grid help: plan leaves out verbs [migrate]`, exit 1. Reverted with `git checkout -- internal/cli/cli.go`; `git status --short` clean. |
| 3. `grid brain help` lists all 16 verbs | HOLDS | `go run ./cmd/grid brain help`, exit 0: usage line `<show\|search\|topics\|index\|propose\|accept\|keep\|decide\|check\|changes\|revert\|drop\|restore\|write\|rename\|supersede>` and a line for each of the 16; no change needed. |
| 4. braincmd, brain, contract | HOLDS | `go test -count=1 ./internal/braincmd/ ./internal/brain/ ./contract/` exit 0: all three `ok` |
| 4. `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` | HOLDS | exit 0: `ok .../internal/cli 0.042s` |
| 4. `./internal/job/lint/` | HOLDS | exit 0: `ok .../internal/job/lint 0.192s` |
| 5. `make golden` | HOLDS | exit 0: `PASS ok .../internal/golden 0.289s` |
| 5. `make lint` | HOLDS | exit 0 (go vet clean, no gofmt output) |
| 5. `make cross` | HOLDS | exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| Packages importing internal/cli | HOLDS | `go list` shows cmd/grid (no tests) and lib; `go test -count=1 ./cmd/grid/ ./lib/` exit 0 (`ok lib 11.126s`) |
| Walk tests, app-only (webui, fv, grid-server, slices) | n/a | plugin repo; the plugin walk tests are the two rows above |
| No guard/gate/cap on agents | HOLDS | only a help summary and test rules changed |
| Real-plugin tests | n/a | none named in this brief |

## What changed

- `internal/cli/cli.go`: brain summary is now `show/search/topics/index/propose/accept/keep/decide/check/write (grid brain help lists all)` (95 columns after the prefix, line under 120).
- `internal/cli/ex_help_summary_verbs_test.go`: `TestExHelpSummariesNameEveryVerb` accepts a summary ending `(grid <group> help lists all)` only when the group's full verb list would not fit. The test computes that from the verb names, the prefix (two spaces, the widest command name, two spaces) and the limit, with a one-column separator (the most lenient case). Nothing is hardcoded to brain. The 120 limit is now the constant `exHelpLineLimit`, same value; the layout is unchanged.
- Regenerated with make: `contract/commands.json` (`make generate`) and `testdata/golden/help.golden:24` (`make test UPDATE_GOLDEN=1 "GOFLAGS=-count=1 -run=TestDtGateWordsHelpGolden"`). `make golden` itself does not write help.golden; it replays the legacy corpus.

## Stated departure from the no-edit-existing-test rule

`TestKeMtGridHelpExits` (`internal/cli/ke_mount_test.go:138`) pinned the old 16-verb brain line, which decision (b) removes. I changed only that expected string to the new summary. The two exit-status strings in it are unchanged. Written in the orchestrator's decision of 21:52 (option b).

## Open items

- Which verbs the brain line names is my pick: the read and curate verbs most used (show, search, topics, index, propose, accept, keep, decide, check, write). Say so if you want others.

## NOT VERIFIED on a live screen

Nothing here needs a screen.

Verdict: DONE
