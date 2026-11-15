# P-r23-judge-review: a done Job that no Judge takes is named, and the docs say who posts its review

Lane branch lane-P-r23-judge-review, from main 0f360eaa. Code commits: "Name a submitted Job that no Judge took as a never_judged orphan in grid job tree." and "Say in the job contract and the orchestrator text that the app's Judge posts the review and the rework." Source: `U-chk-plugin-rev-orch.md` row 4 (the only row the brief names).

Verdict: DONE. `git log HEAD..origin/main` printed nothing after `git fetch origin`, so there was nothing to merge.

## Row 4: confirmed

Row 4 (PARTIAL): a Job whose agent exits `done` stops at `submitted`; its review `J-N.1` and any rework come from the app's Judge, and nothing in the plugin says so or names the stuck Job.

Seen on main, with the row's own check:

- `grep -rn -E "ACCEPTED|REJECTED" internal --include=*.go` (exit 0) finds four hits, all in test or fixture files: `internal/evalcmd/hjg_suitegrade_test.go:12,149`, `internal/job/source_resolve_test.go:134`, `internal/eval/home_test.go:183`. No non-test file reads a verdict.
- `internal/orchestrator/tick.go:329-330`: exit `done` leads to `s.control(j, contract.Submitted, "awaiting judgement", ...)`. `tick.go:225` and `stream.go:352-353` only print a `wake` line.
- The app's Judge acts on that wake (`/srv/gridwork/ref/app.git` main, `internal/kernel/worker/judge.go:12-31`, `hand` at `:658`, `verdict` at `:1003`): it posts the review child, accepts, or sends the Job back and posts the rework. It answers a wake with a record carrying `wake` (`judge.go:394-404`, `:962-975`).
- No orphan kind named a Job in that state: `orphanOf` (`write.go:310`) returns nil for a `submitted` Job, and `agents/orchestrator.md:28` named four kinds. Reproduced by `TestR23NeverJudgedNamesASubmittedJobNobodyJudges` on main (failing line below).
- Nothing in the app reads orphan kinds: `git grep -E "dead_wait|never_acked|no_exit|never_picked_up" main` in the app ref finds only two lane docs.

The row's fix is applied as written. The spec lines (`docs/job-contract.md:122-123`, `agents/orchestrator.md:18`) already say the Judge and the worker post the review, so no spec line shows a better fix.

## Proof table

| Row / rule | Status | Proof |
|---|---|---|
| 1. Row 4 confirmed (paste) | HOLDS | section "Row 4: confirmed" above |
| 2. A test for the fixed row, failing on main and passing on the head | HOLDS | `TestR23NeverJudgedNamesASubmittedJobNobodyJudges`, `TestR23LatestResultNeedsItsOwnWake` (`internal/orchestrator/r23_never_judged_test.go`); `TestR23OrchestratorNamesNeverJudged`, `TestR23JobContractSaysTheJudgePostsReviewAndRework` (`internal/job/lint/r23_judge_text_test.go`). Failing lines below; head: `go test -count=1 -run TestR23 -v ./internal/orchestrator/ ./internal/job/lint/` exit 0, every `--- PASS` |
| 3. `go test` of the changed packages and their importers | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...`: importers of `internal/orchestrator` are `internal/cli`, `internal/mcp`, `internal/migrate`; of `internal/job/lint` are `internal/cli`, `internal/job/jobcmd`. `go test -count=1 ./internal/orchestrator/ ./internal/cli/ ./internal/mcp/ ./internal/migrate/ ./internal/job/... ./internal/jobgraph/` exit 0: orchestrator ok 549.913s, cli ok 325.325s, mcp ok 5.684s, migrate ok 45.006s, job ok 21.772s, job/frontmatter ok, job/jobcmd ok 14.515s, job/lint ok 3.190s, jobgraph ok 2.666s |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`); `gofmt -l internal/orchestrator internal/job/lint` printed nothing |
| 5. Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| 6. Walk test `go test -count=1 -v ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0; `--- PASS: TestHsIpWalksEveryCommand` (walked 39 commands, 39 table entries), `--- PASS: TestIlGuestSiteList`, `ok internal/cli 0.119s` |
| 7. Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | in the run of row 3: `ok internal/job/lint 3.190s`, exit 0 (includes `TestGuDwAgentText`, `TestOrchestratorNamesTheOrphanKinds`, pinned phrases kept) |
| 8. Merge origin/main before the last proof | HOLDS | `git fetch origin` then `git log --oneline HEAD..origin/main` printed nothing: the lane is on current main |
| 9. No existing test edited or deleted | HOLDS | `git diff --stat origin/main..HEAD`: only `agents/orchestrator.md`, `docs/job-contract.md`, `internal/orchestrator/write.go` and two new test files |
| 10. No guard, gate, cap, hold or blocking hook on agents | HOLDS | `neverJudged` sets a report value in `grid job tree` only; nothing reads it but the orchestrator text, which says to leave the Job alone (`write.go`, `neverJudged`) |
| 11. App not changed; no push, no publish | HOLDS | the diff is plugin-only; `judge.go` was read with `git show` from the ref repo |
| *RealPlugin* tests, `make webui-test`, app walk tests, guest walk tables | not run, not asked | the brief names none; this is the plugin repo |
| Whole-module test and cross-repo set | not run, not asked | the orchestrator runs them at landing |

### New tests on main (production files `write.go`, `agents/orchestrator.md`, `docs/job-contract.md` from 0f360eaa; tests from the lane)

```
--- FAIL: TestR23NeverJudgedNamesASubmittedJobNobodyJudges (0.63s)
    r23_never_judged_test.go:45: J-1: want orphan never_judged since 2026-11-13T21:54:54.806Z, got <nil>
