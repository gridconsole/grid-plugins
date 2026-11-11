# C-gm-usage-text-to-stderr-on-bad-flag

## Proof table

| Row | Result |
|---|---|
| 1. `go test -count=1 -v -run TestGmUsageOnBadFlag ./internal/cli/` FAILs on base, PASSes after | HOLDS. Base (new test, old closures), exit 1: `gm_usage_stderr_test.go:45: plain stdout not empty: Usage: grid deliver --outcome <text> ...` and the same for proof add, proof index, history add. After: PASS for all four, plain and `--json`, exit 0. |
| 2. `TestGmUsageOnHelp` PASSes on base and after | DIFFERS ON BASE, PASSES AFTER. On base, `deliver --help` passes, but `proof add --help`, `proof index --help` and `history add --help` fail: `stderr = "  -by string\n ..."` (`fs.PrintDefaults` writes the flag list to stderr, while the test asks for empty stderr). The closures now send usage and defaults together to stdout on help; after the change all four pass, exit 0. |
| 3. HOME, GRID_HOME, CLAUDE_CONFIG_DIR, CODEX_HOME under t.TempDir(), t.Fatal when HOME is outside | HOLDS. `gmIsolate`, gm_usage_stderr_test.go:12-23. |
| 4. Four members gone from `ggFlagKnownOpen`; `TestGgUnknownFlag` PASSes with `known open: 5 of ...` | HOLDS for 5; the denominator reads `5 of 136` on this base (brief said 137; the walk now reports walked 136, contract 143). `go test -count=1 -v -run "TestGgUnknownFlag"` exit 0. |
| 5a. `go test -count=1 ./internal/cli/` exit 0, delivery_test.go:311 and :375 unchanged | HOLDS. `go test -count=1 ./internal/cli/ ./cmd/grid/ ./lib/` exit 0 (importers of internal/cli per go list: cmd/grid, lib). |
| 5b. `git diff main --stat` names only Touches files | HOLDS, with one addition inside deliver.go: a 15-line helper `usageOut` (deliver.go:16-31) that the three closures share. No other file. |
| 5c. `make lint` | HOLDS, exit 0 (gofmt of the map fixed in its own commit). |
| 5d. `GOOS=windows CGO_ENABLED=0 go build ./...`, `GOOS=darwin ...` | HOLDS, both exit 0. |
| New tests fail on base, pass on lane | HOLDS (row 1 output above). |
| No existing test edited, none deleted | HOLDS. Only four map entries removed, as the brief allows. |
| No guard, gate, cap or hold added | HOLDS. |
| *RealPlugin* tests | Not applicable; no row names one. |

## What changed
- `internal/cli/deliver.go`, `proof.go`, `history.go`: the Usage closures print through `usageOut(args, env)`, which gives stdout when the arguments hold `-h`, `-help` or `--help` (before `--`), otherwise stderr. proof and history also point `fs.PrintDefaults` at that writer. A bad flag now leaves stdout empty, so `commandFailure.finish` writes the `invalid_argument` envelope under `--json`.
- `internal/cli/gg_unknown_flag_walk_test.go`: four map entries removed (and gofmt realignment).
- `internal/cli/gm_usage_stderr_test.go`: `TestGmUsageOnBadFlag`, `TestGmUsageOnHelp`.
- `usageOut` tests the flag by trimming dashes rather than writing a `"--h"` literal, because `TestContractCommandsParserInventory` collects probe flags from source literals and failed on `version --h` when I first wrote the literal.

## NOT VERIFIED
Nothing needs a live screen. A bad flag combined with `--help` in the same argv is printed to stdout (help wins); not tested.
