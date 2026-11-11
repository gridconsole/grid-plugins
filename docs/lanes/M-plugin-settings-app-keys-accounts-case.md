# M-plugin-settings-app-keys-accounts-case

Test-only lane on top of C-fh-plugin-settings-app-keys (head cde7af0b).

## Proof table

| Row / rule | Result |
|---|---|
| 1. Entry at accounts_key_test.go:87 leaves TestUnknownSettingsKeysAreStillRefused; other five cases stay | HOLDS: `go test -count=1 ./internal/settings/` exit 0 |
| 1. Test now covering that input with the new answer | HOLDS: neither ez_app_keys_test.go test reads that exact input, so `TestAccountsWithAccountSelectionIsAcceptedAndKept` (accounts_key_test.go) takes `{"accounts":[],"accountSelection":{"mode":"use","acct":"","fullAt":95}}`: Effective reads, BudgetShow does not refuse, SetSkill rewrite keeps accountSelection DeepEqual on a UseNumber decode |
| 2. accountSelection beside an unknown key is refused, naming it | HOLDS: `TestAccountSelectionBesideAnUnknownKeyIsRefused` (error must contain `unknown key "unknown"`); also ez_app_keys_test.go:226 |
| 3. `go test -count=1 ./internal/settings/ ./internal/cli/` | HOLDS, exit 0: `ok .../internal/settings 1.084s`, `ok .../internal/cli 48.917s` |
| 4. `make lint` | HOLDS, exit 0 (after gofmt of the test file) |
| 4. `GOOS=windows go vet ./internal/settings/` | HOLDS, exit 0 |
| Importers of settings (go list): account, cli, doctor, guard, hook, settings | HOLDS: `go test -count=1` on all six, exit 0 |
| Windows and macOS builds | HOLDS: `GOOS=windows go build ./...` and `GOOS=darwin go build ./...`, exit 0 |
| New tests fail on base | HOLDS: with json.go, store.go, claude_status.go from before the C-fh change: `accounts_key_test.go:92: accounts with accountSelection must read: settings.json: unknown key "accountSelection"; allowed: agents, skills, hooks, budget, accounts`; the refusal test fails there too (`:109`, names accountSelection, not unknown). Files restored; tree clean |
| No existing test edited to pass, no guard added | HOLDS: only the map entry was removed (covered above); no production file touched |
| *RealPlugin* tests | Not run: this lane touches no plugin-binary behaviour |
| 5. Diff names only Touches files | see `git diff origin/lane-C-fh-plugin-settings-app-keys --stat` in the report |

## What changed
- internal/settings/accounts_key_test.go: removed the `accountSelection` map entry; added the two tests above; added the `reflect` import.
- This doc.

## NOT VERIFIED on a live screen
Nothing here is UI.
