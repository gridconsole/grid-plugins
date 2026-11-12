# C-bs-tick-settles-defaulted

Base check: on main (cdbef50c) `internal/question/question.go:392` is `if slices.ContainsFunc(r.History, func(e Event) bool { return e.Kind == "defaulted" }) {` followed by `continue`, and `:418` is `if e := s.settleJobs(changed...); e != nil {`. Tick did not settle earlier-defaulted records, so the brief holds.

## What changed

- `internal/question/question.go` Tick: inside the locked read, a record with `State == "defaulted"`, a `defaulted` event and a Job that the loop skips is collected in `stranded`. After the lock is let go, one `settleJobs` call settles `stranded` together with `changed`. `changed` still holds only newly recorded events. No Job port call runs under questions/.lock.
- `internal/question/bs_tick_settles_defaulted_test.go`: `TestBsTsSecondTickClosesDefaultedJob`.
- `internal/orchestrator/bs_tick_settles_defaulted_test.go`: `TestBsTsOrPassClosesStrandedPersonJob`, `TestBsTsOrThirdPassPostsNothing`.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 `TestBsTsSecondTickClosesDefaultedJob` red on base, green on lane | HOLDS | base (question.go stashed): `--- FAIL: TestBsTsSecondTickClosesDefaultedJob` / `bs_tick_settles_defaulted_test.go:65: closes [], want [[J-7 CSV]]`; lane: `--- PASS` |
| 2 `TestBsTsOrPassClosesStrandedPersonJob` red on base, green on lane | HOLDS | base: `person Job J-1 is posted after the second pass, want accepted` / `J-1 accepted with answer "" (accepted false), want the default "Keep parked"` / `J-2 is posted, want assigned`; lane: `--- PASS` |
| 3 third pass posts nothing, Q-1.md byte-equal | HOLDS | `TestBsTsOrThirdPassPostsNothing` `--- PASS` (rnQdAbout unchanged, bytes.Equal on questions/Q-1.md); red on base at the second-pass precondition (`person Job J-1 is posted after the second pass, want accepted`) |
| 4 named tests pass, shared tests unedited | HOLDS | `go test -count=1 -v ./internal/question ./internal/orchestrator -run 'TestSettleJobViaAnswerAndWithdraw\|TestJwLoJobPortRunsWithQuestionsLockFree\|TestRnQd\|TestBsTs'` exit 0: `--- PASS:` for TestSettleJobViaAnswerAndWithdraw, TestJwLoJobPortRunsWithQuestionsLockFree, TestRnQdPassDefaultsDueQuestionAndReleasesJobAfterPerson, TestRnQdSecondPassRecordsNothingNew, TestRnQdPassRaisesAgingTwiceThenStops, TestRnQdPassLeavesDecidedAndUnpostedQuestionsAlone, TestRnQdCloseRunsWithBothLocksFree and the three TestBsTs tests; `git diff main --stat` shows no edit to ask_job_test.go, jw_lock_order_test.go, rn_question_defaults_test.go |
| 5a `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| 5b `make cross` | HOLDS | exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| 5c `go test -count=1 ./internal/question/ ./internal/orchestrator/` | HOLDS | exit 0 (question 2.5s, orchestrator 111s) |
| 5d `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand'` | HOLDS | exit 0 (run with TestIlGuestSiteList, `ok`) |
| 5e `git diff main --stat` | HOLDS | question.go, the two new test files, and this lane doc only |
| Importers of internal/question (`go list`) | HOLDS | internal/cli, internal/orchestrator, internal/question, internal/task; `go test -count=1 ./internal/cli` exit 0 (36s), `./internal/task` exit 0 |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| Merge of origin/main before the last run | HOLDS | `git fetch origin && git merge origin/main`: Already up to date |
| Lock order | HOLDS | Close still runs after `s.locked` returns; TestJwLoJobPortRunsWithQuestionsLockFree and TestRnQdCloseRunsWithBothLocksFree pass |
| No guard, cap, counter or cache added | HOLDS | diff has none |

## NOT VERIFIED on a live screen

Nothing here has a screen. The app rig `fj_question_defaulted_realplugin_test.go` and the guest walk tables (`make realplugin`) belong to the app repo and were not run; the brief says the plugin train only and the rig's questions have no Job.
