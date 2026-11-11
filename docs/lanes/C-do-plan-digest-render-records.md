# C-do-plan-digest-render-records

Base: 714b5680b993460c31ca204dcd1eedbd72145b5e (lane-C-do-plan-digest-render-records with the receipts branch merged in).

## Proof table

| Row | Result |
|---|---|
| 1. New test fails on base | HOLDS. With only the new test file added on base 714b5680, `go test -count=1 ./internal/cli/ -run TestPlanDigestRenderRecords` exit 1: `plan_digest_render_records_test.go:62: digest-and-stale step 7 exit = 2, want 1` and `:65: digest-and-stale step 7 stderr_contains = "plan needs approval: review and approve the current plan before rendering a Job", want "already has different saved inputs"`; also `:53: legacy-reapprove step 3 records stderr "plan needs approval: ..."` and `:72: legacy-reapprove step 3 exit = 2, want 0`. |
| 2. Skip dropped: fails before the record fix, passes after | HOLDS. Before (skip dropped, records stale), `go test -count=1 ./internal/cli/ -run PlanDigest` exit 1: `TestPlanDigestRoundTrip/digest-and-stale/07-job_render_$task_1: exit = 1, want 2; stderr: job: Job J-1 already has different saved inputs; create a new Job for revised work` and `TestPlanDigestRoundTrip/legacy-reapprove/03-job_render_$task_1: exit = 0, want 2; stdout: id: J-1 / path / inputs / ok: true`. After the record fix: exit 0, `ok ... internal/cli 0.192s`, every subtest PASS. The measured answers equal the ones in the brief. |
| 3. Other render steps unchanged | HOLDS. digest-and-stale 4, legacy-reapprove 6, short-token 4 are untouched in the transcript (record exit 0) and PASS in the -v run above, now with exit and stderr checked. |
| 4. Diff names only the allowed files | HOLDS. `git diff --stat 714b5680 HEAD`: plan_digest_render_records_test.go (new), plan_digest_roundtrip_test.go, testdata/contract/plan-digest/transcript.json, plus this report. Nothing under internal/job; no CLI code changed. |
| 5. `go test -count=1 ./internal/cli/` and lint | HOLDS. `go test -count=1 ./internal/cli/` exit 0 (`ok ... 43.876s`). `make lint` (`go vet ./...`) exit 0. `gofmt -l internal/cli` empty. |
| Importers of internal/cli | HOLDS. `go list` shows cmd/grid and lib import internal/cli: `go test -count=1 ./cmd/grid ./lib` exit 0 (cmd/grid has no test files, lib ok). The package's non-test code did not change. |
| Windows and macOS builds | HOLDS. `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0. |
| New tests in a new file, named `TestPlanDigestRenderRecords*` | HOLDS. internal/cli/plan_digest_render_records_test.go; no shared table touched. |
| No existing test edited to pass | HOLDS. The only edit to an existing test is the render skip the brief names; no assertion was loosened and none deleted. |
| No guard, gate, cap or hold added | HOLDS. Test and fixture only. |
| No private paths, hosts or emails in the diff | HOLDS. `TestPlanDigestFixturePortable` passes. |

## What changed

- New `internal/cli/plan_digest_render_records_test.go`: three tests that load the transcript. No `job render` step records "plan needs approval"; digest-and-stale step 7 records exit 1 and stderr_contains "already has different saved inputs"; legacy-reapprove step 3 records exit 0 with no stderr.
- `internal/cli/plan_digest_roundtrip_test.go`: removed the `render` variable, its comment and both `!render &&` conditions, so render steps check exit and stderr like every other step.
- `testdata/contract/plan-digest/transcript.json`: digest-and-stale step 7 is now exit 1, stderr_contains "already has different saved inputs"; legacy-reapprove step 3 is now exit 0 with no stderr_contains.

## Not touched

Update mode (UPDATE_PLAN_DIGEST_FIXTURE=1) still does not rewrite stderr_contains; the brief limits the test change to the render skip.

## NOT VERIFIED on a live screen

Nothing here is UI; no live screen is needed.

DONE
