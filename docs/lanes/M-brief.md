# M-brief: bring lane-E-verdict-2-brief up to date with main

Merged origin/main (d8f1a550) into the lane. No later main commit existed at the final `git fetch`.

## Proof table

| Row | Result |
|---|---|
| 1. Merge origin/main, resolve every conflict, both intents survive | HOLDS. `git merge-base --is-ancestor origin/main HEAD` exit 0. Files below. |
| 2. `go test ./...` 0 fail | HOLDS. `go test ./...` exit 0, every package ok. |
| 2. `make lint` | HOLDS. `make lint` (go vet ./...) exit 0. |
| 2. `GOOS=windows go build ./...` | HOLDS. exit 0. |
| 2. `GOOS=darwin go build ./...` | HOLDS. exit 0. |
| Importers of changed packages | HOLDS. The whole-module `go test ./...` covers cli, settings, hook, guard and every importer; exit 0. |
| New tests fail on base | No new tests in this lane; the merge only resolves conflicts. The E-verdict-2-brief tests are unchanged. |
| No guard, gate, cap or hold added | HOLDS. Nothing added. |
| One-sentence commits, no attribution lines, no personal paths | HOLDS. One merge commit, one sentence. |

## Conflicts and what was kept

- `internal/cli/settings.go:27` (usage text): lane removed brief-lint from the text, main changed only-writer to default warn. Kept both: "host-guard and plan-guard default to block; only-writer defaults to warn."
- `internal/settings/catalog.go:81` (`defaultLevel`): lane dropped brief-lint, main dropped only-writer from the block list. Kept both: `host-guard`, `job-lint`, `plan-guard`, `stop-check` block; all else warn.
- `internal/cli/testdata/settings/hooks.json`: kept no `brief-lint` key (lane) and `only-writer: warn` (main).
- `internal/hook/hold_hook_test.go`: main deleted the file because the plan hold feature was removed. The lane had only deleted `TestHoldKeepsAGuardDeny` from it (brief guard was its only denying guard). Resolved as deleted. Nothing to move: the hold is gone, so no test needs to cover a hold keeping a guard deny.

## NOT VERIFIED on a live screen

Nothing here is UI.

## Open items

- `docs/settings.md:110,126-127` still lists brief-lint (stale before this merge, outside the merge's needs); left as the E-verdict-2-brief report already notes.
