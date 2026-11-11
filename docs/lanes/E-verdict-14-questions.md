# E-verdict-14-questions: a question is a person Job

Brief: verdicts-11nov v-14. Repo plugin, branch `lane-E-verdict-14-questions`, from main 38a3d1b2.

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1 | Map of today's ask, answer, withdraw | HOLDS, below (file:line on main). |
| 2 | `grid question ask` posts a person Job, dependents only wait, answer posts its result, withdraw closes it | HOLDS with one reading made explicit: a Job record cannot be amended, so the Jobs that need the answer name the question Job (or `gate:Q-N`) in their own `after`. The question Job's own `after` is empty (`internal/cli/question_job_test.go` asserts it). Choices ride on the question post the Job names (Job record has no choices field, `contract/` is outside the touch list). `command.go:64`, `question.go:131-170`, `job.go`, `internal/cli/question.go:36-95`. |
| 3 | questions/<person>.md is a read-only view of the Job; nothing reads it back as truth | PARTLY. HOLDS for a task with a Feed: `save()` (`files.go:137`) writes `questions/Q-N.md` from the folded Feed record after the posts; state, answer and history come from the fold (`feed.go:137-210`). NOT DONE: `readFiles` (`files.go:187`) still lists a task with no task ULID from its files, and `project()` keeps a legacy file with no Feed post as "not in the Feed". Existing tests pin both; I did not edit them. The file is named `Q-N.md`, not `<person>.md`, because every app reader keys on `Q-n.md` and matches the id to the filename (below); a person-named file would break them. |
| 4 | Go tests: ask, answer, withdraw round trip; a Job outside the after list is ready while the question is open | HOLDS. `TestQuestionAskIsPersonJobAnswerClosesIt` (J-4 ready, J-3 waits while open; after answer and the holder's pass J-2 accepted, J-3 assigned), `TestQuestionWithdrawClosesTheJob`, `TestQuestionAskByAgentPostsNoJob` in `internal/cli/question_job_test.go`. |
| 5 | go test and make lint 0 fail | Packages I changed and importers: HOLDS (below). Whole-module test left to the orchestrator at landing. |
| R1 | New tests fail on base | HOLDS. On main: `--- FAIL: TestQuestionAskIsPersonJobAnswerClosesIt  question_job_test.go:54: ask 2: question: unknown action "ask"` (same for the other two, exit 1). |
| R2 | No existing test edited or deleted | HOLDS. `git diff --stat origin/main..HEAD` touches no `_test.go` except the new file. |
| R3 | No guard, gate, cap, hold on agents | HOLDS. An agent or cli writer gets no Job from `ask` (`job.go` postJob) and no new refusal; the Feed's own "may not post person gates" for an agent is unchanged from main for `add`. Nothing waits on a Job unless that Job lists it in `after`. |
| R4 | Tests of changed packages and importers | HOLDS. `go list` importers of internal/question: cli, orchestrator, question, task. `go test ./internal/question ./internal/orchestrator ./internal/task -count=1` exit 0; `go test ./internal/question ./internal/cli -count=1` exit 0 (run before `gofmt -w`, which only moved whitespace). |
| R5 | make lint | HOLDS, exit 0 after `gofmt -w internal/question/job.go internal/question/question.go` (the first run failed on those two files). |
| R6 | Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| R7 | Commits small, one sentence, no attribution, no personal paths | HOLDS. |
| R8 | Budget | OVER: about 65 tool calls against 60. |

## Row 1: today's paths on main

| Path | Writes a file | Posts a Feed record | App reads |
|------|---------------|---------------------|-----------|
| ask (`grid question add`) | `save()` `files.go:137`, after the post | type `question` with a person gate, `feed.go:32-65` via `Store.Add` `question.go:131` | via the file |
| answer (`grid answer <task> Q-N`, `RunAnswer` `command.go:40`) | rewritten by `save()` | type `answer`, `feed.go:76-81` via `Store.Answer` `question.go:177` and `change` `question.go:239` | the app calls `grid answer` (app `api/ops.yaml:215`, `:2110`) and reads the file |
| withdraw (`grid question withdraw`) | rewritten by `save()` | type `control` with `question_event: withdrawn`, `feed.go:82-90` via `Store.Withdraw` `question.go:204` | the app calls it (app `api/ops.yaml:2267`) and reads the file |

Files read back today: `project()` `feed.go:212-247` (reserve ids, keep legacy files), `readFiles` `files.go:187` (no-Feed listing), `nextID` `reserve.go:31` (filenames only).
App readers (app.git main, unchanged): `internal/kernel/questionfile/questionfile.go` (`Parse`, front matter JSON, `job` is "the Person Job the question belongs to", id must equal the filename), `internal/slices/taskview/store.go:148`, `internal/slices/gates/model.go:408,436`, `internal/slices/asks/read.go:98,186`, `internal/slices/notifications/read.go:360,421`, `internal/slices/me/setup.go:123,129`.

## What changed

- `internal/question/job.go` (new): `JobPort`, the person Job body (role Person, kind `ask`, owner = the person asked, `verify` = file-exists on the view), `postJob`, `settleJob`.
- `question.go`: `Store.Ask`, `Store.Jobs`; a repeated answer or withdraw still settles the Job. `feed.go`: `publishEvents` settles the Job after posting an answer, default or withdrawal, then writes the file last.
- `command.go`: `ask` action (same flags as `add`, prints the Job id); `add` stays the plain question with no Job.
- `internal/cli/question.go`: wires the port to the orchestrator (`PostPerson`, `AnswerPerson`). The question package cannot import the orchestrator (cycle), so the CLI supplies it.
- A person of the team answers or withdraws without the lease, so the Job is accepted by the holder's next pass (the existing unleased person-answer path); a lease holder closes it at once.

## Open items

- Withdraw closes the Job as accepted with the result `withdrawn: <reason>`, not cancelled, so the Jobs behind it carry on; a cancelled Job would exhaust them.
- New Job id is the next free top-level `J-N` in the Feed; a plan posted later with the same id would be ignored by the fold. Needs an id scheme from the plan owner.
- Row 3 remainder: removing the file-listing fallbacks needs the owner to retire the legacy-file tests.
- `<person>.md` filename: kept `Q-N.md`, needs the owner's word if `<person>.md` is wanted (the app readers would change).
- A guest answer does not close the Job (a guest is not a person of the team); the question itself closes.

## NOT VERIFIED on a live screen

The app's Needs-you list showing the new Job and Q-N together; `grid question ask` in a real session.
