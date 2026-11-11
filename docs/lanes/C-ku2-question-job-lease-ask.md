# C-ku2-question-job-lease-ask: proof table

Base: 35a498dcf910235896a0a700a0ebe94e269065ea (origin/main = main; `git merge main`: "Already up to date.").

## Base check

- cli/question.go:24-29 (base): the ask writer is `feedWriter(env, GRID_LEASE, GRID_LEASE_EPOCH)`; question/command.go:214-223 sets it as Writer and ControlWriter. Neither file reads GRID_TASK_EPOCH (grep: only orchestrator.go, deliver_keeper_job.go, reopen_job.go and tests do).
- An unleased cli question post passes Store.Post: leaseLock returns nil for an empty lease (feed/store.go:126-129), and the default person gate roles include cli (workflow/gates.go:86). Refusal text is at store.go:221.
- postQuestionJobOnce (cli/question.go, base :80-87) posts only for lease "task" or a person writer; postsJob (question/job.go:59-67 on base) refused cli.

## Rows

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | TestKuJaOrchestratorAskPostsPersonJob | HOLDS | `go test -count=1 -run 'TestKuJa' -v ./internal/cli/` exit 0: `--- PASS: TestKuJaOrchestratorAskPostsPersonJob`. Base: `ku_job_lease_ask_test.go:50: ask by the Job agent with GRID_TASK_EPOCH: exit 1 question: role worker may not post person gates ; jobs on the Feed: 1` |
| 2 | TestKuJaLaterJobWaitsOnTheAnswer | HOLDS | same run: `--- PASS: TestKuJaLaterJobWaitsOnTheAnswer`. Base: `:74: ask by the Job agent: exit 1 job "" question: role worker may not post person gates` then `:81: post J-3 by the Job agent: exit 3 J-3: after target "J-2" does not exist; post J-2 first` |
| 3 | TestKuJaStaleTaskEpochPostsNothing | HOLDS | same run: `--- PASS`. The test asserts stderr carries feed.ErrStaleLease ("stale lease"), so the refusal is the task write's own; no question post, only J-1. Base: `:97: ask with the stale task epoch 0.1: want the stale-lease refusal, got exit 1 question: role worker may not post person gates` |
| 4 | TestKuJaWithdrawKeepsItsWriter | HOLDS | same run: `--- PASS`, log `withdraw with and without GRID_TASK_EPOCH: exit 3 "question: Q-1 role \"worker\" may not withdraw\n"`. Passes on base too, by design: it pins unchanged behaviour (a task writer would skip the withdraw policy and exit 0, so the comparison would fail). |
| 5 | Unchanged tests after `git merge main` | HOLDS | `go test -count=1 ./internal/cli -run 'TestKb3QAskByWorkerIsRefusedNoJob\|TestQuestionAsk\|TestQuestionWithdraw\|TestJwRp\|TestJwLo\|TestJobPost' -v` → `ok internal/cli 4.427s` (all `--- PASS`, incl. TestKb3QAskByWorkerIsRefusedNoJob). `go test -count=1 ./internal/question/` → `ok 3.589s`. |
| 6a | make lint | HOLDS | `make lint` exit 0 |
| 6b | make cross | HOLDS | `make cross` exit 0 (darwin/arm64, windows/amd64 vet and test compile) |
| 6c | walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` → `--- PASS` both, `ok`; `go test -count=1 ./internal/job/lint/` → `ok` |
| 6d | `git diff main --stat` | HOLDS, with two named extra files | Lists cli/question.go, question/command.go, the new test file, and question/job.go. job.go was added to Touches by the re-file. This report (docs/lanes/) is required by the runbook. |
| R | Re-file row: TestQuestionAskByAgentPostsNoJob | HOLDS (command corrected) | As written, `go test ./internal/question/ -run TestQuestionAskByAgentPostsNoJob -v` exits 0 but prints `testing: warning: no tests to run`. The test lives at internal/cli/question_job_test.go:140 and passes there: `--- PASS: TestQuestionAskByAgentPostsNoJob`. In internal/question, TestAskWithoutJobBehindIt (ask_job_test.go:80-91; subtests agent writer and cli writer, Post called 0 times) passes in `go test -count=1 ./internal/question/`. |

## Rules

| Rule | Result | Proof |
|------|--------|-------|
| Tests of changed packages and importers | HOLDS | go list importers of internal/cli or internal/question: cmd/grid, contract, contract/formats, internal/{account, braincmd, cli, feed, golden, jobgraph, mcp, orchestrator, packaging, question, settings, task}, lib. `go test -count=1` on all of them: every package `ok` (cmd/grid has no test files). |
| New tests fail on base | HOLDS for rows 1-3 | Failing lines quoted above. Row 4 pins unchanged behaviour, so it passes on base by design. |
| No existing test edited, none deleted | HOLDS | diff touches no existing _test.go file |
| No guard, gate, cap, hold or blocking hook added | HOLDS | No new refusal, flag, verb or error code. A stale epoch is refused by the existing leaseLock (store.go:143-144). |
| No hsIpTable entry, no new command | HOLDS | TestHsIpWalksEveryCommand passes; no table edited |
| Small commits, no attribution | HOLDS | 873f8cd9, 2190da30, 6045236d, plus this report |
| RealPlugin / UI | n/a | No RealPlugin test and no UI row in this brief |

## What changed

- internal/question/command.go: `Env.AskWriter func() (feed.Writer, error)`. When it is set, `ask` uses it as Writer and ControlWriter. When it is nil, ask stays on Writer(true). withdraw and tick always use Writer(true).
- internal/cli/question.go: AskWriter. When GRID_LEASE is set and is not "task", and GRID_TASK_EPOCH is set, it returns `orchestratorWriter(env, "task", GRID_TASK_EPOCH)`. A typed global --lease-epoch is used instead of GRID_TASK_EPOCH, as at deliver_keeper_job.go:117-122. In every other case it returns today's writer.
- internal/question/job.go postsJob: a cli writer posts a person Job only when both the writer and its ControlWriter hold lease "task".
  - This is narrower than the brief's "ControlWriter lease is task". The existing TestAskWithoutJobBehindIt/cli_writer pairs an unleased cli Writer with a task-lease owner ControlWriter (question_test.go:39-42) and expects no Post.
  - With the ControlWriter-only rule that test failed: `ask_job_test.go:102: Post called 1 times`.
  - In production both writers are the same (command.go sets both), so the narrower rule changes nothing for a task-holder ask.
- The owner default now applies: an app or person caller with GRID_ROLE=cli and GRID_LEASE=task also gets a person Job.

## NOT VERIFIED

- Nothing needs a live screen. The whole-module test and the cross-repo set are run by the orchestrator at landing.

## Open items

- The re-file row's command names the wrong package (see row R).

DONE
