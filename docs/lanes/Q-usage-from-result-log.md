# Q-usage-from-result-log: `grid usage post --result <attempt log>`

Base: plugin main 330256ce (the brief names 9eea4651; the clone's origin/main is 330256ce).

## Proof table

| Row / rule | Status | Proof |
|---|---|---|
| 1. Test fails on main: `post --task T-01 --job J-2 --result <log>` exits 2 | HOLDS | Commit 246ba506 has only the tests and fixtures. On 330256ce code `go test -count=1 ./internal/cli -run UsageResult` failed: `usage_result_test.go:62: first post: exit 2: flag provided but not defined: -result` (TestUsageResultPostBooksOnce), and `want exit 1 naming .../killed.log, got 2: flag provided but not defined: -result` for each refusal case. |
| 2. One row: job, session, dollars 0.476067, minutes, tokens, dearest model, key; `show --json` totals match | HOLDS | `TestUsageResultPostBooksOnce` (internal/cli/usage_result_test.go:48-80): job J-2, session 5f1c2a9e-..., model claude-opus-4-1 (0.41 > haiku 0.066067), dollars 0.476067, minutes 2.09 (125400/60000), tokens 174412 (12+3400+150000+21000), key `result:<session>`; show totals dollars/minutes/tokens/posts equal. `go test ./internal/cli -run Usage` exit 0. Also `TestUsageParseResultFixture` (internal/usage/result_test.go:20). |
| 3. Once only: repeat exits 0, usage.jsonl byte-identical, answer says already booked; other session appends | HOLDS | `TestUsageResultPostBooksOnce` compares raw bytes before/after, checks `"alreadyBooked":true`, then books attempt-2.log (session 9b7e4d20-..., 0.1234, 1 minute) as a second row. `TestUsagePostKeyBooksOnce` covers the same in the store, under the lock (internal/usage/usage.go Post). |
| 4. No result line / unreadable / cost missing or negative: exit 1 naming the file, nothing appended; trailing non-JSON still parses | HOLDS | `TestUsageResultPostRefusals`: killed-run, unreadable, no-cost, negative-cost, no-session (also exit 1, names the path, no usage.jsonl); `trailing-text` exits 0 and books. Parser unit cases in `TestUsageParseResultRefusals` and `TestUsageParseResultLastLineWins`. |
| 5. `--result` with `--dollars`/`--minutes`/`--tokens` exits 2, nothing appended; typed `--session`/`--model` override | HOLDS | `TestUsageResultConflictsAndOverrides` (internal/cli/usage_result_test.go): three flags give exit 2 and no file; typed session/model override while dollars stay 0.476067. Check is at internal/cli/usage.go before the log is read. |
| 6. usage.jsonl without `key` folds the same | HOLDS | Existing `TestUsageGolden`, `TestUsageCLI` pass unedited; new `TestUsageLogWithoutKeyFolds` folds a key-less log and then books a keyed row beside it. `key` is `omitempty`, `alreadyBooked` is `omitempty`, so typed-post JSON is unchanged. |
| 7. `go test -count=1 ./internal/usage/ ./internal/cli/ -run 'Usage'`; the contract inventory test; `go vet ./internal/usage/` | HOLDS | all three exit 0 (usage ok, cli ok; `TestContractCommandsParserInventory` ok; vet silent). |
| Tests of changed packages and importers | HOLDS | Importers of internal/usage by go list (imports, test imports, external tests): internal/cli, internal/hook (and usage itself). `go test -count=1 ./internal/usage/ ./internal/cli/ ./internal/hook/` all ok. |
| Lint | HOLDS | `make lint` exit 0 (go vet ./... and gofmt -l empty). |
| Windows and macOS builds | HOLDS | `make cross`: vet and test-compile for darwin/arm64 and windows/amd64, no FAIL or error lines. |
| `make generate` | HOLDS | contract/commands.json gains only the `--result` flag (value, not required) on `usage post`; a second `make generate` after the final edit left the tree unchanged. |
| New tests fail on base, pass on lane; no existing test edited | HOLDS | See row 1. `git diff origin/main --stat` shows no edited `_test.go` or golden. New tests are in new files only: internal/usage/result_test.go, internal/cli/usage_result_test.go. |
| No guard, gate, cap or blocking hook on agents | HOLDS | The only refusals are input validation of the post (as before); nothing stops or holds a run. |
| No personal paths, hosts or emails in the public repo | HOLDS | Fixtures use made-up session ids. |

## What changed

- `internal/usage/result.go`: `ParseResult(io.Reader) (Input, string, error)`. It takes the last `"type":"result"` line (non-JSON and trailing lines skipped), maps `total_cost_usd` to Dollars (missing, null or negative is an error), `duration_ms`/60000 to Minutes, the four `usage` counts summed to Tokens, `session_id` to Session (required, same pattern as a typed session), the dearest `modelUsage` model (lower name on a tie; a `[1m]` tag is cut) to Model, and returns `result:<session_id>`.
- `internal/usage/usage.go`, `fold.go`: `Input.Key`, `Record.Key` (`json:"key,omitempty"`), `PostResult.AlreadyBooked` (`alreadyBooked,omitempty`). `Post` checks the key under its lock after reading the log and appends nothing if it is there. The key is per task log, not per Job.
- `internal/cli/usage.go`: `--result <path>` on `post`.
- `contract/commands.json`: generated. Fixtures `internal/usage/testdata/attempt-1.log` and `attempt-2.log`.

## Notes for the reviewer

- The top-level `grid usage --help` text is pinned by golden `usage-help` (TestTokenHelp), so it was left alone to avoid editing an existing test's data; `--result` is described in `grid usage post --help`. Adding it to usageHelp needs a deliberate golden update.
- Old binaries (main) fold with `DisallowUnknownFields`, so a usage.jsonl with a `key` row is rejected by a binary without this change. Upgrade the readers with the writer.
- `num_turns` is not stored: the brief maps no field for it and Record has none.
- Follow-up, a separate app lane: the worker calls `grid usage post --job <J> --result <attempt log>` after each attempt.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: a real `claude` run, the worker calling the command, and the dev UI reading the ledger.
