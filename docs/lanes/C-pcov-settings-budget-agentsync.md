# C-pcov-settings-budget-agentsync

Tests for budget preset validation, BudgetUndo and the agent file sync helpers. Tests only: one new file, `internal/settings/budget_agentsync_test.go`. No non-test file changed.

## Proof table

Coverage before (untouched clone, linux, `go test -cover ./internal/settings/ -count=1`): `ok ... coverage: 83.7% of statements`.
Coverage after (same command, final run, exit 0): `ok ... coverage: 85.7% of statements`.

| Row / rule | Result | Proof |
|---|---|---|
| 1 validateBudget refusals, one subtest each, error names the field; codex compact 0 passes | HOLDS | `TestValidateBudgetRefusals` (15 subtests: missing, extra, wrong name, compact 99999 / 1000001, advisor gpt, no agents, unknown agent, empty effort, empty model, model and effort refused by validateAgent, default turbo, codex turbo, codex compact 5), `TestValidateBudgetAccepts` (codex compact 0, bounds 100000 / 1000000, empty default), `TestValidateBudgetDefaultsPass` (built-in table and the fixture note pass). `go test ./internal/settings/ -count=1` exit 0. validateBudget 59.3% -> 100.0% (`go tool cover -func`) |
| 2 BudgetUndo after Apply: changed file back, created file gone, snapshot gone, second Undo is CodedError nothing_to_undo | HOLDS, with one deviation | `TestBudgetUndoRestoresChangedAndRemovesCreated`: Apply High changes grid-builder only and creates the overlay `<grid>/settings.json`; Undo puts builder back byte for byte, removes the overlay and the snapshot; second Undo `errors.As` -> `*CodedError` Code `codeNothingToUndo`, Msg "There is no Apply to undo.". Deviation: BudgetApply never creates an agent file (`budget.go:560-561`, a missing file is "Skipped"), so the created file in a real Apply is the overlay, and `Restored` lists `["grid-builder"]` (the overlay is not an agent, `budget.go:904-907`), not both. The "created agent file is removed and listed in Restored" path (`budget.go:850-852`) is covered by `TestBudgetUndoRemovesAnAgentFileTheSnapshotSaysWasCreated`, which writes the snapshot by hand. |
| 3 BudgetUndo with one agent file edited after Apply | HOLDS | `TestBudgetUndoSkipsTheFileThePersonEditedAfterApply`: grid-builder edited; it is untouched, `Skipped = [{grid-builder, "changed after apply"}]`, the other four are back to pre-Apply bytes, `Restored` has 4. Exit 0. |
| 4 projectFile | HOLDS | `TestProjectFile`: ProjectDir "" false; no file false; regular `.claude/agents/<name>.md` true; directory of that name false. projectFile 50.0% -> 100.0%. tilde 100.0%, fileSHA 100.0% (`TestTildeAndFileSHA`). |
| 5 writeRendered | HOLDS | `TestWriteRendered`: new file -> `written` and parent made; same bytes -> `unchanged`; directory -> `yours`; file without marker -> `yours`, bytes unchanged; Grid-owned different file -> `written`; parent that is a regular file -> error, state "", parent untouched; symlink -> `yours`, target unchanged (Skip on windows). writeRendered 68.8% -> 81.2% (above 75%). |
| 6 splitFront | HOLDS | `TestSplitFront`: normal file (front lines split on "\n", body starts "\n"); no leading marker; `---\n---\n` -> ok, no lines, body "\n"; no closing marker (three inputs); CRLF `---\r\n` -> not ok (pinned). splitFront 66.7% -> 100.0%. |
| 7 ClaudeAgentsDir, ClaudeRowNames, defaultManagedPath, CodedError | HOLDS (defaultManagedPath: compiled, needs Windows / macOS run for the other branches) | `TestClaudeAgentsDir` (`<home>/.claude/agents`; `<configDir>/agents`; relative configDir made absolute; both empty -> error), `TestClaudeRowNamesIsACopy`, `TestDefaultManagedPath` (checks the runtime.GOOS branch; on linux it walks the default branch only, 50.0%), `TestCodedErrorMessageAndUnwrap` (Error 100%, Unwrap 100%). Windows branch: compiled, needs Windows run: `go test ./internal/settings/ -run TestDefaultManagedPath`. macOS branch: compiled, needs macOS run: same command. |
| 8a `go test ./internal/settings/ -count=1` | HOLDS | exit 0, 0 fail |
| 8b coverage | HOLDS | before 83.7%, after 85.7%. validateBudget 100.0%, splitFront 100.0%, writeRendered 81.2%, BudgetUndo 64.0% (not below its before figure) |
| 8c `GOOS=windows go test -c -o /dev/null ./internal/settings/` | HOLDS | exit 0 |
| 8d `GOOS=darwin go test -c -o /dev/null ./internal/settings/` | HOLDS | exit 0 |
| 8e `make lint` | HOLDS | exit 0 (`go vet ./...`); `gofmt -l internal/settings/` empty |
| Tests of the package and its importers | HOLDS | `go list` importers of internal/settings: internal/cli, internal/doctor, internal/guard, internal/hook. `go test -count=1 ./internal/cli/ ./internal/doctor/ ./internal/guard/ ./internal/hook/ ./internal/settings/`: all ok, exit 0 |
| New tests fail on base | NOT APPLICABLE | The lane adds tests for code that already behaves as the brief says; no behaviour changed, so the tests pass on base code too. The new file does not exist on base, so there is nothing to run there. |
| Existing tests / helpers not edited, nothing deleted | HOLDS | `git diff --stat origin/main..HEAD` shows one new test file and this report |
| No guard, gate, cap or hold added | HOLDS | tests only |
| Commit rules | HOLDS | one-sentence messages, no attribution lines |

## What changed

New `internal/settings/budget_agentsync_test.go`. It reuses `budgetFixture`, `agentText`, `put`, `contents`, `must` and the other helpers unedited.

## Findings (not fixed, per brief)

- `splitFront` does not read CRLF files (`agentsync.go:131`). Its callers `renderAgent` (`agentsync.go:155`) and `ownedByGrid` (`agentsync.go:202`) convert CRLF to LF first, so CRLF templates and CRLF rendered files work. No bug found.
- `BudgetApply` skips a missing agent file instead of creating it (`budget.go:560`); the brief's "a file Apply created" is therefore the overlay in a real run.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: the Windows and macOS test binaries (compiled only) and the BudgetUndo branch for an overlay that moved on after Apply (`budget.go:877` on), which the brief leaves out of this lane.
