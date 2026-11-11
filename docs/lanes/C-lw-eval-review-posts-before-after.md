# Lane C-lw-eval-review-posts-before-after

Base: 3ab3b0cecc28b34d2bf601013fd161e21e9ec7ac (main-merge lane-Pint129 head the clone started from).

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1 red on base | HOLDS | `go test -count=1 -v -run TestLjAllSuite ./evals/suitecheck/ljall/` on base 3ab3b0ce: `ljall_test.go:117: review-posts-found: known-open: grader found-before-result: tool_order grader has unknown key "before_tool"`, exit 0 |
| 1 green on lane | HOLDS | same command on lane: `ljall_test.go:113: review-posts-found: graded`, `--- PASS: TestLjAllSuite` (line 113 is the graded log line in the current file; the `ljAllKnownOpen` entry is gone so the file shifted by two lines) |
| 2 grader keys | HOLDS | `git grep -n -E '^(name\|before_tool\|before_match\|after_tool\|after_match):' -- evals/suite/review-posts-found/graders` prints nothing (exit 1); `git grep -n -E '^(before\|after):' -- .../found-before-result.md` prints `:3 before: {tool: Bash, input_match: '--type found'}` and `:4 after: {tool: Bash, input_match: 'grid job result'}`; `git diff main --stat`: the three grader files, kb_found_case_test.go, ljall_test.go only (plus this report) |
| 3 known-open entry | HOLDS | `git grep -n '"review-posts-found": "unknown key"' -- evals/suitecheck/ljall/ljall_test.go` prints nothing (exit 1); `go test -count=1 -v -run 'TestLjAllKnownOpenBothWays\|TestLjAllCatches' ./evals/suitecheck/ljall/` prints `--- PASS: TestLjAllKnownOpenBothWays` and `--- PASS: TestLjAllCatches` |
| 4 TestKbEv | HOLDS | `go test -count=1 -v -run TestKbEv ./internal/evalcmd/`: PASS TestKbEvBad, TestKbEvGood, TestKbEvShape, TestKbEvRules, no FAIL. Verdicts unchanged from base: bad `found-before-result ... (before at -1, after at 3)` fails, `found-posted ... called 0 times` fails; good `found-before-result: passed (before at 3, after at 4)`, `found-posted: passed (... called 1 times)`. TestKbEvRules keeps its three tool_order cases (true, false, false) with `before`/`after` maps |
| 5 tests, lint, cross | HOLDS | `go test -count=1 ./evals/suitecheck/... ./internal/evalcmd/ ./internal/cli/` exit 0 (importers of internal/evalcmd by grep: internal/cli, suitecheck/kqft, kqrn, kqcc, ljall, ljba); `make lint` (go vet ./...) exit 0; `make cross` (darwin/arm64 and windows/amd64 vet + test compile) exit 0 |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| Existing tests not edited to pass | HOLDS | TestKbEv* kept, their helpers (grader struct, front matter read, applier, shape check, inline rules) moved to the new keys as the brief lists; verdicts identical |
| New tests | n/a | no new test; TestLjAllSuite already fails both ways (row 1) |
| No guard/gate/cap on agents | HOLDS | none added |
| RealPlugin tests | n/a | no row names one |

## What changed

- `found-before-result.md`: `type: tool_order`, `before`/`after` maps; body unchanged. `found-posted.md`, `found-security.md`: `name:` line removed only (found-security keeps `type: llm`, `weight: 1`).
- `kb_found_case_test.go`: grader front matter is decoded with `mdfile.Split`/`mdfile.Load`; the grader name is the file name (the shape check now asserts no `name:` key); the tool_order applier, shape check and inline rules read the `before`/`after` maps. `kbEvFrontMatter`/`kbEvFlat` stay for prompt.md and case.yaml.
- `ljall_test.go`: the review-posts-found entry and its comment leave `ljAllKnownOpen`.
- No production file touched.

## NOT VERIFIED on a live screen

None; no UI. A live `claude plugin eval` run of the case was not done (not allowed in this sandbox).

Verdict: DONE
