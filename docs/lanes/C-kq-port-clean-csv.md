# C-kq-port-clean-csv

Ports the eval case clean-csv to the built-in `claude plugin eval` format under evals/suite/clean-csv. Branch lane-C-kq-port-clean-csv, stacked on lane-C-kq-eval-builtin-fit (already merged into it).

## Proof table

| Row | Result |
|---|---|
| 1 TestKqCcShape | HOLDS. `go test -count=1 -v -run 'TestKqCc' ./evals/suitecheck/kqcc/` exit 0, prints `--- PASS: TestKqCcShape`. On the base (HEAD~1) the package does not exist: `stat .../evals/suitecheck/kqcc: directory not found` / `FAIL ./evals/suitecheck/kqcc [setup failed]`. |
| 2 TestKqCcMap | HOLDS, same run, `--- PASS: TestKqCcMap`. csv, rows and summary all have graders/<id>.md; `kqCcDropped` is empty (kqcc_test.go:30). prompt.md body and fixture.sh equal the task files. |
| 3 TestKqCcGood | HOLDS, same run, `--- PASS: TestKqCcGood`. Passed true, NotGraded empty. Expected bytes are built from the `rows` printf arguments (kqCcRowsWant). |
| 4 TestKqCcBad | HOLDS, same run, `--- PASS: TestKqCcBad`. `summary` and `rows` both fail; `csv` passes. |
| 5 TestKqCcFixture | HOLDS, same run, `--- PASS: TestKqCcFixture`. Row differs: setup.sh writes the header plus three data rows (one duplicate), four lines in all, not four data lines after the header. The test asserts what the file holds. |
| 6 diff scope | HOLDS. `git diff HEAD~1 --stat` names only evals/suite/clean-csv and evals/suitecheck/kqcc (12 files). `git diff origin/lane-C-kq-eval-builtin-fit --stat` is larger, because my local ref of that branch is older than what the launcher merged in (other lanes' merges); those files are not mine. |
| 6 make lint | HOLDS, exit 0 (`go vet ./...`). `go vet ./evals/...` exit 0, so no doc.go was needed. |
| 6 make cross | HOLDS, exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| Walk tests (plugin) | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. |
| Packages that import changed code | HOLDS. I changed no Go production code. Importers of internal/evalcmd (go list): internal/cli. `go test -count=1 ./internal/evalcmd/ ./internal/cli/ ./evals/...` all ok. |
| New tests fail on base | HOLDS, see row 1. |
| No edit of an existing test, no deletion | HOLDS. Only new files. |
| No guard, gate, cap on agents | HOLDS. None added. |
| Real-plugin tests | None named in the brief. |

## What changed

- evals/suite/clean-csv/: case.yaml, prompt.md (front matter name, runs 1, max_turns 20, allowed_tools Read, Write, Bash), fixture.sh (copy of tasks/clean-csv/setup.sh), graders csv.md (file_exists), summary.md (regex on summary.txt), rows.md (regex on clean.csv).
- rows.md: builtin-fit.md (a) says a whole-file compare needs an anchored JavaScript regex, so `rows` is not dropped. The pattern is `^item,quantity\napple,2\npear,3\n$`, with no flags. Without the `m` flag `^` and `$` anchor the whole file in both JS and Go.
- evals/suitecheck/kqcc/: kqcc_test.go and testdata. `graderKeys` is unexported, so the test keeps its own copy of the four type names (kqcc_test.go:25).
- Where builtin-fit.md differs from the brief: it says an exported prompt.md has no `name`. The brief asks for `name: clean-csv` in the prompt.md front matter, as research-unverified has. I followed the brief. builtin-fit.md (f) names Write and Bash as grants for clean-csv, which matches the live-run line.

## NOT VERIFIED

- The built-in `claude plugin eval` has not read these files. I ran no claude and no eval. Whether the built-in accepts the multi-line anchored pattern in rows.md the way the offline GradeCase does is unproven; GradeCase uses Go regexp.
- The live run is the orchestrator's step after landing.
