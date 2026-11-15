# C-r10-reviewer-sees-person-answer: a reviewer reads the person's answer to a person Job

Repo: plugin. Proof runs on the merged head 5bee29e2 (lane commit 85df56a1 plus origin/main 1688462e).

## Finding (Do 1)

- A person Job's answer is written by `AnswerPersonWith` (internal/orchestrator/person.go:280). With the task lease, the person's words go into the control that closes the Job: `{target, status: accepted, answer}` (person.go:352). Without the lease, an `answer` record is written with re = the Job record (person.go:327-335), and the holder's next pass closes the Job with a control that copies the text (person.go:114). For a plan sent back, the closing control is a rejected one with `send_back` (person.go:269).
- None of these records has the `job` field set, and none is of type `result`. `grid feed read --job J --type result` (internal/cli/feed.go:113, MCP internal/mcp/tools.go:193) filters on `value(e.Job)` and `e.Type` (internal/feed/read.go:204 on base), so it returned nothing for a person Job. This is r10 P3.
- A Job whose `after` names the person Job directly already gets the answer in its brief: `waitedAnswers` (internal/job/jobcmd/command.go:637-718) feeds the "Answers this Job waited for" block (internal/job/render.go:516). A reviewer reads a Job's outcome with `grid feed read --task <task> --job <job> --type result` (agents/reviewer.md:15). The orchestrator's deliver and re-plan steps do the same (agents/orchestrator.md:48,50). That read path missed the answer.

## Choice (Do 2)

The read side: `grid feed read --job <J>`, with or without `--type result`, now also returns the control that closed person Job J with the answer. That is the same record `waitedAnswers` takes as the answer: target J, a string `answer`, and status accepted, or rejected with `send_back`. It is returned only when J's Job record has role Person (internal/feed/read.go `personClosings`). Why not close the person Job with a result record: a `result` is Job-side and needs the Job lease (internal/cli/feed.go:288 `refuseJobSide`). It would change the fold in contract, and it would add a second copy of the answer next to the closing control. The read side writes nothing, so the answer stays in one record. The unleased `answer` record is not returned as well, so the reader sees the text once. A builder Job's read is unchanged: the person check excludes the control that `yvPamControl` writes on the builder Job (person.go:168-170).

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| Do 1: where the answer is recorded and where a dependent or reviewer reads it | HOLDS | file:line above |
| Do 2: the decision is reachable by `feed read --job <J> --type result`, one record of the answer | HOLDS | internal/feed/read.go `applyFilter` + `personClosings`; commit 85df56a1 |
| Do 3 and acceptance: new test fails on main | HOLDS | on base 301d1185 with only the test file: `go test -count=1 ./internal/orchestrator -run TestRsPersonAnswer` exit 1: `--- FAIL: TestRsPersonAnswerReadAsResultLeased (0.27s)` `rs_person_answer_result_test.go:56: the dependent Job reading J-1 by result wants one record with the answer, got 0: []` `--- FAIL: TestRsPersonAnswerReadAsResultUnleased (0.27s)` `rs_person_answer_result_test.go:68: the dependent Job reading J-1 by result wants one record with the answer, got 0: []` |
| Acceptance: new test passes on the lane | HOLDS | `go test -count=1 -v ./internal/orchestrator -run TestRsPersonAnswer` exit 0: `--- PASS: TestRsPersonAnswerReadAsResultLeased` `--- PASS: TestRsPersonAnswerReadAsResultUnleased`. The orchestrator package passes again on 5bee29e2 (below) |
| Acceptance: `make lint` | HOLDS | `make lint` (go vet ./...) exit 0 on 5bee29e2 |
| Acceptance: tests of changed packages and importers | HOLDS | Changed: internal/feed (code), internal/orchestrator (test). `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... \| grep -E ' .../internal/(feed\|orchestrator)( \|$)'` gives cmd/grid approvefix braincmd cli doctor env guard hook job job/jobcmd job/jobtest job/lint jobgraph land lease mcp migrate orchestrator packaging packaging/cmd ports question signal task updates usage lib. `go test -count=1 ./internal/feed` plus all of those: exit 0, every package ok (cmd/grid, approvefix, job/jobtest, updates have no test files) |
| Acceptance: plugin walk tests | HOLDS | after the merge: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok, exit 0; `go test -count=1 ./internal/job/lint/` ok, exit 0 |
| Rule: Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0 on 5bee29e2 |
| Rule: merge origin/main before the last proof run | HOLDS | `git fetch origin && git merge origin/main` merged 1688462e (contract oneof change). Walk tests, importers, lint and builds above were rerun after it |
| Rule: app guest walk tables (`make realplugin ... TestLcHostRealPlugin\|...`) | not run, not asked | app-repo rule; this lane is in the plugin repo |
| Rule: no guard, gate, cap, hold or blocking hook | HOLDS | the change is a read filter only |
| Rule: no existing test edited | HOLDS | `git diff origin/main --stat` shows only the new test file, internal/feed/read.go and this doc |
| Whole-module test, cross-repo set | not run, not asked | run by the orchestrator at landing |

## What changed

- internal/feed/read.go: `applyFilter` also selects, for `--job J`, the controls `personClosings` names. Those are the controls that closed person Job J with the person's answer, and they count as J's `result` for `--type result`. Other type filters still match on the record's own type.
- internal/orchestrator/rs_person_answer_result_test.go: person Job J-1, with J-2 waiting on it, is answered with the lease and without it. J-1 read by result returns exactly one counted record carrying the answer. A worker Job with no result still reads none.

## Open items

- The record returned keeps its own type, `control`, with body `{target, status, answer, reason}`. A reader looking for `body.text` or `body.summary`, as in a builder's result, finds the decision in `body.answer`. The reviewer agent text was not changed.
- The r10 task files under the eval grid home could not be read from this lane (the read was refused). The finding comes from the code and the report quote in the brief.

## NOT VERIFIED on a live screen

Nothing; no UI.

DONE
