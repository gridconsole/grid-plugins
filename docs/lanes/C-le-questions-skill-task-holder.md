# C-le-questions-skill-task-holder: proof table

| Row | Result |
|---|---|
| 1 base is ku2 | HOLDS: `git grep -n 'GRID_TASK_EPOCH' -- internal/cli/question.go` prints lines 32, 34, 35 (exit 0) |
| 2 step 2 names the epoch | HOLDS: skills/questions/SKILL.md:18; `go test -count=1 -v -run TestLeQs ./internal/contract/` exit 0, `--- PASS: TestLeQsStep2NamesTheTaskEpoch`. On base (SKILL.md stashed) it failed: `le_questions_task_holder_test.go:20: step 2 of skills/questions/SKILL.md lacks "GRID_TASK_EPOCH"`, exit 1 |
| 3 code matches text | HOLDS: `--- PASS: TestLeQsCodeMatchesText` (same run, exit 0) |
| 4 pins unchanged | HOLDS: `go test -count=1 -run 'TestKb3QWorkerAskText\|TestSkillQuestionsAsk' ./internal/contract/` exit 0; `go test -count=1 -run 'TestKuJa\|TestKb3QAskByWorkerIsRefusedNoJob' ./internal/cli/` exit 0 |
| 5 diff and checks | HOLDS: `git diff main --stat` names only SKILL.md and the new test file (plus this report); `make lint` exit 0; `make cross` exit 0 (darwin/arm64 and windows/amd64) |
| New tests fail on base, pass on lane | HOLDS: see row 2 |
| Package tests | HOLDS: `go test -count=1 ./internal/contract/` exit 0. The change is a `_test.go` file and a skill text; no package imports it as test code, so no importer is affected (`go list` checked) |
| Walk tests | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| No guard added; no existing test edited | HOLDS |
| RealPlugin tests | Not asked by any row |

## What changed
- skills/questions/SKILL.md step 2: a Job agent with GRID_TASK_EPOCH (every Job agent the app starts) asks as the task holder and `ask` posts its person Job, so it posts no second one. An agent without GRID_TASK_EPOCH still gets `role worker may not post person gates` and posts a person Job with `grid job post --person`.
- internal/contract/le_questions_task_holder_test.go: `TestLeQsStep2NamesTheTaskEpoch`, `TestLeQsCodeMatchesText`.

## NOT VERIFIED on a live screen
Nothing here is UI.
