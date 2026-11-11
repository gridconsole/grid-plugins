# Lane M-C-lw-eval-review-posts-before-after-onto-main

Carries C-lw-eval-review-posts-before-after (its report: docs/lanes/C-lw-eval-review-posts-before-after.md) onto plugin main 3ca6184639945669bf688f3f145bc7572b2c2ffe.

## Conflict and resolution

One conflict, `evals/suitecheck/ljall/ljall_test.go`, the `ljAllKnownOpen` map. Main had removed the six `name:` entries (job-result-posted, bash-one-command, builder-not-done, plan-step-anchor, replan-dead-job, research-unverified) and kept review-posts-found. The lane removes review-posts-found and still carried the six. Both removals are kept, so the map is empty: `var ljAllKnownOpen = map[string]string{}` (gofmt form). No test, row or error code was dropped; the lane's own three review-posts-found grader edits and `internal/evalcmd/kb_found_case_test.go` merged without conflict.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1 ancestor | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0 (origin/main = 3ca61846) |
| 1 diff names | HOLDS | `git diff origin/main --stat`: docs/lanes/C-lw-eval-review-posts-before-after.md, the three review-posts-found grader files, ljall_test.go, kb_found_case_test.go (plus this report) |
| 2 lane row 1 (red then green) | HOLDS | red on base is in the C-lw report (`review-posts-found: known-open: grader found-before-result: tool_order grader has unknown key "before_tool"`, base 3ab3b0ce). Merged head: `go test -count=1 -v -run TestLjAll ./evals/suitecheck/ljall/` prints `review-posts-found: graded`, `--- PASS: TestLjAllSuite`, exit 0 |
| 2 lane row 2 (grader keys) | HOLDS | `git grep -n -E '^(name\|before_tool\|before_match\|after_tool\|after_match):' -- evals/suite/review-posts-found/graders` prints nothing (0 lines, exit 1); `git grep ... '^(before\|after):' found-before-result.md` prints `:3 before: {tool: Bash, input_match: '--type found'}` and `:4 after: {tool: Bash, input_match: 'grid job result'}` |
| 2 lane row 3 (known-open) | HOLDS | `git grep -n '"review-posts-found": "unknown key"' -- evals/suitecheck/ljall/ljall_test.go` prints nothing; same run prints `--- PASS: TestLjAllKnownOpenBothWays` and `--- PASS: TestLjAllCatches` |
| 2 lane row 4 (TestKbEv) | HOLDS | `go test -count=1 -v -run TestKbEv ./internal/evalcmd/` exit 0: PASS TestKbEvBad, TestKbEvGood, TestKbEvShape, TestKbEvRules; bad `found-before-result ... (before at -1, after at 3)` fails, good `found-before-result: passed (before at 3, after at 4)`, verdicts as in the C-lw report |
| 2 lane row 5 | HOLDS | `go test -count=1 ./evals/suitecheck/... ./internal/evalcmd/` exit 0 (kqcc, kqft, kqrn, ljall, ljba, evalcmd ok); `make lint` exit 0 after gofmt fix of the emptied map (first run exited 2 on gofmt of ljall_test.go; fixed in "Format the emptied ljAllKnownOpen map."); `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile; run on the merge commit, the later commit changes only whitespace in one test file and `make lint` and ljall tests were rerun after it) |
| 3 walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| Existing tests not edited to pass | HOLDS | merge resolution only; no test edited or removed beyond the C-lw lane's own helper moves |
| New tests | n/a | none added |
| No guard/gate/cap on agents | HOLDS | none added |
| RealPlugin tests | n/a | no row names one |

## What changed

- Merge of origin/main into the lane; conflict in ljall_test.go resolved to an empty `ljAllKnownOpen`.
- Gofmt fix of that map literal.

## NOT VERIFIED on a live screen

None; no UI. A live `claude plugin eval` run was not done (not allowed in this sandbox).

Verdict: DONE
