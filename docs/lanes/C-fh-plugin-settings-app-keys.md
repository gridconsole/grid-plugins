# C-fh-plugin-settings-app-keys

The plugin reads and keeps the seven keys the app saves in the same `<home>/settings.json`, instead of refusing the whole file.

Base: `b17389b791d078adc7377e41457ed8e54a7526c2` (lane-Pint84 merge, `origin/main`). Base check: `internal/settings/json.go:73` on base passes `object` only `agents, skills, hooks, budget, accounts`, so `notifications` was refused (`unknown key "notifications"`, shown below). Not already done.

## Proof table

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | Effective on the app's file: no error, default agents, skills, hooks. Fails on base, passes after | HOLDS | Base (tests alone, fix stashed), `go test -count=1 -run TestEzAppKeys ./internal/settings/` exit 1: `ez_app_keys_test.go:132: a settings.json the app saved must read: settings.json: unknown key "notifications"; allowed: agents, skills, hooks, budget, accounts`. After: same command, exit 0, five tests PASS (`go test -v`). Test: `TestEzAppKeysEffectiveReadsTheAppFile` in `internal/settings/ez_app_keys_test.go` |
| 2 | `SetAgent`, `SetSkill`, `SetHook` each succeed; the seven keys and accounts read back JSON-equal (UseNumber, one object); the plugin's change is present | HOLDS | `TestEzAppKeysPluginWritesKeepThem` PASS: after each write it compares the eight keys with `reflect.DeepEqual` on a `UseNumber` decode (a number past 2^53 stays `9007199254740993`), accounts also with `sameJSON`, and the plugin's `agents`, `skills`, `hooks` sit beside them. It also covers `SetAgents`. `TestEzAppKeysBudgetApplyAndUndoKeepThem` PASS (the budget apply and undo rewrite the same file) |
| 3 | `ResolveHookLevel` for a hook set to `off` answers `off`, no error | HOLDS | `TestEzAppKeysResolveHookLevelAnswersTheSetLevel` PASS. On base: `home-allow set to off resolves to "block", Cannot read hook settings: settings.json: unknown key "notifications"; ...` |
| 4a | `{"unknown":1}` and `{"notifications":1,"unknown":2}` still refused | HOLDS | `TestEzAppKeysOtherKeysAreStillRefused` PASS (also an unknown key after an app key, a duplicate app key, a second JSON value; a refused file is not rewritten) |
| 4b | existing `store_test.go` passes unchanged | HOLDS | `go test -count=1 -skip TestUnknownSettingsKeysAreStillRefused ./internal/settings/` exit 0 (`ok ... 0.453s`); no existing test file touched (`git diff origin/main --stat`) |
| 4c | existing `accounts_key_test.go` passes unchanged | **NOT DONE** | See "Brief conflict" below. `go test -count=1 ./internal/settings/` exit 1, one failure |
| 5 | In-process CLI on the app-shaped file: `hook levels --json` 0, `hook level home-allow off` 0, `agent set grid-builder --model opus` 0, the eight keys still JSON-equal | HOLDS | `go test -count=1 -v -run TestEzAppKeys ./internal/cli/` exit 0: `--- PASS: TestEzAppKeysSettingsVerbsOnTheAppFile`, `--- PASS: TestEzAppKeysListsDoNotShowThem`. On base (production files stashed) exit 1: `hook levels --json on the app's settings.json: exit 1: hook: settings.json: unknown key "notifications"; allowed: agents, skills, hooks, budget, accounts`. `TestEzAppKeysListsDoNotShowThem` shows `agent list`, `skill list`, `hook levels` (plain and `--json`) print none of the app's keys or values. No built binary is run |
| 6a | `git diff main --stat` names only the four Touches files | HOLDS | `git diff main --stat`: `internal/cli/ez_app_keys_cli_test.go`, `internal/settings/ez_app_keys_test.go`, `internal/settings/json.go` (7 lines), `internal/settings/store.go` (40 lines added). This lane doc is the fifth file, as the runbook requires |
| 6b | `make lint` exit 0 | HOLDS | `make lint` (`go vet ./...`) exit 0 |
| 6c | `GOOS=windows go vet ./internal/settings/` exit 0 | HOLDS | exit 0 (also `./internal/cli/`) |
| R1 | Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| R2 | Tests of the changed packages and every importer | HOLDS except 4c | `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... \| grep grid-plugins/internal/settings` gives: `cmd/grid`, `internal/account`, `internal/cli`, `internal/doctor`, `internal/guard`, `internal/hook`, `internal/packaging`, `internal/packaging/cmd`, `internal/settings`, `lib`. `go test -count=1` on them: `account`, `cli`, `doctor`, `guard`, `hook`, `packaging`, `packaging/cmd`, `lib` ok; `cmd/grid` has no test files; `settings` fails only on the 4c test |
| R3 | New tests fail on base, pass on the lane | HOLDS | Settings tests: all five new tests fail on base (`go test -count=1 -run TestEzAppKeys ./internal/settings/` exit 1; the output is in rows 1 and 3; the refusal test fails there on its message assertions, its refusal cases hold on base too). CLI tests: both fail on base (row 5). The tests are the first commit, the fix the second |
| R4 | No guard, gate, cap, hold or blocking hook added | HOLDS | The diff only widens what `decode` accepts and carries values; no agent-facing check was added |
| R5 | No existing test edited or deleted; no commit attribution; no personal path, host or email | HOLDS | `git diff origin/main --stat` lists no existing test file. The test data uses `ann@example.com`. Commit messages carry no attribution lines |
| R6 | RealPlugin tests | NOT RUN, no row needs one | No acceptance row names a `*RealPlugin*` test, so the plugin was not built |

