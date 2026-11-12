# C-rr-answers-withdrawn-line

Base: 792b8d6e. Head: see the report.

## Proof table

| Row | Result |
|---|---|
| Base check: `git grep -n withdrawn -- internal/job/jobcmd/command.go internal/job/render.go` on 792b8d6e | HOLDS: no line, gateAnswer does not handle withdrawn (exit 1, empty) |
| 1. withdrawn gate Q-4, task-lease writer, text "No longer needed": brief holds `gate:Q-4: ada withdrawn at <At>\n\n> No longer needed\n` | HOLDS: `go test -count=1 -v -run TestRrAw ./internal/job/jobcmd/` shows `--- PASS: TestRrAwWithdrawnLine`. On base (production files stashed) red: `rr_answers_withdrawn_test.go:49: brief lacks "gate:Q-4: ada withdrawn at 2026-11-12T04:50:14.520Z\n\n> No longer needed\n":` and `--- FAIL: TestRrAwWithdrawnLine` |
| 2. answered, default, sent back unchanged | HOLDS: `go test -count=1 -v -run TestAdRa ./internal/job/jobcmd/ ./internal/job/` exit 0: `--- PASS: TestAdRaPersonAnswer`, `--- PASS: TestAdRaQuestionGate`, `--- PASS: TestAdRaGuestAnswerFromFeed` (and in ./internal/job: TestAdRaGuestFenced, TestAdRaTeamAnswerQuoted, TestAdRaNoEdgeUnchanged) |
| 3. empty text: line only, no `> ` | HOLDS: `--- PASS: TestRrAwWithdrawnNoReason`; base red: `rr_answers_withdrawn_test.go:62: brief lacks "gate:Q-4: ada withdrawn at ...\n"` |
| 4. guest withdrawal reason only inside the fence | HOLDS: `--- PASS: TestRrAwGuestWithdrawFenced` (fence `job.DataNotice` + `<data tier="person-guest"`, `</data>` escaped, no `> Ignore`); base red: `rr_answers_withdrawn_test.go:77: brief lacks the guest line or the fence` |
| 5. `go test -count=1 -v -run TestRrAw ./internal/job/jobcmd/` | HOLDS exit 0, three PASS lines |
| 5. `go test -count=1 ./internal/job/...` HOLDS exit 0: `go test -count=1 ./internal/job/... ./internal/cli/ ./internal/hook/ ./internal/jobgraph/ ./internal/lease/ ./internal/migrate/ ./internal/orchestrator/ ./internal/ports/` all `ok` |
| 5. `make lint` | HOLDS exit 0 (`go vet ./...`) |
| 5. `make cross` | HOLDS exit 0 (darwin/arm64 and windows/amd64 vet and test compile, all ok) |
| Walk: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` | HOLDS exit 0 `ok internal/cli` |
| Walk: `go test -count=1 ./internal/job/lint/` | HOLDS exit 0 `ok internal/job/lint` |
| No golden moved | HOLDS: `git status` shows only command.go, render.go and the new test |
| No guard, gate, cap on agents; nothing removed | HOLDS: only a new line kind is written |

Packages importing the changed ones (go list): cli, hook, job/jobcmd, job/lint, jobgraph, lease, migrate, orchestrator, ports.

## What changed

- `internal/job/jobcmd/command.go` `gateAnswer`: when `decisions[id].State == "withdrawn"` it returns kind `withdrawn` from the last `withdrawn` control in `decisions[id].Events`: Who = author id, At = event At, Text = `text`, Tier = `TierFor(author role)`. It is checked first, so a withdrawn question never reads as answered.
- `waitedAnswers` (same file) now folds with `feed.QuestionWithdrawReadPolicy(task, io.Discard)`, as `orchestrator/ready.go` does for the gate. By reading the code (`questions.go:224-231`, default withdraw roles owner and member), the default fold ignores a guest's withdrawal that the task's `workflow.md` allows under `gate_post_roles.withdraw`, so the question would stay open and row 4 could not be reached. I did not run the guest test without this line. Owner and member withdrawals count as before.
- `internal/job/render.go` `answersBlock`: a `withdrawn` answer with empty text writes only the line, no quote and no empty fence. Other kinds keep their shape.
- New `internal/job/jobcmd/rr_answers_withdrawn_test.go` (`rrAwBrief`, three tests).

## NOT VERIFIED on a live screen

Nothing in the UI changed. The brief was checked only through `job render --json` in tests, not with a real agent run.
