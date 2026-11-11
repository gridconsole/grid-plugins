# C-kb-eval-review-posts-found: lane report

Head: see the bundle (one commit on top of main: "Add a live eval case where the reviewer posts a found record before its verdict, with its test.").

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. TestKbEvShape passes; fails on base | HOLDS | `go test -count=1 -v -run 'TestKbEv' ./internal/evalcmd/` exit 0, prints `--- PASS: TestKbEvShape`. On base (test file only, no case folder), exit 1: `kb_found_case_test.go:287: open ../../evals/suite/review-posts-found/prompt.md: no such file or directory` / `--- FAIL: TestKbEvShape` (also `no graders under ../../evals/suite/review-posts-found` for TestKbEvBad and TestKbEvGood). |
| 2. TestKbEvBad passes | HOLDS | Same command: `--- PASS: TestKbEvBad`; `found-posted on bad.jsonl: fails as expected (Bash matching "--type found" called 0 times)`, `found-before-result ... (before at -1, after at 3)`; llm grader `found-security` is not graded. |
| 3. TestKbEvGood passes | HOLDS | Same command: `--- PASS: TestKbEvGood`; `found-before-result: passed (before at 3, after at 4)`, `found-posted: passed (... called 1 times)`; llm not counted. good.jsonl holds one `grid job post --task T-01 --job J-2 --type found --body ...` then `grid job result`. |
| 4a. `go test -count=1 -run 'TestHjr\|TestSampleSuite' ./internal/evalcmd/ ./internal/eval/` | HOLDS | exit 0: `ok .../internal/evalcmd 0.006s`, `ok .../internal/eval 0.052s`. |
| 4b. `sh -n evals/suite/review-posts-found/fixture.sh` | NOT DONE | The command is outside my allow list and was refused (also as `/bin/sh -n`). I read the file: three quoted heredocs (`<<'EOF'`) each closed by `EOF`, plus one for `src/auth.go`, `set -eu`, same shape as `job-result-posted/fixture.sh`. The orchestrator should run it. |
| 5a. `git diff main --stat` names only new files | HOLDS | 10 files, 525 insertions, 0 deletions; all under `evals/suite/review-posts-found/` and `internal/evalcmd/` (kb_found_case_test.go, testdata/kbfound/). The report file is added after this stat. |
| 5b. `make lint` | HOLDS | exit 0 (`go vet ./...` and the rest of the target). |
| 5c. `make cross` (darwin/arm64, windows/amd64) | NOT DONE (whole target) | `make cross` was stopped by the 10-minute background limit: darwin/arm64 vet and test compile finished green, windows/amd64 vet was still running when it was killed. Run on the changed package only: `GOOS=windows CGO_ENABLED=0 go vet ./internal/evalcmd/` exit 0, `GOOS=darwin CGO_ENABLED=0 go vet ./internal/evalcmd/` exit 0. The change is one new `_test.go` file plus data and shell files, so no other package is affected; the orchestrator runs the full target at landing. |

## Rules

| Rule | State | Proof |
|---|---|---|
| New tests fail on base, pass on lane | HOLDS | Base failure lines under row 1; lane run under rows 1-3. Also TestKbEvRules (the grader logic itself) passes. |
| No existing test edited or deleted | HOLDS | `git diff main --stat`: only new files. |
| No guard, gate, cap, hold or hook on agents | HOLDS | Only an eval case and a test; nothing touches agent behaviour. |
| Packages that import the changed package | HOLDS | Changed package: `internal/evalcmd` (test file only, no non-test code). `go list` importers of it: `internal/cli`. `go test -count=1 ./internal/evalcmd/ ./internal/eval/` exit 0 (`ok internal/evalcmd 23.293s`). `internal/cli` is covered by the walk test below; its whole-package run is left to the landing run because no non-test code changed. |
| Walk test `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0. |
| Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | `ok .../internal/job/lint 9.682s`. |
| Helpers prefixed `kbEv`, tests `TestKbEv`, no hjr helper called | HOLDS | kb_found_case_test.go: constants, types and functions all start `kbEv`; tests `TestKbEvBad/Good/Shape/Rules`. |
| Graders: tool_used, tool_order, llm only | HOLDS | graders/found-posted.md (tool_used, Bash, `--type found`, min 1), found-before-result.md (tool_order), found-security.md (llm, kind security, `at` names src/auth.go:11). |
| `internal/eval/suite_test.go` id list untouched | HOLDS | The case is in `evals/suite/`, not `evals/tasks/`. |
| No personal paths, hosts or emails; no attribution lines | HOLDS | Files written from scratch with synthetic names only; commit message has no trailer. |

## Seen red, not this lane's

Whole-package `go test -count=1 ./internal/eval/` exited 1 in `TestFakeAgentSamples` (clean-csv, fix-total) and `TestJudgeHook/success`, all with `TimeoutMS:3000 ... context deadline exceeded` in the fake runner (`runner_test.go:63`, `:235`); a rerun of `-run 'TestFakeAgentSamples|TestJudgeHook'` exited 1 again on clean-csv with Setup at 892 ms of the 3 s budget. These tests run the `evals/tasks` samples with a 3 s timeout on a loaded VM; this lane adds no file under `evals/tasks` or `internal/eval`. I did not run them on a clean `main` clone, so "same on base" is not proven. `TestSampleSuite` (the row 4 command) passes. `internal/evalcmd` whole package passes (`ok 23.293s`).

## What changed

- `evals/suite/review-posts-found/`: case.yaml, prompt.md, fixture.sh, graders/{found-posted,found-before-result,found-security}.md. The fixture builds task T-01: builder Job J-1 and its result, reviewer Job J-2, and `src/auth.go` whose line 11 is `log.Printf("login ok user=%s token=%s", user, token)`. The prompt does not say to post a `found` record, so the live run tests what the skills teach.
- `internal/evalcmd/kb_found_case_test.go` and `testdata/kbfound/` (good.jsonl, bad.jsonl, files/src/auth.go).

## NOT VERIFIED

- No live run (needs claude; an orchestrator step after the text lanes land).
- The exact `found` body shape (`kind`, `at`, `text`) is taken from the brief; `docs/job-contract.md` and the CLI help do not document it. The llm grader reads the posted body, so if the real schema names the fields differently, the grader text and good.jsonl need the same edit.

## Open items

- Run `sh -n` on the fixture (refused here).
- Run the full `make cross` at landing (it needs more than 10 minutes here).
- `internal/cli` whole-package test not run here (only the two walk tests); no non-test code changed.
