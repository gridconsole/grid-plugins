# C-tc-approve-press-unanswered-first

A press on a later approve step lands on that step, not on an earlier one that
waits for the holder's tick.

Base: plugin main `c7c10553c3ed4dc33e917c153c2a383939554a42`. Base check:
`git grep -n "func planPersonJob" main -- internal/cli/orchestrator.go` shows
`main:internal/cli/orchestrator.go:522`; the loop (:535-539) returns the first Job
that is not Accepted, Cancelled or Rejected and never reads an answer, so the
brief's premise holds and the lane went ahead.

## Proof table

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | `TestTcPressLaterStep`: two presses, no tick between, the Feed holds one answer re J-2's first record and one re J-3's | HOLDS | `go test -count=1 -v -run 'TestTcPress' ./internal/cli/` exit 0: `--- PASS: TestTcPressLaterStep (0.34s)`; test at `internal/cli/tc_approve_press_test.go:92`. Red on base, see below. |
| 2 | `TestTcPressLaterNotReady`: J-3 has `after` J-1 (not accepted); second press exits 0 and writes no new answer | HOLDS | same run: `--- PASS: TestTcPressLaterNotReady (0.19s)`; asserts exit 0, 1 answer in the Feed, 1 re J-2, 0 re J-3. It also passes on base: the brief marks only row 1 red on base, and this row pins the repeat-press branch (`approve_record.go:83-86`) against the new rule. |
| 3 | `TestTcPressTickAccepts`: after row 1 the holder's tick leaves J-2 and J-3 accepted | HOLDS | same run: `--- PASS: TestTcPressTickAccepts (0.42s)`; asserts both statuses are `accepted` and one accepting control each. Red on base, see below. |
| 4 | `go test -count=1 ./internal/cli/` exits 0 with the three named tests passing | HOLDS | `go test -count=1 ./internal/cli/` exit 0 (`ok ... 40.855s`). `go test -count=1 -v -run 'TestApproveRepeatByAPersonWithoutLeaseWritesAndPostsNothing\|TestApproveRecordPersonWithoutLeaseWaitsForTheHoldersTick\|TestApproveWaitsForTheAfterJob' ./internal/cli/` exit 0: all three `--- PASS`. |
| 5 | `make lint` exits 0 and `make cross` exits 0 | HOLDS | `make lint` (`go vet ./...`) exit 0. `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test-compile, every package `ok`. `gofmt -l` on both changed files prints nothing. |
| R1 | New tests fail on base and pass on the lane | HOLDS | Base production code (`c7c10553`) with the new test file, `go test -count=1 -v -run 'TestTcPress' ./internal/cli/` exit 1: `--- FAIL: TestTcPressLaterStep (0.08s)` / `tc_approve_press_test.go:98: the Feed holds 1 answers, want 2: [{Record:{... Type:answer ... Re:0x29f867346e20 ...}}]` and `--- FAIL: TestTcPressTickAccepts (0.18s)` / `tc_approve_press_test.go:140: J-3 is "posted" after the tick, want accepted`. On the lane the same command exits 0 with three PASS. |
| R2 | No existing test edited or deleted | HOLDS | `git diff --stat origin/main..HEAD` before this report: `internal/cli/orchestrator.go` (+36 -8) and `internal/cli/tc_approve_press_test.go` (new, +146) only. |
| R3 | Edits only inside the brief's Touches | HOLDS | Same diff: `planPersonJob` (`orchestrator.go:526`), one helper `personJobAnswered` beside it (`orchestrator.go:562`), the new test file. `approve_record.go` is not changed. This report is the docs file the runbook asks for. |
| R4 | Helper and test names | HOLDS | Package-level names in the new file: type `tcPressRig`, func `tcPressSetup`; tests `TestTcPressLaterStep`, `TestTcPressLaterNotReady`, `TestTcPressTickAccepts`. |
| R5 | Packages that import the changed package, tested | HOLDS | `go list` of importers of `internal/cli`: `cmd/grid`, `lib`; of tests that import it: `contract/formats`, `internal/account`, `internal/braincmd`, `internal/golden`, `internal/jobgraph`, `internal/mcp`, `internal/packaging`, `internal/question`, `internal/settings`. `go test -count=1 ./cmd/grid/ ./lib/` exit 0 (`cmd/grid` has no test files); `go test -count=1 ./contract/formats ./internal/account ./internal/braincmd ./internal/golden ./internal/jobgraph ./internal/mcp ./internal/packaging ./internal/question ./internal/settings` exit 0, all `ok`. |
| R6 | Plugin walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok ... 0.038s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok ... 0.063s`). The app-side walk tests belong to the app repo, which this lane does not touch. |
| R7 | `*RealPlugin*` tests | N/A | The brief names no RealPlugin test and no row depends on a built plugin; no skipped test is claimed as a pass. |
| R8 | No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change only chooses which open approve person Job a person's `grid approve` and `--send-back` name; no agent path is touched. |
| R9 | Commits | HOLDS | One-sentence messages, no attribution lines, no personal paths, hosts or emails in the diff or this report. |

## What changed

`planPersonJob` (`internal/cli/orchestrator.go:526`) now walks the approve person
Jobs in Feed order and returns the first open one that has no answer yet
(`personJobAnswered`, `:562`: a record of type `answer` whose `re` is the Job's
first history record) and for which `orchestrator.PersonReady(snap, id, now)`
returns nil. When no open Job meets both it returns what it returned before: the
first open one, else the last accepted one. An open Job that waits for the
holder's tick is therefore passed over for a later, ready, unanswered one.

`recordApproval` is unchanged: a repeat press for a plan whose answer is already
in the Feed still finds it (`answered`, `approve_record.go:42-56`) and posts
nothing (`:83-86`). With J-3 not ready (row 2) nothing meets both conditions, so
the press falls back to J-2 and takes that branch.

Side effect worth the reviewer's eye: `sendBackPlan` (`task_write.go:312`) also
calls `planPersonJob`, so a `--send-back` press now also lands on the first open
approve Job that has no answer and is ready, not on one that already holds a
person's answer. That is the same rule applied to the same question (which step
does this press mean) and the existing send-back tests in `./internal/cli/` still
pass.

An answer counts whoever wrote it and whatever plan hash it carries (the brief's
definition). An answer the holder's pass will never use (empty or over the length
limit, or from a non-addressee) still counts as answered, so a press passes over
that Job to a later ready one; if none exists it falls back to the first open Job.

## NOT VERIFIED on a live screen

Nothing here has a screen. Not run: the app's real-plugin rigs that press more
than once (gates, digest, hq replay approve land). The brief says they run on a
task with one approve person Job, where this lane changes nothing; the
orchestrator runs them at landing.

Verdict: DONE. No NOT DONE row.
