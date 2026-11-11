# C-pcov-question-ask-job: proof table

Change: one new file, `internal/question/ask_job_test.go`. No non-test file changed.
Coverage of `internal/question` (Linux, `go test -cover ./internal/question/ -count=1`): before 79.6%, after 85.3%.

| Row | Status | Proof |
|---|---|---|
| 1 Ask with Post returning J-7 | HOLDS | `TestAskPostsPersonJobThenQuestion` (ask_job_test.go:46): Record Job J-7, ID Q-1; Post body kind "ask", role "Person", intent.owner "alice", verify.path "questions/Q-1.md"; Post gets the lease writer as lease and the person writer as person; `list` shows the question post with Job J-7. `go test ./internal/question/ -count=1` exit 0. |
| 2 nil Post, agent, cli | HOLDS | `TestAskWithoutJobBehindIt`: no Post call and Record.Job "" in all three. Outcome per writer: nil Post (owner) publishes the question; **agent**: Ask returns the Feed's refusal `role agent may not post person gates` and no question is posted; **cli**: the question is published (err nil, `list` shows Q-1 with no Job). |
| 3 Post error | HOLDS | `TestAskPostErrorLeavesNothing`: Ask returns the Post error (`errors.Is`), no questions/Q-1.md, `list` empty. |
| 4 jobBody | HOLDS | `TestJobBody`: 200-rune question gives a title of exactly 120 runes ending "…" (multi-byte runes); 120 runes kept whole, 121 cut; tabs/double spaces collapsed in title, goal only trimmed; item 0 and -1 give 1, 1 gives 1, 3 gives 3; also intent.item, acceptance A1, verify file-exists, kind, role. |
| 5 settleJob via Answer | HOLDS | `TestSettleJobViaAnswerAndWithdraw`: Answer calls Close once with (J-7, "JSON"); repeat answer calls Close again with the same text and the history stays at 1 event; Withdraw closes with "withdrawn: Not needed"; Tick default closes with the default text; Add (no Job) and Close nil make no call. |
| 6 ReserveGate | HOLDS | `TestReserveGate`: empty folder gives Q-1 and a Q-1.feed marker of size 0, mode 0600 (mode check skipped on windows); Q-2.md present gives Q-3, minimum 5 gives Q-5; publish error returned, marker stays, next call gives Q-2; two goroutines get {Q-1, Q-2}; Q-9223372036854775807.md gives "question id range exhausted", publish not called, no .feed marker (skipped when int is not 64-bit). |
| 7 RunAnswer | HOLDS | `TestRunAnswerUsageAndAnswer`: `--help` and `-h` print usage to stdout, exit 0; `["T-1","Q-1"]` prints usage to stderr, exit 2; full call on the fixture task exits 0 and `list` shows Q-1 answered "JSON". |
| 8 DefaultWithdrawAllowed | HOLDS | `TestDefaultWithdrawAllowed`: owner, member true; guest, agent, cli, "" false. |
| 9a go test, 0 fail | HOLDS | `go test ./internal/question/ -count=1`: ok, exit 0. |
| 9b coverage | HOLDS | After 85.3% (before 79.6%, measured on the untouched clone). `go tool cover -func`: jobBody 100.0%, postJob 100.0%, settleJob 87.5%, ReserveGate 87.5%, nextID 93.8%, Ask 100.0%, add 88.0%, RunAnswer 100.0%, DefaultWithdrawAllowed 100.0%. The marker-write failure (reserve.go:24-26) stays uncovered. `withdrawPolicy` stays 66.7% (the brief lists it, no row asks for it). |
| 9c Windows/macOS builds | HOLDS | `GOOS=windows go test -c -o /dev/null ./internal/question/` exit 0; `GOOS=darwin go test -c -o /dev/null ./internal/question/` exit 0. Running them needs a Windows/macOS host: compiled, needs Windows run and needs macOS run (same commands without `-c -o /dev/null`). |
| 9d make lint | HOLDS | `make lint` (go vet ./...) printed no finding, exit 0; `gofmt -l internal/question` empty. |
| Importers' tests | HOLDS | Importers by `go list`: internal/cli, internal/orchestrator, internal/task. `go test -count=1 ./internal/question/ ./internal/orchestrator/ ./internal/task/ ./internal/cli/`: all ok, exit 0. |
| New tests fail on base | NOT APPLICABLE | This is a coverage lane for existing behaviour and changes no code, so the tests pass on base as well. The base fact: before the file, Ask, jobBody, postJob, ReserveGate and DefaultWithdrawAllowed were at 0.0% in the brief's figures. I did not run a mutation of the code (editing a non-test file is outside the brief). |
| No existing test edited | HOLDS | `git diff --stat origin/main..HEAD`: one added file. |
| No guard/gate/cap on agents | HOLDS | Tests only. |
| Small commit, no attribution lines | HOLDS | One commit, one-sentence message, no trailer. |

## What changed
`internal/question/ask_job_test.go` (400 lines): a recording `JobPort` stub and tests for Ask, jobBody, postJob, settleJob, ReserveGate, RunAnswer and DefaultWithdrawAllowed. Existing helpers (setup, input, add, list, commandEnv) are reused, not edited.

## Findings
No code bug found. Worth knowing: with an agent writer, Ask posts no Job and the Feed refuses the question post, so Ask returns an error (`role agent may not post person gates`) rather than a plain question. job.go:61-62 says the Job is skipped so that the question post is not left without its question, and the Feed's refusal means the caller gets an error. That matches the code, so it is pinned as is.

## NOT VERIFIED on a live screen
Nothing here is UI. The Windows and macOS test runs need those hosts.
