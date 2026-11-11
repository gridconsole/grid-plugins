# C-lw-eval-graders-name-only

Base: main 3ab3b0ce (merge of lane-Pint129 parts). Lane head: see the report footer.

## Base check (row 1, red)

`go test -count=1 -v -run 'TestLjAllSuite' ./evals/suitecheck/ljall/` on 3ab3b0ce:

```
ljall_test.go:117: bash-one-command: known-open: grader no-bypass: tool_used grader has unknown key "name"
ljall_test.go:117: builder-not-done: known-open: grader names-test: regex grader has unknown key "name"
ljall_test.go:117: job-result-posted: known-open: grader not-met: tool_used grader has unknown key "name"
ljall_test.go:117: plan-step-anchor: known-open: grader no-steps-anchor: tool_used grader has unknown key "name"
ljall_test.go:117: replan-dead-job: known-open: grader cancelled: tool_used grader has unknown key "name"
ljall_test.go:117: research-unverified: known-open: grader cites-lines: regex grader has unknown key "name"
ljall_test.go:117: review-posts-found: known-open: grader found-before-result: tool_order grader has unknown key "before_match"
```

None of the six graded clean on the base.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1 green | HOLDS | `go test -count=1 -v -run 'TestLjAllSuite|TestLjAllKnownOpenBothWays|TestLjAllCatches' ./evals/suitecheck/ljall/` exit 0: `ljall_test.go:103: bash-one-command: graded`, `builder-not-done: graded`, `job-result-posted: graded`, `plan-step-anchor: graded`, `replan-dead-job: graded`, `research-unverified: graded`; `review-posts-found: known-open`; `--- PASS: TestLjAllSuite` |
| 2 no `name:` in graders | HOLDS | `git grep -c '^name:' -- <six grader folders>` prints nothing (no hit). `git diff main --stat`: 21 graders/*.md (1 line deleted each), the six case test files, ljall_test.go; nothing else |
| 3 map holds one entry | HOLDS | `git grep -c '": "unknown key",$' -- evals/suitecheck/ljall/ljall_test.go` prints `evals/suitecheck/ljall/ljall_test.go:1`; `git grep -n '"review-posts-found": "unknown key"'` hits ljall_test.go:23; `--- PASS: TestLjAllKnownOpenBothWays` and `--- PASS: TestLjAllCatches` (exit 0) |
| 4 case tests | HOLDS | `go test -count=1 -v -run 'TestHja\|TestHjb\|TestHjn\|TestHjr\|TestHju\|TestIcp' ./internal/evalcmd/` exit 0: `--- PASS` for all 26 named tests, no `--- FAIL`. Bad tests: the `-v` lines carry the same grader verdicts as the base run (e.g. TestHjaBad no-steps-anchor fails; TestHjbBad no-compound/no-bypass fail, ran-test passes; TestHjnBad all three fail; TestHjrBad result-posted/not-met fail; TestHjuBad marks-unverified/no-false-fact fail, cites-lines/research-written pass; TestIcpBad showed/cancelled fail, replaced passes). No grader changed verdict |
| 5 tests, lint, cross | HOLDS | `go test -count=1 ./evals/suitecheck/... ./internal/evalcmd/` exit 0; `make lint` exit 0 (go vet ./...); `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| Importers tested | HOLDS | Packages importing internal/evalcmd (go list): evals/suitecheck/{kqcc,kqft,kqrn,ljall,ljba}, internal/cli (and cmd/grid imports cli). `go test -count=1 ./evals/suitecheck/... ./internal/evalcmd/ ./internal/cli/ ./cmd/grid/` exit 0, all ok |
| New tests fail on base, pass on lane | HOLDS (no new test) | The brief forbids a new test file; TestLjAllSuite is the walk and fails both ways (ljall_test.go:106-125, ljAllCheck); the base run above is the red state, the lane run the green. I did not run the "graded but listed" direction. The "no `name:` key" checks are not shown failing on a grader that carries `name:`, except once by accident: with the check in hjuParseGrader, TestHjuRules failed with `hju_case_test.go:490: grader front matter has a name: key` on its inline `name: a` strings, so the check was moved to hjuLoadGraders |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok (exit 0); `go test -count=1 ./internal/job/lint/` ok (exit 0). The app walk tests are not in this repo |
| RealPlugin tests | n/a | No row names a *RealPlugin* test |
| No guard/gate/cap | HOLDS | Only grader front matter lines, test helpers and the known-open map changed; suitegrade.go untouched |
| Existing tests | HOLDS | No existing test edited to pass: the helpers that read `name:` now read the file name, and the old "name matches the file name" assertions became "no `name:` key" assertions as the brief states; no test deleted. hju's inline `name: a` strings in TestHjuRules are untouched (the no-name check sits in hjuLoadGraders, not hjuParseGrader) |
| Public repo hygiene | HOLDS | No personal paths, hosts or emails added |

## What changed

- Removed the `name:` line (line 2) from the 21 graders/*.md of bash-one-command, builder-not-done, job-result-posted, plan-step-anchor, replan-dead-job, research-unverified. No other key or body text.
- internal/evalcmd hja/hjn/hjr/icp `name()` returns the file name without `.md`; hjb and hju set `Name` from the file name; the "name matches the file" checks became "front matter has no `name:` key" (hjn gained the check in its shape loop, which had none).
- evals/suitecheck/ljall/ljall_test.go: six entries leave `ljAllKnownOpen`; review-posts-found stays, its comment now names the real reason (before_tool/after_tool keys).

## NOT DONE

Nothing.

## NOT VERIFIED on a live screen

Nothing here is UI. `claude plugin eval` was not run (forbidden); the six cases are graded by GradeCase only, not run live.

Verdict: DONE
