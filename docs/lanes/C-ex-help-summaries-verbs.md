# C-ex-help-summaries-verbs

Base: 7d516e1c (main with lane-C-eo-plugin-gate-words ea2fcf64 merged; `git branch -a --contains ea2fcf64` lists this lane's base).
Head: see the handoff report.

## Proof table

| Row | Result |
|---|---|
| Base: help_golden_test.go on base and green before the change | HOLDS: `go test -count=1 -run 'TestDtGateWordsHelpGolden\|TestContractCommandsParserInventory' ./internal/cli/` exit 0 on 7d516e1c |
| 1. Before: check prints the ten commands with missing verbs | HOLDS: `go test -run TestExHelp ./internal/cli/` on base, exit 1: `history leaves out verbs [add]`, `worker [next]`, `account [folder list note scan sign-in status statusline windows]`, `signal [move]`, `task [decline delete rename restore]`, `question [ask]`, `brain [supersede]`, `workflow [write]`, `intent [release renew]`, `job [order trace]`. After: same test, exit 0 |
| 2. Setup has `Claude Code settings` and `--sandbox`; account names all account verbs | HOLDS: internal/cli/cli.go:136, :133; TestExHelpSetupSummaryNamesSettingsAndSandbox passes. The parser has eight account verbs, not seven: `statusline` is also there (added by lane-C-ei-plugin-account-statusline); the summary names it too |
| 3. cli.go diff only summary strings; help lines under 120 columns | HOLDS: `git diff 7d516e1c -- internal/cli/cli.go` hunks are lines 108-137, all inside commands(); TestExHelpLinesFitOneLine runs `grid help` through the harness, passes |
| 4. TestContractCommandsParserInventory passes; commands.json diff only `summary` | HOLDS: `make generate` exit 0 (touched only contract/commands.json); `git diff 7d516e1c -- contract/commands.json` shows 11 summary lines changed, nothing else; test passes in `go test ./internal/cli/` |
| 5. TestDtGateWordsHelpGolden passes; help.golden diff only the rewritten summary lines | HOLDS: golden edited by hand from the test's `got` output (no UPDATE_GOLDEN); 11 lines changed (task, job, question, history, brain, worker, workflow, intent, account, setup, signal); test passes |
| 6. Diff names only Touches files; cli tests; lint; windows vet | HOLDS: `git diff --stat` = cli.go, commands.json, help.golden (+ the new test file, + this doc); `go test -count=1 ./internal/cli/` exit 0 (ok 32s); `make lint` exit 0; `GOOS=windows go vet ./internal/cli/` exit 0; `GOOS=darwin go vet ./internal/cli/` exit 0 |
| Importers tested | HOLDS: importers of internal/cli are cmd/grid (no test files) and lib; `go test -count=1 ./cmd/grid/ ./lib/` exit 0 |
| New tests fail on base, pass on lane | HOLDS: base failures pasted in row 1 and `setup summary "record validated project setup answers" lacks "Claude Code settings"` / `lacks "--sandbox"`; all pass on the lane |
| No guard, gate, cap or hold added; nothing removed | HOLDS: only summary strings changed |
| No attribution lines in commits | HOLDS |
| *RealPlugin* tests | not needed: no row depends on one |

## What changed

- internal/cli/cli.go: eleven summary strings in commands() (task, job, question, history, brain, worker, workflow, intent, account, setup, signal).
- contract/commands.json: the same eleven `summary` values, from `make generate`.
- testdata/golden/help.golden: the same eleven lines.
- internal/cli/ex_help_summary_verbs_test.go: TestExHelpSummariesNameEveryVerb, TestExHelpSetupSummaryNamesSettingsAndSandbox, TestExHelpLinesFitOneLine.
- docs/lanes/C-ex-help-summaries-verbs.md: this file (runbook).

Notes: the history summary is reworded ("add a dated line to the task history") so the verb `add` is a word of it. The task, job, brain and setup lines are over the brief's "about 90" but under the 120 limit (TestExHelpLinesFitOneLine); the task and job lines are verb lists that cannot be shortened without dropping a verb. The worker `next` and workflow `write` phrases describe the verbs from their usage strings (internal/cli/worker.go:30, workflow.go:103).

## NOT VERIFIED on a live screen

Nothing here needs a live screen; `grid help` was checked through the test harness only, not a built binary in a terminal.
