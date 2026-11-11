# D-budget-accounts (plugin repo): agent_budget accepts the `accounts` key the app's account reads write into settings.json

Changed: `internal/settings/json.go`, `internal/settings/store.go`, `internal/settings/accounts_key_test.go` (new), `docs/settings.md`, this file. No app file changed. Added no guard, gate, cap, hold or blocking hook.

| Row | Status | Proof |
|---|---|---|
| 1 Name the strict decode and the app writer | HOLDS | Strict decode (base): `internal/settings/json.go:72` `object(data, []string{"agents","skills","hooks","budget"})` calls `allowed("key", key, keys)` at `json.go:32`, so any other top-level key fails with `unknown key "accounts"; allowed: agents, skills, hooks, budget`. It is reached from `readDocument` `store.go:78` (`Store.Effective`, the budget `load` at `budget.go:315`, every `Set*`), and so from `grid agent budget show`, which the app's agent_budget op serves; that read error is the 502. Writer in the app (read from `/srv/gridwork/ref/app.git` main, not changed): `internal/slices/settings/store.go:185` `Sync` commits the key at `store.go:274` (`s.commit(doc, "accounts", stored)`); `store.go:391` `save` with `actOn` (`store.go:408`) adds and removes records; the account ops in `internal/slices/agentaccounts` (`records.go`, `http.go:231`) call `Sync`. The record shape is `{id,kind,name,email,dir,added}` (`store.go` `Account`). The app's own `agentreads/http.go:368` deletes `accounts` before serving `saved`, because it holds emails. |
| 2 Plugin reads settings.json with `accounts`; unknown keys elsewhere keep today's behaviour | HOLDS | `json.go:72` allows `accounts`; `json.go:77-83` requires a JSON array and keeps the raw bytes in the new `Document.Accounts` (`store.go:28`, `json:"accounts,omitempty"`). The budget does not need the records, but all three writers re-marshal `Document` (`Store.update` `store.go:246`, budget overlay `budget.go:578`, undo fallback `budget.go:896`), so an accept-and-ignore fix would have erased the person's records on the next Apply; carrying the raw array through `Document` keeps them. `merge` does not copy it, so `Effective` and the budget read never show the records (emails). Still refused, pinned by `TestUnknownSettingsKeysAreStillRefused`: `{"surprise":1}`, `accountSelection` and the other keys the app keeps, `accounts` that is not an array or is null or repeated, an unknown agent name. |
| 3 Go test with the `accounts` block that fails before the fix | HOLDS | `internal/settings/accounts_key_test.go`: `TestSettingsWithAccountsKeyReadsAndShowsBudget` and `TestBudgetApplyUndoAndSetKeepTheAccountsKey` use the app's own record shape. On base (fix stashed, test file present), `go test ./internal/settings/ -run 'AccountsKey\|UnknownSettingsKeys' -count=1` exit 1: `accounts_key_test.go:40: a settings.json with the accounts key must read: settings.json: unknown key "accounts"; allowed: agents, skills, hooks, budget` and `accounts_key_test.go:64: apply on a home with accounts: settings.json: unknown key "accounts"; allowed: agents, skills, hooks, budget`. On the lane, the same command exit 0, three tests pass (`-v`). The third test passes on both: it pins today's behaviour. |
| 4a `go test ./...` | HOLDS | `go test ./... -count=1` exit 0, every package `ok`. |
| 4b `make lint` | HOLDS | `make lint` (`go vet ./...`) exit 0. |
| Packages that import `settings` | HOLDS | `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... \| grep grid-plugins/internal/settings` gives: `cmd/grid`, `internal/cli`, `internal/doctor`, `internal/guard`, `internal/hook`, `internal/packaging`, `internal/packaging/cmd`, `internal/settings`. `go test ./cmd/grid/ ./internal/cli/ ./internal/doctor/ ./internal/guard/ ./internal/hook/ ./internal/packaging/... ./internal/settings/ -count=1` exit 0 (cli, doctor, guard, hook, packaging, settings `ok`; cmd/grid and packaging/cmd have no test files). |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| New tests fail on base, pass on lane | HOLDS | Row 3; the two failing lines are pasted there. |
| No existing test edited or deleted | HOLDS | `git diff origin/main..lane-D-budget-accounts --stat` touches no `_test.go` except the new file. |
| No guard, gate, cap, hold, blocking hook | HOLDS | The change only widens one read and keeps data on write; nothing blocks an agent. |
| No personal path, host or email in the public repo | HOLDS | Test data uses `ann@example.com`, `~/.claude-ann`, `~/.codex-lab`. |
| Small commits, one-sentence messages, no attribution lines | HOLDS | Two code commits (fix, test) plus the doc and report commit; no `Co-Authored-By`, no session line, as the owner's rule says. |
| Runbook read | HOLDS | `/srv/gridwork/tmp/hand/runbook-reviewer.txt` read; each acceptance row has a command or a file:line above. |

## What changed

- `internal/settings/json.go`: `decode` accepts the top-level key `accounts` when its value is a JSON array and keeps it verbatim in `Document.Accounts`; any other value is refused as before.
- `internal/settings/store.go`: `Document.Accounts json.RawMessage` (omitted when absent), not merged into `Effective`.
- `internal/settings/accounts_key_test.go`: three tests (read with the key; Apply, Set and Undo keep the records; other unknown keys and a bad `accounts` value stay refused).
- `docs/settings.md`: one paragraph naming the extra key.

## Open items

- The same app-written file can also hold `notifications`, `models`, `usageRules`, `windowRules`, `workspace`, `accountSelection` and `hiddenProjects` (app `internal/slices/settings/check.go:14`, `store.go:423`). The brief names only `accounts` and says other unknown keys keep today's behaviour, so they are still refused: a home where the person has saved any of those still answers 502 on agent_budget. That needs the owner's decision (accept the app's key list, or have the plugin keep its own file); I did not widen it.
- `accounts` content is not validated beyond being an array (the app owns the record shape and drops bad rows itself).
- The hook-level read (`ReadHookLevels`, used by guards) shares `readDocument`, so a home with `accounts` no longer fails it either; `TestSettingsWithAccountsKeyReadsAndShowsBudget` covers that call.

## NOT VERIFIED on a live screen

- The dev UI answer of `agent_budget` on a real home with `accounts` (200 instead of 502): the orchestrator does the live check; here the settings package is proven by test, not through the served op.
