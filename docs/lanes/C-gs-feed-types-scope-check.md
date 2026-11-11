# C-gs-feed-types-scope-check

| Row | Status | Proof |
|---|---|---|
| 1 no scope: same JSON, exit 0 | HOLDS | `go test -count=1 -run TestGsTypes ./internal/cli/` exit 0, subtest "no scope prints the list" (byte compare vs contract.PostTypes) |
| 2 `--task T-99 --json` same exit as read, `no_such_task` | HOLDS | subtest "unknown task", same run |
| 3 `--project nope --json` `no_such_project`, same exit as read | HOLDS | subtest "unknown project" |
| 4 shorthand `types T-99` stderr "grid feed: ", empty stdout; existing task prints list | HOLDS | subtests "shorthand unknown task", "existing task prints the list" |
| 5a `go test -count=1 ./internal/cli/` | HOLDS | exit 0 (ok 49.8s) |
| 5b importers of internal/cli (go list): cmd/grid, lib | HOLDS | `go test -count=1 ./internal/cli/ ./cmd/grid/ ./lib/` ok (cmd/grid has no tests) |
| 5c `make lint` | HOLDS | exit 0 after gofmt of the two walk files |
| 5d `-run TestGgUnknown` with the two entries removed | HOLDS | exit 0 (ran together with TestGsTypes, ok) |
| 5e diff names only feed.go, new test, two walk tests | HOLDS | `git diff --stat` (plus new file and this report) |
| Windows/macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin ...` exit 0 |
| New tests fail on base | HOLDS | with feed.go stashed: `--- FAIL: TestGsTypes/unknown_task`, `/unknown_project`, `/shorthand_unknown_task` (code 0, list printed) |
| No guard/gate/cap on agents; nothing deleted | HOLDS | only a scope lookup in `types`; walk entries removed as the brief directs |
| RealPlugin tests | n/a | no row names one |

## What changed
- internal/cli/feed.go: `types` resolves --task/--project/shorthand through feedStore and refuses through fail; no scope prints the list with no lookup.
- internal/cli/gs_feed_types_scope_test.go: TestGsTypes.
- Walk tests: `feed types` removed from ggTaskKnownOpen, its comment, and ggProjectKnownOpen.

## NOT VERIFIED on a live screen
No UI change; nothing needs a live screen.
