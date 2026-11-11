# C-jw-question-lock-order: no question verb waits for .orchestrator.lock while it holds the questions lock

Lane head before this report: 2ffb7241 (merge of origin/main 17efd4a3 into the lane). Base: origin/main 17efd4a3.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1 Ask cycle (cli) | HOLDS | `go test -count=1 -v -run 'Question\|Answer\|Ask\|Withdraw\|TestJwLo' ./internal/cli/` exit 0: `--- PASS: TestJwLoAskWaitsForThePassWithoutTheQuestionsLock`. The test holds `<task>/feed/.orchestrator.lock` on its own fd, waits for the OnBlockForTest event on `.orchestrator.lock`, then runs `question.Store{TaskDir, TaskID}.ReserveGate(1, ...)`; a `questions/.lock` event fails it with "lock cycle". On the lane ReserveGate returns, the pass lock is let go, ask exits 0 and its --json has a `Q-n` id and a Job (internal/cli/jw_question_lock_order_test.go:134-150). Base (tests-only commit ca98b28b): `jw_question_lock_order_test.go:139: lock cycle: the holder's ReserveGate waits on questions/.lock while grid question ask holds it and waits on .orchestrator.lock` |
| 2 Answer cycle (cli) | HOLDS | Same command, exit 0: `--- PASS: TestJwLoAnswerWaitsForThePassWithoutTheQuestionsLock`; the answer exits 0 and Q-1.md says answered (internal/cli/jw_question_lock_order_test.go:152-163). Base: `jw_question_lock_order_test.go:156: lock cycle: the holder's ReserveGate waits on questions/.lock while grid answer holds it and waits on .orchestrator.lock` |
| 3 Port walk (question) | HOLDS | `go test -count=1 -v -run TestJwLo ./internal/question/` exit 0: `--- PASS: TestJwLoJobPortRunsWithQuestionsLockFree`. Post and Close open questions/.lock on a new fd and TryLockExclusive it; ask, answer, repeated answer, withdraw and a defaulting tick each make exactly one port call, none with the lock held (internal/question/jw_lock_order_test.go:73-118). Base, red for every verb: `ask: the Job port ran while questions/.lock was held` / `answer: ...` / `repeated answer: ...` / `withdraw: ...` / `tick default: the Job port ran while questions/.lock was held` (all at jw_lock_order_test.go:115) |
| 4 Package tests | HOLDS | After `git merge` of origin/main: `go test -count=1 ./internal/question/ ./internal/orchestrator/` exit 0 (`ok .../internal/question 5.348s`, `ok .../internal/orchestrator 91.338s`); `go test -count=1 -v -run 'Question\|Answer\|Ask\|Withdraw\|TestJwLo' ./internal/cli/` exit 0, no `--- FAIL`, with `--- PASS: TestJwLoAskWaitsForThePassWithoutTheQuestionsLock` and `--- PASS: TestJwLoAnswerWaitsForThePassWithoutTheQuestionsLock`; `--- PASS: TestJwLoJobPortRunsWithQuestionsLockFree` |
| 5 lint, cross, diff | HOLDS | `make lint` exit 0 (go vet ./...); `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile). `git diff main --stat` (local main set to origin/main 17efd4a3) names only internal/question/question.go, feed.go, job.go, jw_lock_order_test.go and internal/cli/jw_question_lock_order_test.go, plus this report, which the runbook itself asks for |

| Rule | Verdict | Proof |
|---|---|---|
| Small commits, one sentence, no attribution | HOLDS | `git log origin/main..HEAD`: ca98b28b (tests), f9f7925b (fix), 2ffb7241 (merge), and the report commit; no trailer lines |
| New tests fail on base, pass on lane | HOLDS | Failing lines pasted in rows 1-3, run on ca98b28b (tests only, code as main); passing lines in row 4 |
| No existing test edited or deleted | HOLDS | `git diff main --stat` lists only the two new test files among tests |
| No guard, gate, cap, hold or blocking hook | HOLDS | The change only moves Job port calls outside questions/.lock; it adds no wait, refusal or limit |
| Tests of changed package and importers | HOLDS | Importers of internal/question from `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...`: internal/cli, internal/feed, internal/orchestrator, internal/question, internal/task. After the merge: `go test -count=1 ./internal/question/ ./internal/orchestrator/ ./internal/feed/ ./internal/task/ ./internal/cli/` exit 0, all five `ok` |
| Lint, Windows and macOS builds | HOLDS | row 5 |
| No sleep; timer only as failure path (30 s); hook tests not parallel, hook restored by t.Cleanup | HOLDS | internal/cli/jw_question_lock_order_test.go:19 (jwLoDeadline, used only in failure branches), :27 (`t.Cleanup(fslock.OnBlockForTest(...))`, hook compares the path and sends without blocking); no t.Parallel in either new file |
| Shared files untouched (reserve.go, events.go, internal/cli/question.go) | HOLDS | `git diff main --stat` |
| Helpers `jwLo*`, tests `TestJwLo*`, new tests only in new files | HOLDS | jwLoPort, jwLoLocks, jwLoHoldPass, jwLoCycle, jwLoDeadline |
| `git merge main` before last proof run | HOLDS | 2ffb7241 merges origin/main 17efd4a3; rows 4 and 5 were run on it |
| ask/answer/withdraw output keeps its shape | HOLDS | The verbs return the same Record; row 1 reads the --json id and Job; the existing golden and command tests in internal/question and internal/cli pass unchanged (row 4) |
| UI rows / RealPlugin | n/a | The brief has no UI row and no RealPlugin test |

## What changed

- `internal/question/question.go:146-191` `add`: an ask that posts a person Job (`withJob`, a port and a person writer) takes the lock, projects, picks Q-n and writes the `Q-n.feed` marker as ReserveGate does; lets the lock go; posts the Job through the port; takes the lock again and publishes the question with its Job. `Add`, an agent's ask and a cli ask post no Job and keep the single lock as before.
- `internal/question/question.go:328` `change` (answer, withdraw): the decision posts and the file stay under the lock; `settleJobs` runs after the lock returns, for the new decision and for a repeated one alike.
- `internal/question/question.go:376` `Tick`: defaults and aging are posted and saved under the lock; the Jobs of every record it changed are settled after, each even when another fails (errors joined; before, the first Close error stopped the loop).
- `internal/question/feed.go:119-121` `publishEvents` no longer closes the Job; it posts and saves.
- `internal/question/job.go:57-96`: `postsJob` (the check `postJob` made), `settleJobs` (the after-lock Close loop).

## Outcome after a crash between the phases

- Ask, crash after the marker and before the Job post: only `questions/Q-n.feed` is left, a gap in Q ids (the gap ReserveGate documents). Today nothing is left.
- Ask, crash after the Job post and before the question post: a person Job with no question post naming it, which is what a crash between the two posts leaves today. The marker now keeps the next ask off Q-n, so the orphan Job's title names an id no other question takes (today the next ask reuses Q-n).
- A failed Job post now also leaves the marker (a gap); `TestAskPostErrorLeavesNothing` still holds (no Q-1.md, no question post).
- Answer or withdraw, crash after the decision and the file and before the Job's result: the Job stays open, as today after a crash between the two posts; the same `grid answer` or `withdraw` again closes it (settleJob repeats safely). The file is now already written.
- Tick default, crash after the default and before the Job's result: the Job stays open and a later Tick does not repair it (Tick skips a record that has a defaulted event, question.go:350), the same as today; a person's later answer closes it.

## NOT VERIFIED

Nothing needs a live screen: no UI in this lane.

## Open items

- internal/orchestrator/ready.go:5-10 already states the order this lane makes true; it was not edited.
- The local `main` branch of the clone was moved to origin/main so that `git diff main --stat` compares against the base the bundle is cut from.

DONE
