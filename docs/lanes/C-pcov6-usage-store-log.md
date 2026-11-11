# C-pcov6-usage-store-log

Test-only lane: pins the usage store's task checks, bad log lines and result-log edges. Two new files, no behaviour change.

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. Coverage before | HOLDS | `go test -count=1 -coverprofile=/srv/gridwork/tmp/C-pcov6-usage-store-log-before.out ./internal/usage/` exit 0; `go tool cover -func=... \| tail -1` gives 84.8% (as expected) |
| 2. Task checks (usage.go:111,115,118,128,143) | HOLDS | `TestUsageStoreTaskChecks` in usage_store_log_test.go; profile blocks usage.go:110-120, 126-129, 141-144 count 1 |
| 3. Key check (usage.go:103) | HOLDS | `TestUsageStoreKeyCheck`; exact text asserted, Show then empty; block usage.go:102.2-104.1 count 1 |
| 4a. Post with `usage` as a file (usage.go:153) | HOLDS | `TestUsageStorePostUsageIsFile`, prefix `open `; usage.go:151-154 count 1 |
| 4b. Show with `usage` as a file (fold.go:106) | HOLDS | `TestUsageStoreShowUsageIsFile` in usage_store_log_unix_test.go (`//go:build unix`); fold.go:105-107 count 1 |
| 5. Log is a folder (fold.go:123) | HOLDS | `TestUsageStoreLogIsFolder` in the unix file, prefix `read `; fold.go:122-124 count 1 |
| 6. Bad log lines (fold.go:132,162,165) | HOLDS | `TestUsageStoreBadLogLines` (two-objects, version, at); each bad line built from a posted line by unmarshal into map, change one field, marshal; exact `usage.jsonl line 1: ...` asserted; fold.go:130-133, 161-166 count 1 |
| 7. ParseResult edges (result.go:55,83,103) | HOLDS | `TestUsageParseResultEdges` (read-error via `errors.Is`, overflow exact text, Model `m1`); result.go:54-55, 82-84, 101-103 count 1 |
| 8. Coverage after and gates | HOLDS | after profile: 91.5% (before 84.8%). `go test -count=1 ./internal/usage/` exit 0; `make lint` (go vet ./...) exit 0; `GOOS=windows go vet ./internal/usage/` exit 0; `GOOS=darwin go vet ./internal/usage/` exit 0 |
| Importers of usage | HOLDS | `go list` import scan: only `internal/cli` imports `internal/usage` (`cmd/grid` and `lib` only via cli); `go test -count=1 ./internal/cli/` exit 0 (ok 40.9s) |
| New tests fail on base | NOT APPLICABLE | the lane changes no non-test code. The blocks had count 0 and the tests only pin existing behaviour, so they pass on base too. There is no failing line to paste |
| No existing test edited or deleted | HOLDS | `git diff --stat origin/main` lists only the two new test files and this report |
| No guard, gate, cap or hold added | HOLDS | test-only |
| No Unix-only call without a build tag | HOLDS | no chmod, Mkfifo or syscall; 4b and 5 are in the `//go:build unix` file; OS error text asserted by prefix only (`open `, `read `) |
| Lane prefix on helpers | HOLDS | `usErr`, `usValid`, `usLogPath`, `usWantErr`, `usWantPrefix`, `usBadLine`; test funcs `TestUsageStore*` and `TestUsageParseResultEdges` |
| approval.json / jobtest Approve | HOLDS | not touched |

Profile lines (after), one grep: all blocks of rows 2-7 have count 1. Out of scope usage.go:133 stays at count 0, as the brief says.

## What changed
- internal/usage/usage_store_log_test.go (new)
- internal/usage/usage_store_log_unix_test.go (new, `//go:build unix`)
- docs/lanes/C-pcov6-usage-store-log.md (this report; the runbook requires it)

No bug found.

## NOT VERIFIED on a live screen
Nothing here is UI. The Windows and macOS builds were only vetted (`go vet`), not run.
