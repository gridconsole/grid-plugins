# C-kb3-questions-skill-text

Proof table (head: see the report line).

| Row | Result |
|---|---|
| 1. `go test -count=1 -v -run TestKb3QWorkerAskText ./internal/contract/` | HOLDS, `--- PASS: TestKb3QWorkerAskText`. With SKILL.md at main: `--- FAIL: TestKb3QWorkerAskText (0.00s)` (step 2 lacks "may not post person gates", lacks "gate_post_roles", still holds the old refusal text) |
| 2. `go test -count=1 -v -run 'TestKb3QAskByWorkerIsRefusedNoJob\|TestQuestionAskByAgentPostsNoJob' ./internal/cli/` | HOLDS, both `--- PASS`, exit 0. The worker ask exits non-zero with `role worker may not post person gates`, only the harness Job is in the Feed |
| 3. TestSkillQuestionsAskText, TestSkillQuestionsAskContract, TestSkillAndAgentCommandsExist | HOLDS, three `--- PASS` |
| 4. `go test -count=1 ./internal/packaging/ -run TestEm` | HOLDS, exit 0 |
| 5. TestHsIpWalksEveryCommand\|TestIlGuestSiteList; `./internal/job/lint/`; `make lint` | HOLDS, all exit 0 |
| 5. `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| 6. `git diff main --stat` | only skills/questions/SKILL.md, the two new tests, this note |
| Rule: new tests fail on base, pass on lane | HOLDS (row 1 for the text test; the cli test pins current code behaviour and passes on base too, as the brief models it on the agent test) |
| Rule: packages that import the changed ones | the changed packages are test-only plus a skill text; internal/contract and internal/cli tests ran above |
| Rule: no guard added | HOLDS, text and tests only |
| Rule: existing tests not edited | HOLDS |

## What changed
- skills/questions/SKILL.md step 2: names the refusal `role worker may not post person gates`, its source (the workflow's `gate_post_roles`, default cli, owner, member, lead) and when it applies. The `grid job post --person` span has no `--body`.
- internal/contract/kb3_questions_worker_ask_test.go: TestKb3QWorkerAskText.
- internal/cli/kb3_question_ask_worker_test.go: TestKb3QAskByWorkerIsRefusedNoJob.

## NOT VERIFIED on a live screen
Nothing here is UI.
