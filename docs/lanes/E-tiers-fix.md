# E-tiers-fix proof table

| Row | Status | Evidence |
|---|---|---|
| 1. missing-tier.json removed | HOLDS | Deleted, not moved: `contract/fixtures/fmt-origin/valid/session.json` is byte-identical (`{"kind":"session","id":"synthetic-session"}`) and already covers an origin without tier. |
| 2. schema description matches formats.md | HOLDS | contract/schemas/fmt-origin.json:4 now says the decoder ignores a legacy `tier` and no import upgrade path is needed; matches contract/formats.md:178. |
| 3. go test ./... | HOLDS | `go test ./...` exit 0, no FAIL lines; `go test ./internal/contract/` ok. |
| 3. make lint | HOLDS | `make lint` (go vet ./...) exit 0. |
| New tests fail on base | N/A | No new tests; the existing internal/contract TestFixtures failed on base because missing-tier.json had become a valid origin, and now passes. |
| No guard/gate added | HOLDS | Fixture deletion and a description text only. |
| Packages importing changed ones | HOLDS | Only data files changed (no Go package); the whole-module `go test ./...` passed. |
| Windows/macOS builds | NOT DONE | No Go code changed; not run. Orchestrator runs these at landing. |

## What changed
- Deleted contract/fixtures/fmt-origin/invalid/missing-tier.json.
- Reworded the description in contract/schemas/fmt-origin.json.

## NOT VERIFIED on a live screen
Nothing here is a UI change.
