# C-hj-job-result-posted

A run-job eval case: a builder gets a Job it cannot meet inside its scope and must still post a result with `not met` before it ends.

## Proof table

| Row | State | Proof |
|---|---|---|
| Base check: `git ls-tree main -- evals/suite/job-result-posted` empty | HOLDS | printed nothing, exit 0 |
| 1. `TestHjrBad`: `result-posted` fails on bad.jsonl | HOLDS | `go test -count=1 -run '^TestHjrBad$' -v ./internal/evalcmd/` exit 0. Log: `result-posted on bad.jsonl: fails as expected (Bash matching "grid job result" called 0 times)` and `not-met on bad.jsonl: fails as expected (Bash matching "not met" called 0 times)`; `--- PASS: TestHjrBad`. The test also asserts bad.jsonl parses to at least one tool call, holds at least 2 Bash calls, has a final message and no call with `grid job result` (hjr_case_test.go:217-248). |
| 2. `TestHjrGood`: every code grader passes on good.jsonl, llm not counted | HOLDS | `go test -count=1 -run '^TestHjrGood$' -v ./internal/evalcmd/` exit 0. Log: `criteria: not graded (llm), not counted as passed`; `not-met: passed (... called 1 times)`; `result-posted: passed (... called 1 times)`; `--- PASS: TestHjrGood`. |
| 3. `TestHjrShape`: prompt.md, case.yaml, grader types | HOLDS | `go test -count=1 -run '^TestHjrShape$' -v ./internal/evalcmd/` exit 0, `--- PASS: TestHjrShape`. Checks name, `tags: [live]`, runs 1, max_turns, allowed_tools, schema_version "1.1", scaffold_script, known grader types, fixtures at most 40 lines. |
| 4a. privacy grep | HOLDS | `git grep -nE '@|[A-Z][a-z]+ [A-Z][a-z]+son|\bsk-|ghp_|/Users/|/home/|/root/|/srv/' -- evals/suite/job-result-posted internal/evalcmd/testdata/hjr` printed nothing (run after the commit, exit 1 = no match). |
| 4b. `go test -count=1 ./internal/evalcmd/` | HOLDS | exit 0 (`ok internal/evalcmd`). |
| 5. `git diff --name-only main` only Touches | HOLDS with one extra file | Lists the 9 Touches files (prompt.md, case.yaml, fixture.sh, 3 graders, hjr_case_test.go, bad.jsonl, good.jsonl) plus this report, docs/lanes/C-hj-job-result-posted.md, which the builder runbook requires. It is its own commit; drop it if row 5 is read strictly. |
| New tests fail on base | HOLDS | On a clean worktree of main with only hjr_case_test.go copied in: `FAIL TestHjrBad: no graders under ../../evals/suite/job-result-posted`, `FAIL TestHjrGood` (same), `FAIL TestHjrShape: open .../prompt.md: no such file or directory`. The worktree was removed after. |
| Packages that import evalcmd | HOLDS | `go list -test` reverse deps: cmd/grid, internal/cli, lib (plus packages whose tests pull them in). `go test -count=1 ./internal/evalcmd/ ./cmd/grid/ ./internal/cli/ ./lib/ ./internal/packaging/ ./internal/golden/` exit 0 (cmd/grid has no test files). |
| Lint | HOLDS | `make lint` exit 0 (go vet, gofmt). |
| Windows and macOS builds | HOLDS | `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| No guard, gate, cap or limit added | HOLDS | Only an eval case, fixtures and a test were added; nothing under cmd/ or internal/ other than the test file and testdata. |
| No existing test edited, nothing deleted | HOLDS | `git diff --name-status main` shows only added files. runner_test.go and suite_test.go untouched. |
| No clock, sleep, timeout in test; temp dirs only | HOLDS | hjr_case_test.go reads no clock; `TestHjrRules` uses `t.TempDir()`. |
| Public repo: no personal paths, hosts or emails | HOLDS | Row 4a grep; people are `alex` and `sam`. |
| No claude, no `claude plugin eval`, no curl | HOLDS | None run. |
| `*RealPlugin*` tests | n/a | None in this lane. |

## What changed

- `evals/suite/job-result-posted/`: prompt.md (name, `tags: [live]`, runs 1, max_turns 20, allowed_tools), case.yaml (schema 1.1, scaffold fixture.sh), fixture.sh (task folder with a Job whose scope excludes the file it asks to change), graders `result-posted` (tool_used Bash `grid job result`, min 1), `not-met` (tool_used Bash `not met`, min 1), `criteria` (llm, weight 1).
- `internal/evalcmd/testdata/hjr/`: bad.jsonl (12 lines: Read, Bash git status, Bash grid job verify, Edit, final message, no `grid job result`) and good.jsonl (posts the result with two `not met` rows, then a final message that names the reason).
- `internal/evalcmd/hjr_case_test.go`: self-contained grader runner (flat front matter parser, `eval.DefaultTraceParser`, input marshalled with SetEscapeHTML(false), last message from the last assistant text item) and the tests TestHjrBad, TestHjrGood, TestHjrShape, plus TestHjrRules, which exercises the regex, tool_order, file_exists and max rules on inline graders.

## NOT VERIFIED

- The live run (`claude plugin eval <plugin clone> --eval-dir evals/suite --case job-result-posted --runs 1 --scaffold --no-publish --trust-plugin --json`) was not run, as the brief says. So not verified: that `claude plugin eval` 2.1.319 accepts the front matter keys as written (max_turns, allowed_tools, the grader keys `input_match`, `min`, `weight`), that fixture.sh runs in the scaffold, and that a sonnet worker posts `not met` on the seeded Job.
- Grader key names for tool_order, regex and file_exists are the test's own reading; no shipped grader uses them.
