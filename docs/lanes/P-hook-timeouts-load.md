# P-hook-timeouts-load

Proof table (head: see the report line; one code commit 9adfbf0f, no merge needed, main was up to date).

| Row | Result |
|---|---|
| A1 SessionStart, SubagentStart, PreCompact, SessionEnd, PostToolUseFailure are 30 in internal/hook/events.go and hooks/hooks.json; Stop and SubagentStop stay 60 | HOLDS: events.go:40-56, hooks/hooks.json (diff in commit 9adfbf0f, `git show 9adfbf0f`) |
| A2 zj_timeouts_test.go pins the new values; `go test ./internal/hook/ ./internal/packaging/` | HOLDS, exit 0: `ok internal/hook 38.297s`, `ok internal/packaging 19.612s`. On base events.go it fails: `SessionStart timeout 10, want 30` (and SubagentStart, PreCompact, SessionEnd, PostToolUseFailure) |
| A3 events.go comment cites load above 600, 16 Nov, report 2026-11-16T1352Z Issue 1 | HOLDS: events.go:34-38 |
| A4 no behaviour change, no new guard or cap | HOLDS: diff is five numbers, a comment, the test table |
| A5 doctor 'plugin hooks' check ok on the new hooks.json | HOLDS: `go test ./internal/doctor/ ./internal/settings/` exit 0 (ok 12.680s, ok 4.844s); packaging layout test compares hooks.json with the table |
| Importers of internal/hook (go list) | HOLDS: internal/cli, cmd/grid; `go test ./internal/cli/ ./cmd/grid/` exit 0 (cli ok 190.487s, cmd/grid no test files) |
| make lint | HOLDS, exit 0 (go vet ./...) |
| Windows build `GOOS=windows CGO_ENABLED=0 go build ./...` | HOLDS, exit 0 |
| macOS build `GOOS=darwin CGO_ENABLED=0 go build ./...` | HOLDS, exit 0 |
| Walk test `go test ./internal/job/lint/` | HOLDS: ok 0.385s |
| Walk tests TestHsIpWalksEveryCommand, TestIlGuestSiteList | HOLDS: inside `go test ./internal/cli/` (ok), not run separately |
| git fetch origin && git merge origin/main | HOLDS: Already up to date |
| realplugin guest walk tables | not run, not asked (app repo only) |
| Whole-module test | not run, not asked (orchestrator at landing) |

## What changed
Five hook events moved from a 10 s to a 30 s timeout, in the events table and the checked-in hooks/hooks.json, with the test and the comment updated.

## NOT VERIFIED on a live screen
Nothing here is UI. Not seen: a real SessionStart under load above 600.

DONE
