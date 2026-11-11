# C-ei-plugin-account-statusline

`grid account statusline on|off` installs the status line that feeds the 5-hour window and keeps the person's own status line. `note` gains `--then CMD` and `--show`.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. `statusline on --json` writes the object, other keys untouched, second on is a no-op | HOLDS | `go test ./internal/account/ -run TestStatusLineOnKeepsEveryOtherKeyAndIsIdempotent` exit 0 (internal/account/ei_statusline_test.go). Compares the parsed objects without statusLine, a 20-digit number keeps its text, and the second on leaves mtime and bytes unchanged. |
| 2. Earlier `my-line.sh` is wrapped, `kept` reports it, off puts it back, off with no earlier key removes it | HOLDS | `TestStatusLineOnWrapsTheirCommandAndOffPutsItBack` and `TestStatusLineOffRemovesTheKeyWeMade` exit 0. Also holds for `padding` and the field order in the statusLine object, and for a command with a pipe or quotes (`TestStatusLineQuotesACommandThatIsNotOneWord`). |
| 3. `note --then 'printf hi'` writes the snapshot and prints `hi`; a payload that is not JSON still prints `hi`, exit 0 | HOLDS | `TestNoteThenKeepsNumbersAndPassesTheirOutput`, `TestNoteThenSurvivesAPayloadItCannotParse` exit 0 (also empty stdin, no Grid home, and the command's exit code passes through). |
| 4. `note --show` prints `42%` and `18%`; a second CLAUDE_CONFIG_DIR gets its own snapshot | HOLDS | `TestNoteShowPrintsBothWindowsUnderTheConfigFolder` and `TestNoteShowIsEmptyWithoutAWindow` exit 0; output is `5h 42% · 7d 18%`. |
| 5. `make test`, `make lint`, `GOOS=windows go vet ./internal/account/ ./internal/cli/` | HOLDS | `make test` exit 0, "temp leak check: no new matching entries". `make lint` exit 0. `GOOS=windows go vet ./internal/account/ ./internal/cli/ ./internal/settings/` exit 0. `GOOS=darwin go vet` on the same three exit 0. `GOOS=windows` and `GOOS=darwin` `CGO_ENABLED=0 go build ./cmd/grid` exit 0. |
| New tests fail on base | HOLDS | With main's `account.go` and `windows.go` and without the new files, `go test ./internal/account/ -run 'StatusLine\|NoteThen\|NoteShow'` exits 1 with 10 FAIL, for example `ei_statusline_test.go:143: exit 2: account: unknown command "statusline"`, `ei_statusline_test.go:334: exit 2: flag provided but not defined: -then`, `ei_statusline_test.go:379: exit 2: flag provided but not defined: -show`. |
| No existing test edited to pass | HOLDS, with one allowlist line | `internal/cli/settings_choke_test.go` gets one entry in its reviewed-exports map for `EditClaudeStatusLine` (see below). No assertion changed. No test deleted. |
| Packages that import the changed ones tested | HOLDS | `go list` importers of account, settings and cli: cmd/grid, contract/formats, internal/doctor, internal/golden, internal/guard, internal/hook, internal/mcp, internal/orchestrator, internal/packaging, internal/question, lib. All `ok` in `make test`. |
| No guard, gate, cap or hold on agents | HOLDS | Nothing added that blocks or limits an agent. The status line wraps a person's command and never stops it. |
| Public repo: no personal paths, hosts or emails | HOLDS | Tests use `t.TempDir()`. |
| Commit messages: one sentence, no attribution | HOLDS | `git log origin/main..HEAD`. |

## What changed

- `internal/account/statusline.go` (new): `StatusLineSet` (on, off), `NoteThen`, `NoteShow`. The person's command is quoted as one shell word when it is not plain, and off undoes the quoting. Off needs no record file: it reads the command back out of `grid account note --then ...`, or removes the key when the command is `--show`.
- `internal/account/windows.go`: `Note` keeps its signature (existing tests call it). New `NoteKept` returns the snapshot, and `parsePayload` is split out.
- `internal/settings/claude_statusline.go` (new): `EditClaudeStatusLine`, using `readClaudeFile`, `stageClaudeFile`, the unchanged check and the rename. Only the statusLine key changes. A statusLine that is not a command object, or a settings file that is not JSON, is refused and nothing is written. Other keys keep their order and number text; the file is re-indented as setup does.
- `internal/cli/account.go`: `statusline on|off [--json]`, `note --then`, `note --show`, help text.
- `internal/cli/contract_inventory_test.go`: the one annotation line `"account statusline"`.
- `contract/commands.json`: the `statusline` route and the `--show`/`--then` flags of `note`, edited by hand in the order the inventory test writes (the UPDATE_CONTRACT_COMMANDS env var was refused in the sandbox). `TestContractCommandsParserInventory` passes against it.

## Deviation from the Touches list

The brief names `readClaudeFile` and `stageClaudeFile`, which are unexported in `internal/settings`. To use them I added a new file there with one exported function, and `TestSettingsStoreChokePoint` requires every settings export to be on its reviewed list, so `internal/cli/settings_choke_test.go` gained one line. Neither file is on the Touches list. The reviewer should decide whether this is the right place.

## Notes

- `TestApproveRecordWorkerPassMovesTheReceiptOfATaskOnlyRead` (approve_record_test.go:406, "accepting controls []") failed in two grouped runs of `./internal/cli` and passed alone and in the full `make test` run. It is unrelated to this lane, and it passes on main alone.

## NOT VERIFIED on a live screen

- That Claude Code runs the written `statusLine` command and feeds it the payload. The tests use the documented payload shape (`rate_limits.five_hour|seven_day.used_percentage|resets_at`), not a live session.
- `--then` runs through `sh -c` (`cmd /C` on Windows when there is no `sh`). The `--then` tests are skipped on Windows, and the Windows path was only vetted and built, not run.
