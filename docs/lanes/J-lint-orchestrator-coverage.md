# J-lint-orchestrator-coverage

An orchestrator Job no longer gets `job.acceptance-coverage` or `job.scope-narrowing`, so rendering its first Job no longer fails lint on the person's own words.

## Proof table

| Row | Result |
| --- | --- |
| 1. role orchestrator skips both rules, other rules still run | HOLDS. `internal/job/lint/lint.go:299-303` returns before the coverage block; `TestOrchestratorJobSkipsPlanWordCoverage` in `orchestrator_test.go` checks neither rule is reported and that `job.on-blocked` still is. `go test ./internal/job/lint/` exit 0 |
| 2. worker Job still gets both findings | HOLDS. `TestWorkerJobStillGetsPlanWordCoverage` (role `writer`: warning `job.acceptance-coverage` and error `job.scope-narrowing`); the existing `TestAcceptanceCoverageIsWarning` still passes. exit 0 |
| 3. `go test ./internal/job/lint/ ./internal/job/jobcmd/` 0 fail | HOLDS. `ok .../lint`, `ok .../jobcmd`, exit 0 |
| New tests fail on base, pass on lane | HOLDS. On base (lint.go stashed) `TestOrchestratorJobSkipsPlanWordCoverage` fails: `orchestrator_test.go:26: orchestrator Job reported job.acceptance-coverage: [{Rule:job.scope-narrowing Line:17 Severity:error ...` and `:30: orchestrator Job has errors`; passes on the lane |
| No existing test edited or deleted | HOLDS. `git diff origin/main..HEAD --stat` shows only lint.go and new files |
| New tests in a new file named after the subject | HOLDS. `internal/job/lint/orchestrator_test.go` |
| Edit stays inside the coverage block (shared with F-review-levels-drop at lint.go:137) | HOLDS. One hunk, before `nouns := contentWords(itemText)` |
| Packages that import the changed package | HOLDS. `go list` importers of `internal/job/lint`: `internal/job/jobcmd`, itself; both tested above |
| Lint | HOLDS. `make lint` (`go vet ./...`) exit 0; `gofmt -l internal/job/lint` prints nothing |
| Windows and macOS builds | HOLDS. `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0; `go vet` of both packages with each GOOS exit 0 |
| No guard, gate, cap or hold added | HOLDS. The change removes two findings for one role; it blocks nothing |
| No personal paths, hosts or emails; no attribution lines | HOLDS |

## What changed

- `internal/job/lint/lint.go`: in `validate`, after every other rule has run, a Job whose `role` is `orchestrator` (case-insensitive, trimmed) returns before the plan-word coverage and out-of-scope narrowing checks. The coverage block is the last thing in `validate`, so no other rule is skipped.
- `internal/job/lint/orchestrator_test.go` (new): the two tests above.

## NOT VERIFIED on a live screen

Rendering a real orchestrator Job from `intent.md#request` through `grid job render` (jobcmd/command.go:127) was not run; the orchestrator does the live check. The tests cover lint only, with no resolver, so the plan text is the title rather than the intent.item pointer text. The skip does not depend on where the text came from.
