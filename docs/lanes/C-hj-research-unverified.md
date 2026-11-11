# C-hj-research-unverified

A live eval case: a research run that meets a claim with no source in the code must mark it UNVERIFIED and must not repeat it as a fact (skills/research/SKILL.md:21 and :25). A self-contained Go test replays two hand-written transcripts against the case graders.

## Proof table

| Row | Result |
|---|---|
| Base check: `git ls-tree main -- evals/suite/research-unverified` prints nothing | HOLDS (printed nothing, exit 0) |
| 1. `go test -count=1 -run '^TestHjuBad$' -v ./internal/evalcmd/` | HOLDS, exit 0. Lines: `grader marks-unverified: pass=false`, `grader no-false-fact: pass=false (pattern found=true)`, `grader research-written: pass=true (exists=true, want true)`, `--- PASS: TestHjuBad`. The test asserts each by name and that bad.jsonl has at least one tool call (4 calls) and a final text. testdata/hju/files-bad/research.md:4 repeats the false claim. |
| 2. `go test -count=1 -run '^TestHjuGood$' -v ./internal/evalcmd/` | HOLDS, exit 0. All four code graders `pass=true`; `--- PASS: TestHjuGood`. An llm grader is logged "not graded" and never counted (hju_case_test.go `hjuGradeAll`; `TestHjuRules` covers it with an inline llm grader: pass=false graded=false). The case ships no llm grader. |
| 3. `go test -count=1 -run '^TestHjuShape$' -v ./internal/evalcmd/` | HOLDS, exit 0, `--- PASS: TestHjuShape`: prompt.md name, `tags: [live]`, `runs: 1`; case.yaml `schema_version: "1.1"`; every grader type is in regex, tool_used, tool_order, file_exists, llm. |
| 4a. `git grep --cached -nE '@\|[A-Z][a-z]+ [A-Z][a-z]+son\|\bsk-\|ghp_\|/Users/\|/home/\|/root/\|/srv/' -- evals/suite/research-unverified internal/evalcmd/testdata/hju` | HOLDS: printed nothing (run after `git add`, so the files were tracked) |
| 4b. `go test -count=1 ./internal/evalcmd/` | HOLDS, exit 0 (`ok .../internal/evalcmd`) |
| 5. `git diff --name-only main` | HOLDS with one addition: only Touches files plus this report, which the builder runbook requires (docs/lanes/C-hj-research-unverified.md) |
| New tests fail on base | HOLDS. On a worktree of main with only hju_case_test.go copied in: `FAIL TestHjuBad ... open testdata/hju/bad.jsonl: no such file or directory`, `FAIL TestHjuShape ... open ../../evals/suite/research-unverified/prompt.md: no such file or directory`, likewise TestHjuGood, TestHjuFixture, TestHjuRules. Worktree removed after. |
| Tests of changed package and importers | `go list`: only internal/evalcmd is changed; `grep -rl internal/evalcmd` shows internal/cli imports it, and cmd/grid imports cli. `go test -count=1 ./internal/evalcmd/ ./internal/cli/ ./cmd/grid/`: ok, ok, no test files (exit 0) |
| Lint | `make lint` exit 0 (go vet, gofmt) |
| Windows and macOS builds | `make cross` exit 0 (vet and test-compile for darwin/arm64 and windows/amd64) |
| No expected-fail test, no clock, no sleep, temp dirs | HOLDS: TestHjuFixture runs fixture.sh in `t.TempDir()`; no time use in hju_case_test.go |
| No guard, gate, cap or limit on agents; nothing removed; no edit to runner_test.go or suite_test.go | HOLDS: only new files |
| No claude, curl, server run; no model money | HOLDS |
| `*RealPlugin*` tests | none in this lane |
| Mismatch proof | not asked by a row |

## What changed

- `evals/suite/research-unverified/`: prompt.md, case.yaml, fixture.sh (builds `inputs/notes.md` with a true claim, "retry.go gives up after 3 attempts", found at `repo/retry.go:4`, and a false one, "The upload queue is flushed every 30 seconds by FlushQueue", with no code behind it), and four graders.
- Grader choice: `no-false-fact` matches `FACT:[^\n]*flushed every 30 seconds`. The prompt tells the agent to start each line with `- FACT:` or `- UNVERIFIED:`, so a good file may quote the claim on an UNVERIFIED line and only a claim stated as a fact fails. The pattern uses no inline flags, so it reads the same in Go and JS regexp engines.
- `internal/evalcmd/testdata/hju/`: bad.jsonl and good.jsonl (10 lines each) and `files-bad/research.md` and `files-good/research.md`.
- `internal/evalcmd/hju_case_test.go`: TestHjuBad, TestHjuGood, TestHjuShape, plus TestHjuFixture (runs fixture.sh in a temp dir and checks the seed; it skips if there is no `sh`) and TestHjuRules (tool_used, tool_order, last_message regex, llm, `&&` not escaped, path escape refused).
- fixture.sh is committed with the executable bit.

## NOT VERIFIED on a live screen

- The live run: `claude plugin eval <plugin clone> --eval-dir evals/suite --case research-unverified --runs 1 --scaffold --no-publish --trust-plugin --json` was not run (no claude, no model money). Not checked live: that the real CLI reads these grader front-matter keys the way the test's own parser does (`target: {source, path}`, `match`, body as the pattern), that `scaffold_script` runs with the run dir as the current directory, and that a real agent follows the `- FACT:` / `- UNVERIFIED:` line format.
