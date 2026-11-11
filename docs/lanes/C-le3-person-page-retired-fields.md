# C-le3-person-page-retired-fields

Base failure (before the edit), `go test -count=1 ./contract -run TestT12Person`:
- t12_person_job_test.go:184: rows for keys the body lacks: [max_rounds review]
- t12_person_job_test.go:202: review: page says "L0", body has "<nil>"
- t12_person_job_test.go:202: max_rounds: page says "1", body has "<nil>"

| Row | Result |
|---|---|
| `go test -count=1 ./contract -run TestT12Person` | HOLDS, exit 0 (`ok .../contract`) |
| `go test -count=1 ./contract/ ./internal/question/ ./internal/job/ ./internal/cli/` | HOLDS, exit 0, all four ok |
| `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0 |
| `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0 |
| `make lint` | HOLDS, exit 0 |
| `make cross` | HOLDS, exit 0 (Windows and macOS builds) |
| Importers of changed packages | Only a test file and a doc changed, no Go package API; the contract package is the changed package and its test is run above |
| Step 3: other pages | Other `max_rounds` hits are schemas, fixtures and old lane reports; no test fails on them, so left alone |
| New tests | None added; the existing TestT12Person* fail on the base lane and pass here |
| Existing test edited | Only the fixed-values key list at contract/t12_person_job_test.go:194, as the brief directs (owner decision 21:4x) |

## What changed
- contract/docs/person-job.md: removed the `review` and `max_rounds` rows.
- contract/t12_person_job_test.go:194: dropped "review" and "max_rounds" from the fixed-values list.

## NOT VERIFIED on a live screen
Nothing here is UI.
