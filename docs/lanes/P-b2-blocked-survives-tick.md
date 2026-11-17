# P-b2-blocked-survives-tick

Lane P-b2-blocked-survives-tick, repo plugin, on main 5d7c2242 plus a6c5cb02 (merge of origin/main: already up to date).
Not verified on a live launcher.

## Proof table

| Row | Result |
|---|---|
| 1 `go test -count=1 -v ./internal/orchestrator/ -run 'TestB2BlockedSurvivesTick\|TestB2DoneStillAwaitsJudgement'` | HOLDS, exit 0: `--- PASS: TestB2BlockedSurvivesTick` (`exit_done`, `exit_failed`) and `--- PASS: TestB2DoneStillAwaitsJudgement`. (a), (b) assert the control-post count is the same before and after the tick and the Job is still Blocked; (c) a done exit with no blocked post is Submitted "awaiting judgement". internal/orchestrator/b2_blocked_tick_test.go |
| 1 fails on main for (a), (b) | HOLDS, see the mutation below (the tick.go line without Blocked is main's line) |
| 2 `go test -count=1 -v ./internal/contract/ -run TestB2StateOfBlocked` | HOLDS, exit 0: `--- PASS: TestB2StateOfBlocked`. internal/contract/b2_state_of_blocked_test.go |
| 3 `go test -count=1 ./contract/ -run TestT12Posts` | HOLDS, exit 0 (`ok .../contract`) |
| 3 `go test -count=1 ./internal/orchestrator/ ./internal/contract/ ./internal/board/ ./internal/job/jobcmd/` | see the line under the table |
| 4 `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0 |
| 4 `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0 |
| 5 `make lint` | HOLDS, exit 0 (`go vet ./...`) |
| 5 `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| Merge before the last run: `git fetch origin && git merge origin/main` | HOLDS: "Already up to date." |
| No attribution lines: `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` | HOLDS: prints 0 (see the end of the run notes) |
| `make webui-test`, app guest walk tables, `realplugin` | not run, not asked (plugin repo; the brief names no app run) |
| Windows and macOS builds | HOLDS via `make cross` above |

## Mutations (one per Go row)

- Drop `|| j.Status == contract.Blocked` from the early return in `advance` (internal/orchestrator/tick.go:288), which is main's line:
  `--- FAIL: TestB2BlockedSurvivesTick/exit_done`: `b2_blocked_tick_test.go:73: the tick wrote 1 control posts over a Blocked Job`; `exit_failed`: `the tick wrote 2 control posts over a Blocked Job` (Submitted "awaiting judgement" for done; Ready "retry after failed attempt" and another for failed). `TestB2DoneStillAwaitsJudgement` stays green. Restored with `git checkout`, then committed.
- Drop `case Blocked:` from `StateOf` (internal/contract/types.go):
  `--- FAIL: TestB2StateOfBlocked`: `b2_state_of_blocked_test.go:9: StateOf(Blocked) = "Not started", "", want Started, blocked`. Restored with `git checkout -- internal/contract/types.go`; `git status --short` clean.

## What changed

- internal/orchestrator/tick.go:288: `advance` returns early for Blocked as well, so the tick posts no control for a Blocked Job, whatever its exit reason.
- internal/contract/types.go `StateOf`: `case Blocked:` gives Started with reason "blocked". The fold sets `j.State, j.StateReason` from it (fold.go:349).
- Order controls on a Blocked Job: fold.go:272 refuses an order control unless `StateOf` is Not started. A Blocked Job now reads Started, so the order control is refused ("order changes only a Job that has not started"). Before, it read Not started and would have been accepted. Read from the code; no test of it in this lane.
- contract/docs/feed-posts.md: prose only. The stale "The fold has no case for it yet" line replaced at approval (:80, fold.go:528-547, grant check; no status change), blocked (:118, fold.go:510-513), exit (:306, fold.go:479-515), landed (:660, fold.go:298-312) and started (:860, fold.go:516-517).
- Left as is, listed: cancel (:156). The fold counts a cancel and moves nothing (fold.go:448 comment, no case in `foldAttempts`). Also left, outside the brief's six lines: integrated (:501), task-move (:930), tested (:966), waited (:1090), waiting (:1127), wake (:1179). I did not check them against the fold. The map's :1075 left alone as the brief says.

## Pins checked, not edited

- orchestrator/order_test.go TestFourStates has no Blocked row; contract/subset_time_test.go:86 and the StateOf callers (jobcmd/command.go:335, :378, ew_empty_finish_test.go:89) still pass: `go test` of the packages below.
- contract/t12_feed_posts_test.go: `TestT12Posts` passes (headings, registration lines and examples untouched).

## NOT VERIFIED on a live screen

No live launcher or app was run. A person approving a Blocked Job and the next assign are the existing fold paths (TestPlBd2AllowNeedsPersonApproval); not exercised with a launcher.
