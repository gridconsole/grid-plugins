# C-ic-suite-convert: `grid eval export-suite`

Answer to simplify item 11, from `TestIcxExportShippedSuite` on the shipped evals/tasks:

    totals: 43 cases: 36 checks carried as graders, 222 dropped; 24 cases have no grader left
    dropped by type: map[command_passes:222]

So 36 of the 258 shipped checks (21 text_contains, 15 file_exists) carry over to `claude plugin eval`. All 222 command_passes checks do not, and 24 of the 43 cases are left with no grader at all (for example `execute-check-fails: 0 graders, 6 dropped`).

## Proof table

| Row | State | Proof |
|---|---|---|
| 1 TestIcxExportMaps | HOLDS | `go test -count=1 -run '^TestIcxExportMaps$' -v ./internal/evalcmd/` exit 0: `--- PASS: TestIcxExportMaps`. It compares every written file byte for byte (prompt.md, case.yaml, fixture.sh, 7 graders) and then grades the export with the repo's own reader of the layout (`GradeCase`): 6 graders pass and `judge` (llm) is not graded |
| 2 TestIcxExportNamesWhatItDrops | HOLDS | `go test -count=1 -run '^TestIcxExportNamesWhatItDrops$' -v ./internal/evalcmd/` exit 0: `--- PASS: TestIcxExportNamesWhatItDrops`. Dropped = {drops, runs-tests, command_passes, reason}, graders/ holds only kept.md, and the verb's `--json` prints the same entry |
| 3 TestIcxExportShippedSuite | HOLDS | `go test -count=1 -run '^TestIcxExportShippedSuite$' -v ./internal/evalcmd/` exit 0: `--- PASS: TestIcxExportShippedSuite`. Every case has prompt.md and graders/, and each case with a grader parses with `GradeCase`. Totals line above |
| 4 make generate + tests | HOLDS | `make -C . generate` exit 0. `go test -count=1 ./internal/evalcmd/ ./internal/cli/` exit 0 (`ok internal/evalcmd 0.725s`, `ok internal/cli 67.904s`). These pass by name, with -v: `TestContractCommandsParserInventory`, `TestExHelpSummariesNameEveryVerb`, `TestDtGateWordsHelpGolden` (the help golden test is named this at internal/cli/help_golden_test.go:11), `TestGgUnknownFlagWalk`, `TestGgUnknownFlagWalkCoversTheContract`, `TestEvalCommands`. I took the base failures at :384 and :63 from REVIEW and did not measure them again here. See the note on eval_test.go:52 below |
| 5 diff only Touches | HOLDS | `git diff --name-only origin/main` before the report lists exactly contract/commands.json, internal/cli/cli.go, internal/cli/contract_inventory_test.go, internal/evalcmd/command.go, internal/evalcmd/icx_suiteexport_test.go, internal/evalcmd/suiteexport.go, testdata/golden/help.golden. This report (docs/lanes/C-ic-suite-convert.md) is added because the builder runbook's Definition of Done requires it. `make generate` changed only contract/commands.json |
| New tests fail on base | HOLDS | I copied the test file into a worktree of origin/main and ran `go test -run '^TestIcx' ./internal/evalcmd/`: `icx_suiteexport_test.go:74:17: undefined: ExportSuite`, `:78:79: undefined: ExportedCase`, `:157:14: undefined: Report`, `FAIL ... [build failed]`. The worktree was removed afterwards |
| No existing test edited to pass, none deleted | HOLDS | The only test-file change is the one annotation row `"eval export-suite": {},` at internal/cli/contract_inventory_test.go:568, which the brief names |
| No guard, gate, cap or hold on agents | HOLDS | The verb only writes files and prints a report. It adds no hook and no limit |
| Tests of changed packages and their importers | HOLDS | `go list` importers of internal/evalcmd and internal/cli: cmd/grid, internal/cli, lib. Importer of contract/: internal/contract. `go test -count=1 ./cmd/grid/ ./lib/ ./internal/contract/` exit 0 (`ok lib 86.167s`, `ok internal/contract 1.850s`; cmd/grid has no test files) |
| Lint | HOLDS | `make -C . lint` exit 0 (go vet ./... plus gofmt -l, after commit "Format the export-suite test file with gofmt.") |
| Windows and macOS builds | HOLDS | `make -C . cross` exit 0: vet and test compile for darwin/arm64 and windows/amd64, every package ok |
| contract/commands.json only by make generate | HOLDS | Not edited by hand and no `UPDATE_...=` prefix used. The regenerated route has flags --json, --out and --suite only, with no --results, --size or --tier |
| Fixtures synthetic, temp dirs, no clock or sleep | HOLDS | icx_suiteexport_test.go: t.TempDir only, people `alex`/`sam`, no time or sleep |
| Kept off internal/eval/runner_test.go, suite_test.go; nothing removed | HOLDS | diff list above |
| No claude / curl / server / push | HOLDS | none run |
| Small commits, no attribution | HOLDS | 3 commits with one-sentence messages and no trailers |
| UI rows / RealPlugin rows | n/a | the brief has none |

