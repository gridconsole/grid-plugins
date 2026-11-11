# C-ic-replan-dead-job

A live eval case `evals/suite/replan-dead-job` for the Re-plan Job (skills/plan-jobs/SKILL.md:106-115), with a self-contained grader test. Re-files C-hj-replan-dead-job: the seed has two waiters, so a planner that follows the skill is never told to cancel the dead Job.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. TestIcpBad: `showed` and `cancelled` fail, `replaced` passes, bad.jsonl has tool calls | HOLDS | `go test -count=1 -run '^TestIcpBad$' -v ./internal/evalcmd/` exit 0: `showed ... passed=false (called 0 times)`, `cancelled ... passed=false (called 0 times)`, `replaced ... passed=true (called 1 times)`, `--- PASS: TestIcpBad` |
| 2. TestIcpGood: every code grader passes, llm not graded | HOLDS | `go test -count=1 -run '^TestIcpGood$' -v ./internal/evalcmd/` exit 0: `cancelled: passed`, `replaced: passed`, `showed: passed`, `criteria: not graded (llm), not counted as passed`, `--- PASS: TestIcpGood` |
| 3. TestIcpShape: prompt.md name/tags/runs, case.yaml 1.1, grader types known | HOLDS | `go test -count=1 -run '^TestIcpShape$' -v ./internal/evalcmd/` exit 0, `--- PASS: TestIcpShape` (also checks the three tool_used graders' tool, input_match and min, criteria llm weight 1, fixtures at most 40 lines) |
| 4. no emails, names, keys, home paths; evalcmd tests | HOLDS | `git grep -nE '@\|[A-Z][a-z]+ [A-Z][a-z]+son\|\bsk-\|ghp_\|/Users/\|/home/\|/root/\|/srv/' -- evals/suite/replan-dead-job internal/evalcmd/testdata/icp` printed nothing; `go test -count=1 ./internal/evalcmd/` exit 0 |
| 5. diff lists only Touches | HOLDS, plus the report | `git diff --name-only main`: evals/suite/replan-dead-job/{case.yaml,fixture.sh,prompt.md,graders/*.md}, internal/evalcmd/icp_case_test.go, internal/evalcmd/testdata/icp/{bad,good}.jsonl, and this file docs/lanes/C-ic-replan-dead-job.md (the runbook asks for it) |
| New tests fail on base | HOLDS | With the case folder and testdata stashed (the base state), `go test -count=1 -run TestIcp ./internal/evalcmd/`: `FAIL: TestIcpBad ... no graders under ../../evals/suite/replan-dead-job`, `FAIL: TestIcpGood ... no graders`, `FAIL: TestIcpShape ... open .../prompt.md: no such file or directory`, `FAIL: TestIcpFixture ... fixture.sh ... No such file` |
| No existing test edited or deleted | HOLDS | diff above holds only new files |
| No guard, gate, cap or hold on agents | HOLDS | the change adds an eval case and a test; no production file touched |
| Tests of changed package and importers | HOLDS | importers of internal/evalcmd by `go list`: internal/cli. `go test -count=1 ./internal/evalcmd/ ./internal/cli/` exit 0 (`ok evalcmd 2.142s`, `ok cli 65.471s`). `go test -count=1 -run TestSampleSuite ./internal/eval/` ok (evals/suite is outside it) |
| Lint | HOLDS | `make lint` exit 0 (go vet, gofmt clean) |
| Windows and macOS builds | HOLDS | `make cross` exit 0: vet and test compile for darwin/arm64 and windows/amd64 |
| *RealPlugin* tests | n/a | none in this lane |
| No claude, no eval run, no model money | HOLDS | none run |

## What changed

- `evals/suite/replan-dead-job/`: prompt.md (`tags: [live]`, `runs: 1`, `max_turns: 40`), case.yaml (`schema_version: "1.1"`), fixture.sh (builds `task-replan/` with intent.md, plan.md and feed/J-2..J-5: J-2 cancelled with cause and `exit_code: 137`, J-3 waits on J-2 with plan step 2 open, J-4 waits on J-2 with plan step 3 dropped, J-5 is the Re-plan Job with role orchestrator and skills plan-jobs), graders showed, cancelled, replaced (tool_used, Bash, min 1) and criteria (llm, weight 1).
- `internal/evalcmd/testdata/icp/bad.jsonl` (posts a replacement without reading J-2 and without cancelling J-4) and `good.jsonl` (shows J-2, reads the tree and plan, lints and posts the replacement naming the cause, cancels J-4, reads the tree).
- `internal/evalcmd/icp_case_test.go`: TestIcpBad, TestIcpGood, TestIcpShape, TestIcpRules (grader rules on small inputs), TestIcpFixture (runs fixture.sh in a temp dir and checks the seed). Self-contained, with an `icp` front matter parser and the grader rules. No clock, no sleep, temp dirs only.

## NOT VERIFIED on a live screen

- The live run: `claude plugin eval <plugin clone> --eval-dir evals/suite --case replan-dead-job --runs 1 --scaffold --no-publish --trust-plugin --json` was not run (no claude, no model money in this lane). Whether a live agent reads the seed as a Feed through `grid job show`/`grid job tree` against the synthetic folder, and whether `tool_used` input_match is applied by the built-in as a regexp over the tool input, is for the orchestrator's run. The fixture's Feed is plain Markdown files in `task-replan/feed/`, not Grid store records.
- The `criteria` llm grader is not run by any test here; the test logs it as not graded.
