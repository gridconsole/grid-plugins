# M-C-yc-worker-stopped-requeue-onto-main: lane C-yc-worker-stopped-requeue merged onto plugin main f09dda3a

Repo: plugin. Branch lane-M-C-yc-worker-stopped-requeue-onto-main. Lane head before the merge: be5fbfe2. Merged: origin/main f09dda3a.

## The conflict and how it was resolved

One file conflicted: contract/docs/feed-posts.md, the worker-exit paragraph (:735). Main had rewritten the usage_limit sentence (its `code` is the reset time in unix seconds, and the Job is offered again once that time has passed or an account has room). The lane had added the `worker_stopped` sentence. Resolution keeps both: the lane's `worker_stopped` sentence stays, and main's usage_limit sentence replaces the older one. Nothing of either side was dropped. internal/orchestrator/tick.go auto-merged; `git diff origin/main -- internal/orchestrator/tick.go` is the lane's 3 added lines only, and main's `m.Failures = nil` in `case "cancelled":` (tick.go:299) is kept.

## Proof table

| # | Row / rule | Result | Command, exit code or file:line |
|---|---|---|---|
| 1 | `git merge-base --is-ancestor origin/main HEAD` | HOLDS | exit 0 |
| 1b | `git diff origin/main --stat` names only lane files | HOLDS | contract/docs/feed-posts.md, docs/lanes/C-yc-worker-stopped-requeue.md, internal/orchestrator/tick.go, yc_worker_stopped.go, yc_worker_stopped_test.go (261 insertions, 1 deletion), plus this report |
| 2.1 | original row 1: cancelled + `worker_stopped`: Ready, control reason "worker stopped", no orchestrator job | HOLDS | `go test -count=1 -v -run TestYcWs ./internal/orchestrator/` exit 0: `--- PASS: TestYcWsStoppedIsReady` (yc_worker_stopped_test.go:84-91). See the note in docs/lanes/C-yc-worker-stopped-requeue.md on "is Ready" (the same tick offers the Job again). |
| 2.1b | the new tests are red without the lane change | HOLDS | The `if ycWorkerStopped(...)` branch removed from tick.go, same command, red: `yc_worker_stopped_test.go:91: last control for J-1 = map[close_reason:worker cancelled reason:worker cancelled status:cancelled target:J-1], want status ready, reason worker stopped`, `--- FAIL: TestYcWsStoppedIsReady`, `--- FAIL: TestYcWsStoppedOfferedAgainNeverParked`, `--- FAIL: TestYcWsStoppedClearsCrashCount`. `git checkout -- internal/orchestrator/tick.go`, `git status --short` empty. |
| 2.2 | original row 2: offered again with higher fence; repeated stops never park | HOLDS | `--- PASS: TestYcWsStoppedOfferedAgainNeverParked`, `--- PASS: TestYcWsStoppedClearsCrashCount` |
| 2.3 | original row 3: no code, prepare_failed, number 0 stay Cancelled; no code + J-2 waiting posts one re-plan Job | HOLDS | `--- PASS: TestYcWsOtherCancelledStaysFinal`, `--- PASS: TestYcWsHelper` |
| 2.4a | `go test -count=1 -run 'TestGk\|TestGq\|TestUsageLimitReoffer' ./internal/orchestrator/` | HOLDS | exit 0 (`ok ... 15.268s`) |
| 2.4b | `go test -count=1 ./internal/orchestrator/ ./internal/contract/` | HOLDS | exit 0 |
| 2.4c | `git grep -n worker_stopped -- contract/docs/feed-posts.md` | HOLDS | contract/docs/feed-posts.md:735, in the worker-exit section (:732) |
| 2.5 | `make lint`, `make cross` | HOLDS | both exit 0 (see rows 4) |
| 3a | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0 |
| 3b | `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0 |
| 3c | App-only walks (`make webui-test`, TestFvArgvCollects, TestEveryWiredMount, slices Imports) | NOT APPLICABLE | not in this repo |
| 4a | `make lint` | HOLDS | exit 0 (`go vet ./...`, no output) |
| 4b | `make cross` (darwin/arm64 and windows/amd64 vet and test compile) | HOLDS | exit 0; no FAIL or `make ... Error` line in its output |
| R1 | packages that import the changed package | HOLDS | `go list` shows internal/cli, internal/mcp, internal/migrate import internal/orchestrator. `go test -count=1 ./internal/mcp/ ./internal/migrate/ ./internal/orchestrator/ ./internal/contract/` exit 0; `go test -count=1 ./internal/cli` exit 0 (108.7s) |
| R2 | no existing test edited or deleted; nothing removed | HOLDS | merge diff against origin/main adds one new test file and edits no existing test |
| R3 | no guard, gate, cap, hold or blocking hook added | HOLDS | one branch in the existing cancelled case; a Job whose worker keeps stopping is offered again every time |
| R4 | commit rules | HOLDS | one merge commit, one-sentence message, no attribution lines; no personal path, host or email |
| R5 | RealPlugin tests | NOT APPLICABLE | the brief names none |

## What changed in this lane

Only the merge commit on top of be5fbfe2: contract/docs/feed-posts.md:735 now carries both main's usage_limit sentence and the lane's worker_stopped sentence. This report is added.

## NOT VERIFIED on a live screen

Nothing here is a UI change. A real `grid-worker stop` against a running Job needs the app worker from C-yc2-worker-posts-stopped.

Verdict: DONE