## What changed

- internal/evalcmd/suiteexport.go: `ExportSuite(src, dst) (Report, error)` and the `export-suite` verb with its own flag set (--suite, --out, --json). ExportSuite loads and converts the whole suite before it writes anything, and it refuses to write a case whose folder already exists in dst. Each check maps as follows:
  - file_exists becomes file_exists.
  - text_contains becomes regex, with `target: {source: file, path}` and `pattern:` set to the QuoteMeta of the text, quoted in the front matter so whitespace at either end survives.
  - llm_judge becomes llm, with the rubric as the body.
  - tool_used becomes tool_used with `min` (count_min, default 1) or `max: 0` when absent. A one-field input_contains becomes `input_match`.
  - tool_order becomes tool_order, with before and after as a tool name or `{tool, input_match}`.
  - command_passes goes to Report.Dropped. So does a check with top_level, an input_contains with two or more fields or a value that is not text, or a file_exists path that holds a glob character. Each has a reason.
  - prompt.md gets `runs: 1` and `timeout_seconds: 300` front matter, and its body is kept unchanged. setup.sh is copied as fixture.sh (0755), and case.yaml is `schema_version: "1.1"`, `name`, `context.scaffold_script: fixture.sh`.
- internal/evalcmd/command.go: the verb gate takes export-suite and branches before the shared flag set, the usage text names it, and `defaultTimeout` is now a constant shared by `run --timeout` and the export.
- internal/cli/cli.go:139 and testdata/golden/help.golden:42 now read: `list, run and show task evaluations; export-suite writes plugin eval cases`. The old phrase is kept word for word because internal/cli/eval_test.go:52 checks for it. My first wording ("list, run, show and export-suite ...") failed that test (`eval_test.go:53: registration`). I changed the wording inside Touches and did not edit the test.
- contract/commands.json: regenerated by make generate.

## Open items and NOT VERIFIED

- NOT VERIFIED live: whether `claude plugin eval` accepts the 19 cases that have graders. The format matches the repo's offline reader (suitegrade.go) and the existing evals/suite cases. Two things are unproven: whether the built-in reads `pattern:` from the front matter (the evals/suite cases put the pattern in the body), and whether it accepts a case with an empty graders/ folder. 24 cases have one.
- NOT VERIFIED live: fixture.sh is setup.sh unchanged. The Go runner exports variables to setup (such as GRID_HOME, which many shipped setups use), and a `--scaffold` run may not set them.
- Not carried and not reported: weight (49 checks with 2, 9 with 3) and gate (29 checks) in the shipped suite. The built-in's checked grader types have no weight key.
- The contract shows `--out` as `"required": false` because the generator does not mark it. The verb exits 2 without it.

DONE
