# C-pcov2b-contract-frontmatter-mcp-hookio

Tests only: four new internal test files, no non-test file touched.

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1 coverage before/after | HOLDS | Linux, `go test -count=1 -coverprofile=.cov/<name>.out ./internal/<pkg>/ && go tool cover -func=... \| tail -1`, exit 0 each. Profiles went in a lane-local `.cov/` (ignored `*.out`), since mktemp and /tmp are off limits here. Before: contract 92.2%, job/frontmatter 93.1%, mcp 94.0%, hookio 91.5%. After: contract 94.9%, job/frontmatter 100.0%, mcp 96.6%, hookio 99.2%. Each after is above its before. |
| 2 checkSubset (8 subtests) | HOLDS | `internal/contract/subset_time_test.go` `TestCheckSubsetRefusals`: format email, pattern `(`, properties child, tuple items, not, anyOf, top-level bogus, listed keywords only (nil). Passes. |
| 3 validate/types | HOLDS | `subset_time_test.go`: `TestValidateNestingLimit` ("$: schema nesting limit" at depth 101, nil at 100), `TestMatchesTypeUnknownAndRationalUnreadable`, `TestWorkerMemberSplit`, `TestStateOfIntegrated`, `TestFormatTime` (…12:00:05.000Z from +01:00 zone, ".123Z" for 123456789 ns). |
| 4 exitCode/contains | HOLDS | `TestExitCodeDropsWhatSchemasRefuse` (`""`, `2.5`, `1e300` are nil via `!= nil`; `2.0` is `2`), `TestContainsNoMatch`. |
| 5 frontmatter | HOLDS | `internal/job/frontmatter/values_test.go`: Into func and chan values -> error; alias b == a; "line 3: " and "line 2: " prefixes only. |
| 6 toolError/render | HOLDS | `internal/mcp/call_errors_test.go`: `TestToolErrorCode` (code 3 plus the error text; no code for errors.New), `TestRenderRefusesUnmarshalableValue`. |
| 7 mcp call errors | HOLDS | `TestCallErrors`, five subtests through Serve with `exchange`: no resolver, feed_read and feed_state with empty TaskID (text of feed.ErrNoTaskID), nil Session, Session error (contrast). Each checks `store.Read` has no record afterwards. |
| 8 no arguments | HOLDS | `TestToolsCallWithoutArguments`: tools/call feed_types with no "arguments" key -> not isError, text holds "types". |
| 9 hookio read/prefixFields | HOLDS | `internal/hookio/write_test.go`: `TestReadNothingToRead`, `TestReadCutInputKeepsCwdAndAgentType` (the two fields end at byte 40, the cut at 60 falls inside the "x" value; the test checks the 40). |
| 10 Decision | HOLDS | `TestDecisionContextOnly`, `TestDecisionWrite` (empty -> nil and nothing written; non-empty equals Bytes(); failing writer's error returned). |
| 11 tests/vet/lint | HOLDS | `go test -count=1 ./internal/contract/ ./internal/job/frontmatter/ ./internal/mcp/ ./internal/hookio/`: ok, exit 0. `GOOS=windows go vet` and `GOOS=darwin go vet` on the four packages: exit 0 each. `make lint` (`go vet ./...`): exit 0. |
| Importers' tests | HOLDS | `go list` importers of the four packages: artifact, brain, braincmd, cli, env, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, orchestrator, plan, question, session, signal, task, usage, where, workflow. `go test -count=1` on them plus the four: no FAIL, exit 0. |
| New tests fail on base | NOT APPLICABLE | The lane changes no code. The tests pin behaviour that exists on base, so they pass there. They add coverage of branches that had count 0. They were not run against a mutated base. |
| No existing test edited, no non-test file | HOLDS | `git diff origin/main..HEAD --stat` shows four new `_test.go` files plus this report. |
| No guard/gate/cap added | HOLDS | Tests only. |
| Commits | HOLDS | One commit with a one-sentence message and no attribution lines. |

## What changed

- `internal/contract/subset_time_test.go`
- `internal/job/frontmatter/values_test.go`
- `internal/mcp/call_errors_test.go`
- `internal/hookio/write_test.go`

Where the brief's expectation and the code differed, the code won: none differed.

## NOT VERIFIED on a live screen

Nothing here is UI. Windows and macOS: compiled only (`go vet` with GOOS=windows and GOOS=darwin). The tests were not run on those hosts. Needs a Windows run: `go test ./internal/contract/ ./internal/job/frontmatter/ ./internal/mcp/ ./internal/hookio/`.
