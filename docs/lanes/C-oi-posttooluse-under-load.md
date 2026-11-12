# C-oi-posttooluse-under-load

Base: main at 792b8d6e (the brief names 7a4bc0c2; at the base events.go:42 was `{Event{"PostToolUse", "", 10}, remaining}`, the default branch ran heartbeat, history, session, brain read and then `job message` at :118-124, and pe_job_message.go:46 called `.Read(`; so not yet done).

## Proof table

| Row | Result |
|---|---|
| 1 TestOiPtMessageWithoutReaderLock | HOLDS. `go test -count=1 -v -run 'TestOiPt' ./internal/hook/` exit 0: `--- PASS: TestOiPtMessageWithoutReaderLock`, `--- PASS: TestOiPtJobHasResultWithoutReaderLock`, `--- PASS: TestOiPtJobMessageFirst`. On the base plus only the nil seam and its call in `record`: `oi_posttooluse_load_test.go:109: PostToolUse is about to wait for feed/.cache/reader.lock; the hook Feed read must not take it` / `--- FAIL: TestOiPtMessageWithoutReaderLock`; `oi_posttooluse_load_test.go:130: jobHasResult is about to wait for feed/.cache/reader.lock...` / `--- FAIL: TestOiPtJobHasResultWithoutReaderLock`. |
| 2 TestOiPtJobMessageFirst | HOLDS (same run). PostToolUse records `job message, heartbeat, history, session, brain read`; Stop records `heartbeat, history, session, brain read`. On the base: `oi_posttooluse_load_test.go:151: PostToolUse records ["heartbeat" "history" "session" "brain read" "job message"], want ["job message" "heartbeat" "history" "session" "brain read"]`. |
| 3 timeout 30 | HOLDS. `go test -count=1 -run TestGuardHookTimeoutsSurviveLoad -v ./internal/hook/`: `--- PASS: TestGuardHookTimeoutsSurviveLoad` (only the PostToolUse want changed, 10 to 30). `grep -n -A6 '"PostToolUse": \[' hooks/hooks.json` shows line 20 `"timeout": 30`. `grep -c '"timeout": 10' hooks/hooks.json`: 6 on the base, 5 now. `make generate` then `git diff --exit-code hooks/hooks.json` exit 0 (generate touched only hooks.json). |
| 4 hook and feed tests | HOLDS. `go test -count=1 ./internal/hook/ ./internal/feed/ ./internal/cli/ ./internal/packaging/` exit 0 (hook 12.2s, feed 23.0s, cli 92.6s, packaging 6.2s). Importers of internal/hook by `go list`: internal/cli, internal/packaging. Also `go test -count=1 ./internal/fslock/ ./internal/hook/` exit 0 after fixing a comment (see below). |
| 5 lint, cross, walks | HOLDS. `make lint` exit 0 (go vet ./...). `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile). `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. `gofmt -l internal/hook` empty. |
| Small commits, no attribution | HOLDS. One commit, one sentence, no attribution lines. |
| New tests fail on base, pass on lane | HOLDS, lines above. No existing test edited except the one PostToolUse want the brief allows. No test deleted. |
| No guard, gate, cap or hold on agents | HOLDS. Nothing added. |
| RealPlugin tests | Not applicable: the brief names none. |
| Other repo-wide walks (app only) | Not applicable to the plugin repo. `TestIfLwSeamOnlyInTests` in internal/fslock first failed because my comment in events.go named `fslock.OnBlockForTest`; I reworded the comment and it passes. |

## What changed

- internal/hook/events.go: PostToolUse 10 to 30 and the comment; for posttooluse the `job message` record runs before heartbeat, history, session and brain read (other events keep their order); `jobHasResult` uses `ReadOnly`; new nil-in-production seam `oiRecordSeenForTest`, called at the top of `record`.
- internal/hook/pe_job_message.go: Feed read uses `ReadOnly`.
- hooks/hooks.json: regenerated with `make generate`.
- internal/hook/zj_timeouts_test.go: PostToolUse want 30.
- internal/hook/oi_posttooluse_load_test.go: the three TestOiPt tests. The lock tests hold reader.lock with fslock.LockExclusive and use fslock.OnBlockForTest to signal; they are bounded by `t.Deadline()` less a margin, with no sleep and no timing assertion.

## NOT VERIFIED on a live screen

Claude Code's behaviour with the 30 s timeout on a loaded machine is not run here (no claude in the sandbox).

## Open items

None.

Verdict: DONE