## Brief conflict: row 4c is NOT DONE

Row 4 says `accounts_key_test.go` passes unchanged. It cannot, because the brief's own "What to build" removes the behaviour that file pins.

`internal/settings/accounts_key_test.go:87`, inside `TestUnknownSettingsKeysAreStillRefused`:

```go
"app key with the records": `{"accounts":[],"accountSelection":{"mode":"use","acct":"","fullAt":95}}`,
```

The test expects `Store.Effective()` and `BudgetShow` to refuse this input. The value is a legal app value (app `check.go`: `mode` is smart, round or use; `fullAt` is 85, 90, 95 or 100), and `accountSelection` is one of the seven keys the brief tells decode to accept and keep verbatim. Output as seen after the fix:

```
--- FAIL: TestUnknownSettingsKeysAreStillRefused (0.00s)
    accounts_key_test.go:95: app key with the records: must still be refused
    accounts_key_test.go:98: app key with the records: budget show must still refuse
```

The other five rows of that test (unknown top key, accounts not an array, accounts null, accounts twice, unknown agent key) still pass. I did not edit the test: the rules forbid editing an existing test to make it pass, and Touches names no other file. No implementation satisfies both the row and "What to build", because the plugin is told not to validate the app's values.

For the orchestrator: delete that one map entry (`accounts_key_test.go:87`). The behaviour is removed by the brief's "What to build" paragraph; `TestEzAppKeysEffectiveReadsTheAppFile` and `TestEzAppKeysPluginWritesKeepThem` now cover the same input and pin the opposite, correct answer. `TestEzAppKeysOtherKeysAreStillRefused` keeps the unknown-key refusal covered.

## What changed

- `internal/settings/json.go`: `decode` passes `object` the plugin's five keys plus the seven app keys (`ezKeysAppNames`), and keeps each app value verbatim in the document. A key that is neither is still refused as before.
- `internal/settings/store.go`: `Document` gains one embedded field, `ezKeysApp`, holding the seven values as `json.RawMessage`. Encoding inlines it, so every `MarshalIndent` of a `Document` (`update`, budget apply and undo) writes the keys back beside the plugin's own. The values are never merged into `Effective` (`merge` is unchanged), so no list shows them. Key order on rewrite follows the struct: `agents, skills, hooks, budget, accounts`, then `notifications, models, usageRules, windowRules, workspace, accountSelection, hiddenProjects`.
- Two new test files, `internal/settings/ez_app_keys_test.go` and `internal/cli/ez_app_keys_cli_test.go`.
- The values are not checked: `{"notifications":1}` is carried as is. The app owns the shape of its keys, as for `accounts`.

## NOT VERIFIED on a live screen

- No screen is involved. The app was not run against a file the plugin rewrote. The app's read is reproduced in the tests (one object, `json.Decoder.UseNumber`, as in app `store.go:79-87`), and the test file is modelled on the app's `check.go` shapes, not on a file a running app saved.
- No `*RealPlugin*` test was run (see R6).
