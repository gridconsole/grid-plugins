# C-fix-plugin-oneof-branch-message

A oneOf depth tie now reports the branch whose required keys the body holds most. A one-key branch (the external `ref` form) never wins a tie for a body with more than one key.

## Proof table

| Row | Result |
|---|---|
| On a depth tie, report the branch whose required keys the body matches most; never the one-key `ref` branch for a body with more than one key | HOLDS: internal/contract/schema.go oneOf loop and `branchMatch` |
| A result body missing `knowledge` says `missing required knowledge` | HOLDS: `go test -count=1 ./internal/contract -run TestOneOf -v` exit 0, `TestOneOfTieNamesTheBranchTheBodyMatches` PASS |
| A bare `{"ref":...}` body still validates where the store writes it | HOLDS: `TestOneOfBareRefBodyStillValidates` PASS; internal/feed tests exit 0 |
| New test fails on base | HOLDS, with schema.go stashed: `--- FAIL: TestOneOfTieNamesTheBranchTheBodyMatches` / `oneof_test.go:50: want missing required knowledge, got $.body: oneOf matched 0 branches: $.body: missing required ref` |
| New test passes on the lane | HOLDS: `--- PASS: TestOneOfTieNamesTheBranchTheBodyMatches`, exit 0 |
| Pinned messages in record_exit_test.go:16 and the feed, mcp, orchestrator rejection tests | HOLDS, no edit needed: those tests post an empty body `{}`. It has no keys, so the tie still reports `missing required ref` and the pins are unchanged. All pass unedited |
| `make lint` | HOLDS: exit 0 (`go vet ./...`); `gofmt -l internal/contract` empty |
| Tests of internal/contract and its importers | HOLDS: exit 0 for contract, cli, feed, mcp, orchestrator, artifact, brain, braincmd, guard, hook, intent, job, job/jobcmd, job/lint, land, lease, migrate, question, session, settings, signal, task, updates (no test files), usage, where, workflow (importer list from `go list -f '{{.ImportPath}}: {{.Imports}}' ./...`) |
| Windows build | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS: `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| Walk tests `TestHsIpWalksEveryCommand\|TestIlGuestSiteList`, `./internal/job/lint/` | HOLDS: both ok, exit 0 |
| `git fetch origin && git merge origin/main` before the last run | HOLDS: already up to date |
| Whole-module test, cross-repo set | not run, not asked (orchestrator at landing) |
| No guard, gate, cap or hold added | HOLDS: only the choice of error message changed; validation accepts and rejects the same bodies |

## What changed

- `internal/contract/schema.go`: in the oneOf/anyOf loop the branch to report is chosen by depth, then by the count of its required keys the body holds. On an equal count a narrow one-key branch no longer replaces an earlier branch when the body has more keys. New helper `branchMatch`.
- `internal/contract/oneof_test.go`: two new tests.

## NOT VERIFIED on a live screen

Nothing here needs a screen. An empty `{}` body still reports `missing required ref`, which is outside this brief.
