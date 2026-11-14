# C-fix-plugin-settings-app-keys: the plugin hook accepts the app's own settings keys

Code commits: d453cbbb (reader and tests), 27c4fda2 (walk table). Base b5828ba0. `git merge origin/main` said "Already up to date".

## Proof table

| Row | Result |
|---|---|
| 1. New tests pass; the first fails on main | HOLDS. On the base source (`git checkout b5828ba0 -- internal/settings/json.go internal/settings/store.go internal/hook/settings.go`, then restored): `go test ./internal/hook/ -run TestSettingsFileWith` exit 1: `--- FAIL: TestSettingsFileWithAppKeysSetsTheHookLevel ... Cannot read hook settings: settings.json: unknown key "agentSandbox"; allowed: agents, skills, hooks, budget, accounts, notifications, models, usageRules, windowRules, workspace, accountSelection, hiddenProjects` and `--- FAIL: TestSettingsFileWithAnUnknownKeyWarnsAndIsStillRead ... unknown key "surprise"`. The settings-package tests (`TestEzAppKeysAgentRunKeysLoad`, `TestEzAppKeysUnknownKeyWarnsAndSurvivesARewrite`) fail to compile on base (`doc.Warnings undefined`); they pass on the lane. |
| 2. `go test ./internal/settings/ ./internal/hook/ -count=1` | HOLDS, exit 0 (settings ok, hook ok). |
| 2. `make lint` | HOLDS, exit 0 (`go vet ./...`). `gofmt -l internal` empty. |
| 3. Lane doc with the code SHA | HOLDS: this file, 27c4fda2. |
| Packages that import the changed ones (`go list`: account, brain, cli, doctor, guard, hook, job/jobcmd, packaging, settings, updates) | HOLDS, exit 0: `go test -count=1` on all ten, `updates` has no test files. |
| Windows and macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin CGO_ENABLED=0 go build ./...`, both exit 0. |
| Walk tests (plugin) | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok, `TestSettingsStoreChokePoint\|TestSettingsPackageChokeMutations` ok, `go test -count=1 ./internal/job/lint/` ok. |
| App-repo walk tests, `make realplugin` guest walk | not run, plugin repo lane. |
| Whole-module test, cross-repo set | not run here; the orchestrator runs them at landing. |
| Add no guard, gate, cap, hold or blocking hook | HOLDS: none added; the change removes a refusal. |

## What changed

- App key list (item 1). The app reads these top-level keys from settings.json in `internal/slices/settings/agent.go` (main): `claudePath`, `maxTurns`, `permissionMode`, `agentSandbox`; the Settings page saves `usageRules`, `windowRules`, `workspace`, `accountSelection`, `hiddenProjects` (check.go:14); `accounts` was already carried. The plugin list lacked the first four. `internal/settings/store.go` `ezKeysAppNames` and `ezKeysApp` now carry them verbatim (permissionMode was not in the brief's list; the app reads it, so it is added).
- Unknown key (item 2). `internal/settings/json.go` `scanObject`: at the top level a key outside the list no longer refuses the file. It is kept in `Document.unknown`, one line goes to `Document.Warnings` (`settings.json: unknown key "x" ignored; kept as it is`), and the rest of the file is read. `Document.MarshalJSON` writes the unknown keys back after the known ones, so a rewrite by `grid settings` never drops another program's key (the old refusal also protected this; the data is now kept instead). Repeated keys, non-object files, nested unknown names (agents, skills, hooks entries) and bad values are still refused.
- Hook. `settings.ResolveHookLevelWarn` and `ReadHookLevelsWarn` return the warnings; `internal/hook/settings.go` logs each to stderr as `grid hook: hook settings: settings.json: unknown key ...`. `ResolveHookLevel` and `ReadHookLevels` keep their signatures. The hook asks for the level twice per SessionStart, so a stderr warning appears twice, as the old error did.
- Tests. New: `internal/hook/ezkeys_test.go` (2), `ez_app_keys_test.go` `TestEzAppKeysAgentRunKeysLoad`, `TestEzAppKeysUnknownKeyWarnsAndSurvivesARewrite`, `TestEzAppKeysMalformedFilesAreStillRefused`.

## Existing tests changed (the owner's direction is the brief, item 2: an unknown key gives a warning and the file is still read)

- `ez_app_keys_test.go` `TestEzAppKeysOtherKeysAreStillRefused` replaced: the duplicate-key and second-document cases moved to `TestEzAppKeysMalformedFilesAreStillRefused`; the unknown-key cases are now `TestEzAppKeysUnknownKeyWarnsAndSurvivesARewrite`.
- `accounts_key_test.go` `TestAccountSelectionBesideAnUnknownKeyIsRefused` became `...IsRead` (one warning, `BudgetShow` reads); the `"unknown top key"` row of `TestUnknownSettingsKeysAreStillRefused` removed (covered by the test above).
- `store_test.go` `TestInvalidDocumentsWriteNothing`: the `{"unknown":1}` case removed (same).
- `internal/cli/settings_choke_test.go`: walk table lists `ReadHookLevelsWarn`, `ResolveHookLevelWarn`, `Document.MarshalJSON` as reviewed exports of store.go, and the read-only Store construction exemption moved from `ReadHookLevels` to `ReadHookLevelsWarn` (which `ReadHookLevels` now calls). This table is forced by the changed code (runbook rule 13).

## NOT VERIFIED

- A real SessionStart in a live grid-home holding the app's settings.json (the r4 J-1 case) was not run; the hook tests use the same file shape through the real hook entry point.
- App-side behaviour on a file the plugin rewrote beside the new keys was not run (the plugin keeps each app key's value but re-indents with two spaces, as before).
