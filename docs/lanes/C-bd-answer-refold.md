# C-bd-answer-refold: a person's answer and a plan approval refold the published state at once

Base: plugin main cdbef50c. Repo: plugin.

## Base check

`git grep -n "RefoldPublished" -- internal/cli` at cdbef50c:

```
internal/cli/feed.go:178:	if _, err := store.RefoldPublished(); err != nil {
internal/cli/orchestrator.go:411:	if _, err := refold.RefoldPublished(); err != nil {
```

orchestrator.go:343 `return runPersonApproval(...)`, :347 `record, err := orchestrator.AnswerPerson(...)`, task_write.go:308 `err = recordApproval(...)` followed by `return taskWriteResult(...)`. None refolded, so the brief held.

## Proof table

| Row | Result |
|---|---|
| 1 `TestBdLeasedAnswerRefolds` | HOLDS: `go test -count=1 -v ./internal/cli -run TestBd` exit 0, `--- PASS: TestBdLeasedAnswerRefolds`. Red on base: `bd_answer_refold_test.go:96: state.json shows J-2 as "posted", the fold has "accepted"` |
| 2 `TestBdPlanHashResultRefolds` | HOLDS: same run, PASS. Red on base: `bd_answer_refold_test.go:105: state.json shows J-2 as "posted", the fold has "accepted"` |
| 3 `TestBdApproveRefolds` | HOLDS: same run, PASS. Red on base: `bd_answer_refold_test.go:114: state.json shows J-2 as "posted", the fold has "accepted"` |
| 4 `TestBdRefoldErrorKeepsTheAnswer` | HOLDS: same run, PASS (exit 0, one record id line on stdout, a `refold: ` stderr line, J-2 accepted in the fold). Red on base: `bd_answer_refold_test.go:140: stderr has no line starting "refold: ": ""` |
| 5 TestBd PASS lines; `go test -count=1 ./internal/cli ./internal/feed ./internal/orchestrator` | HOLDS: all four TestBd tests print `--- PASS:`; the three packages plus `./cmd/grid ./lib` exit 0 (cli 37.0s, feed 10.2s, orchestrator 87.4s, lib 7.2s). xh_post_refold_test.go, approve_job_test.go, approve_record_test.go, approve_digest_test.go and person tests are not edited. |
| 6 `make lint` | HOLDS: exit 0 (go vet ./...) |
| 6 `make cross` | HOLDS: exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| 6 `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS: exit 0 |
| 6 `go test -count=1 ./internal/job/lint/` | HOLDS: exit 0 |
| 6 `git diff main --stat` | HOLDS for code: orchestrator.go, task_write.go, bd_answer_refold_test.go. This lane doc is the one extra file, as the runbook asks. |
| Packages importing internal/cli (go list) | `cmd/grid`, `lib`; both tested above |
| Walk tests: plugin set | HOLDS: TestHsIpWalksEveryCommand\|TestIlGuestSiteList exit 0; internal/job/lint exit 0 |
| `git fetch origin && git merge origin/main` before the last run | HOLDS: `Already up to date.` (a new origin branch, lane-C-bt-removed-task-ids, is not main) |
| App-repo walk tests, `make realplugin`, `make webui-test` | not run, plugin repo lane |
| No guard, gate, cap or hold on agents | HOLDS: none added |

## What changed

- internal/cli/orchestrator.go: the four refold lines lifted into `refoldAfterWrite(env, store)` (same effect at the generic write path); called after a good `AnswerPerson`, and after `runPersonApproval` when it returns 0.
- internal/cli/task_write.go: `runApprove` calls `refoldAfterWrite` after `recordApproval` returns nil, when the store from `planPersonJob` has a task dir (`planPersonJob` returns an empty store when the feed identity cannot be read; refolding that would act on no task).
- internal/cli/bd_answer_refold_test.go: the four tests. They read state.json from the file and compare needs-you.json with the fold.

Exit codes and stdout are unchanged. A refold error prints `refold: <err>` on stderr; the answer stays.

## Notes

- Rows 1-3 use the leased path (the harness's GRID_LEASE_EPOCH), as the brief says. An unleased answer posts an `answer` record and the Job stays open until the holder's pass; the refold shows the same there and the tests do not cover it.
- Rows assert on the Job's status in state.json and on needs-you.json equal to the fold. NeedsYou lists open question ids only, so "the item is gone" is not asserted for a Job.

## NOT VERIFIED on a live screen

The app's Approve button and answer form were not run; the plugin side is proved through the CLI in-process.
