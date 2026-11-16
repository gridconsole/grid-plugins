# P-oj-test-serial-pre

TestOjSoTwoSessionsOwnChanges runs its two PreToolUse calls one after the other. The two PostToolUse calls stay concurrent.

Why: since P-r23-hook-levels the Pre takes the session lock without waiting and skips history on a busy lock (internal/hook/history.go:143, :191-194; accepted in docs/lanes/P-r23-hook-levels.md:13). Two Pres started at once could log "history skipped: session lock busy", which the test treats as a failure.

## Proof table

| Row | Result |
|---|---|
| 1. `go test ./internal/hook -run TestOjSoTwoSessionsOwnChanges -count=20 -race` | HOLDS, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/hook  1.302s`. Also `-count=500 -race -cpu 1,2,4`: `ok ... 12.019s`. |
| New test fails on base, passes on lane | HOLDS. Base (same command, count=500): many `--- FAIL: TestOjSoTwoSessionsOwnChanges` with `oj_snapshot_outside_lock_test.go:220: PreToolUse of oj-s1: "grid hook: history skipped: session lock busy\n"`, `:225: after the Pre, oj-s1 has a snapshot under oj-use-1: false`, `:239: 1 Bash lines in history.md, want 2`. Lane: `ok`. |
| 2. history.md has one line per writer; each Post deletes only its own snapshot | HOLDS, assertions unchanged. `strings.Count(h, c.line) != 1` -> "want 1" (test file, the Post block); `strings.Count(h, "via Bash") != len(calls)` -> "want 2"; `len(a.Snapshots) != 0` -> "keeps snapshots after its Post"; the per-Pre check `_, has := a.Snapshots[other.use]; has != (other == c)` is kept. |
| 3. `git diff main --stat` | HOLDS: only `internal/hook/oj_snapshot_outside_lock_test.go` and `docs/lanes/P-oj-test-serial-pre.md`. No non-test file changed. |
| 4a. `go test -count=1 ./internal/hook/...` | HOLDS, exit 0: `ok .../internal/hook 4.865s` |
| 4b. importers of internal/hook (go list): internal/cli, internal/packaging, internal/packaging/cmd (via packaging), cmd/grid, lib (via cli) | HOLDS, exit 0: cli `ok 81.470s`; packaging `ok`; packaging/cmd `ok`; lib `ok 17.213s`; cmd/grid no test files |
| 4c. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0: `ok .../internal/cli 0.210s` |
| 4d. `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0: `ok .../internal/job/lint 0.339s` |
| 4e. `make lint` | HOLDS, exit 0 (`go vet ./...`, no output) |
| Windows and macOS builds | HOLDS, exit 0: `GOOS=windows go build ./...`, `GOOS=darwin go build ./...` |
| `git fetch origin && git merge origin/main` before the last run | HOLDS: `Already up to date.` |
| Tests named *RealPlugin* | not run, not asked |
| No guard, gate, cap or hold added; no non-test file touched | HOLDS |

## What changed

`internal/hook/oj_snapshot_outside_lock_test.go`: in TestOjSoTwoSessionsOwnChanges the Pre step is now a loop that starts each Pre and waits for it before the next. The `both` helper is kept for the two concurrent Posts. A comment says why.

## NOT VERIFIED on a live screen

Nothing here is UI.