--- FAIL: TestR23LatestResultNeedsItsOwnWake (0.96s)
    r23_never_judged_test.go:131: J-1: want orphan never_judged since 2026-11-14T21:54:58.621Z, got <nil>
--- FAIL: TestR23OrchestratorNamesNeverJudged (0.00s)
    r23_judge_text_test.go:14: ../../../agents/orchestrator.md does not contain "`never_judged`"
    r23_judge_text_test.go:14: ../../../agents/orchestrator.md does not contain "app's Judge"
    r23_judge_text_test.go:14: ../../../agents/orchestrator.md does not contain "do not cancel it"
--- FAIL: TestR23JobContractSaysTheJudgePostsReviewAndRework (0.00s)
    r23_judge_text_test.go:29: docs/job-contract.md does not contain "The review of a submitted Job and the rework after a rejection are posted by the app's Judge, not by the plugin"
    r23_judge_text_test.go:29: docs/job-contract.md does not contain "`never_judged`"
```

`TestR23JudgedSubmittedJobIsNoOrphan` (wake acknowledged, review posted, closed, still running) passes on main too. It pins what must not become an orphan and is not a red-to-green test.

## What changed

- `internal/orchestrator/write.go`: new orphan kind `never_judged`. `neverJudged` names a Job whose status is `submitted`, whose lease (`LeaseExpires`) is before the tree's time, whose latest result's wake has no acknowledging record, and that has no Job with `discovered_from` set to it and role reviewer, checker or person. `Since` is the lease end, as for `never_acked` and `no_exit`. `TreeAt` fills it only when no other orphan is set. `grid job tree` prints any kind (`internal/cli/orchestrator.go:222`), and `--json` gains `orphan.kind` = `never_judged`.
  - The review-Job test is there because a summary wake judges the other submitted Jobs with no wake of their own (`judge.go:345-360`), so a Job that has its review can still hold an unacknowledged result.
- `agents/orchestrator.md:28`: names `never_judged`, says the Job's work is done, that the review and rework come from the app's Judge, and to leave the Job and name it in the result. The closing sentence now reads "For any other orphan, cancel ...", so the orchestrator does not cancel and re-post finished work.
- `docs/job-contract.md` (new paragraph after the `children-done` paragraph): the review of a submitted Job and the rework after a rejection are posted by the app's Judge; without the app's worker a done Job stays `submitted`; `grid job tree` names it `never_judged`.

## Open items

- `LeaseExpires` is set only at assign (`internal/contract/fold.go:205`). A Job that runs longer than its lease and then submits reads as `never_judged` from the moment it submits until the Judge's first pass acknowledges the wake. The line is true at that moment, and the orchestrator text says to leave the Job. A grace after the submit time would remove the flicker; the row says "past its lease", so none was added.
- A Job that exited `done` and posted no result has no wake at all, so it reads as `never_judged` as well. That is correct for the Judge's purposes (nothing wakes it), but the line says "no wake of its result acknowledged".
- No CLI test: the text line and `--json` come from the generic printer at `internal/cli/orchestrator.go:222`, which `TestGuDwTreeLine` already covers for a kind.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Whether the orchestrator agent leaves a `never_judged` Job alone needs a live agent run; the tests prove the tree line and the agent text. A live run with the app's worker stopped would show the line; with the worker running, the Judge's acknowledgement removes it.

DONE
