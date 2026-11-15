# P-r23-hook-levels: stored hook levels take effect

Base: origin/main (0f360eaa). `git fetch origin && git merge origin/main` before the last proof run: "Already up to date." Findings: U-chk-plugin-rev-cli row 2; U-chk-plugin-rev-skills rows 15 and 16. Fix round 1: review-P-r23-hook-levels.md points 1 and 2 and its four notes.

## Proof table

| Row | Result |
| --- | --- |
| 1a. cli row 2: `job-lint`, `needs-you`, `session-check` stored, no reader | Confirmed on main. `grep -rn` over non-test Go for the three names finds only the catalog (`internal/settings/catalog.go:31`, `:99`) and docs/settings.md:121-122, :163-165 ("has no effect yet", "currently changes no runtime behavior"). `docs/lanes/L64.md:18-23` says "None" for all three. |
| 1b. Fix `job-lint` | HOLDS. `internal/job/jobcmd/command.go` (render, `jobLintLevel`): block refuses (the default, as on main), warn writes each finding to stderr and renders, off skips lint. Unreadable settings keep the refusal and say why. The doc line is at docs/settings.md:121, and the job-lint words are dropped from the inactive-names paragraph. |
| 1c. `needs-you` and `session-check` | NOT BUILT. This is the brief's own exit (fix round 1), not a fault. No spec line says what either level drives. `docs/needed.md` does not exist in this repo (`ls docs`). `grep -rn -i "needs-you\|session-check"` over `docs/decisions`, `plan.md`, `agents.md` and `delivery.md` finds nothing. `docs/lanes/L64.md:19,23` says "None / No effect yet" for both. `docs/old-plugin-map.md:30,47` describes the retired `session-check.mjs` and `needs-you.mjs` scripts, not a level. Both names and their "have no effect yet" line stay as on main (docs/settings.md:122, and the inactive-names paragraph at :167-168). The missing spec line is a docs/settings.md row that names the hook each level drives. |
| 2. skills row 16: the over-limit deny follows the plan-guard level | HOLDS. `internal/hook/hook.go` (`overLimitDecision`): block denies. warn (the default) warns and lets the call through. off writes nothing. With no Grid home or unreadable settings it still denies. At warn and off no decision is written for PermissionRequest either: `hookio.Decision.Bytes` (internal/hookio/hookio.go:191-203) writes nothing for an empty decision and only `systemMessage` for warnings, and the test now asserts there is no `behavior` or `permissionDecision`. The doc sentence moved into the plan-guard text (docs/settings.md:138-141). |
| 3. skills row 15: the PreToolUse history lock wait | Confirmed on main, and FIXED. On main, PreToolUse with another session holding `sessions/.lock` took 2.00 s (pasted below). Fix: `internal/hook/history.go` takes the lock for PreToolUse through the new `session.Store.UpdateActivityNow` (`internal/session/activity.go`, `files.go` `lockedNow`, using the existing `fslock.TryLockExclusive`). When the lock is busy, the record is skipped at once with one log line, `history skipped: session lock busy`, so a tool call never waits on it. PostToolUse runs after the call and keeps its `historyLockWait` of 2 s; that is unchanged and not asked. |
| 3a. Existing tests changed by row 3 | Two tests pinned the 2 s PreToolUse wait that row 15 removes. The direction to remove it is in the brief Do 2, U-chk-plugin-rev-skills.md:28 and review-P-r23-hook-levels.md point 2. `TestKr2HistorySkipsWhenSessionLockBusy` was removed. `TestKr2PreToolUseHistoryDoesNotWaitForSessionLock` (new) covers the same checks: one skip line, no activity change, no history change, the same decision. Only the 2 s wait became "under 1 s". `TestKr2SlowHookLogsOneLine` was removed. It made a call slow by holding the lock, which a PreToolUse no longer waits for. The slow line is still covered by `TestLtLogSlowSlowWritesOneLine` (internal/hook/lt_log_slow_test.go:29). In `kr2_history_wait_test.go:22`, `kr2Skip` lost its "after 2s" tail. `TestKr2PostAfterSkippedPreWritesNothing` uses it and is otherwise unchanged. A PostToolUse with the lock held could not stand in for these tests: on main it already waits without a bound in another PostToolUse path (5 s test bound hit). That is outside this brief and listed under Open. |
| 4. Test per fixed row: fails on main, passes on the head | HOLDS. See Tests below. |
| 5. go test of changed packages and importers | HOLDS. The changed packages are `internal/hook`, `internal/session` and `internal/job/jobcmd`. `go list` importers: `internal/cli`, `internal/hook`, `internal/lease`, `internal/orchestrator`, `internal/packaging`, `internal/task`. `go test -count=1 ./internal/hook/ ./internal/session/ ./internal/job/jobcmd/ ./internal/cli/ ./internal/lease/ ./internal/orchestrator/ ./internal/packaging/ ./internal/task/`: all 8 `ok` (exit 0). |
| 6. `make lint` | HOLDS. Exit 0. Output is `go vet ./...` with no finding. |
| 7. Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin CGO_ENABLED=0 go build ./...` both exit 0. |
| 8. Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `ok internal/cli 0.467s`. `go test -count=1 ./internal/job/lint/`: `ok 1.394s`. |
| 9. `git fetch origin && git merge origin/main` before the last run | HOLDS. "Already up to date." |
| 10. No guard, gate, cap or hold added | HOLDS. Rows 2 and 1b loosen a refusal main already had. Row 3 removes a wait. No level adds a refusal. |
| 11. Review notes (fix round 1) | HOLDS. The comment at hook.go:472 is reworded for block, warn and off. PermissionRequest at off and warn: see row 2. The over-limit sentence moved into the plan-guard text. The live run is listed below as NOT VERIFIED. |
| 12. Shots at 390/1440/3840 and px reads (fix round 1) | Not run: this brief has no UI row and asks for no px reads. It changes Go and docs only. |
| 13. App guest walk tables (`make realplugin ...`) | Not run, not asked. They are app-repo checks, and this lane is in the plugin repo. |
| 14. Files touched | Inside the brief: `docs/settings.md`, `internal/hook`, `internal/job/jobcmd`, and this doc. `internal/session` (`activity.go`, `files.go`) gains one additive method, `UpdateActivityNow`. Row 3's fix forces it: the session lock is only reachable through `session.Store`. |

## Tests

- `internal/job/jobcmd/job_lint_level_test.go`: `TestRenderFollowsTheJobLintLevel` (default, block, warn, off) and `TestRenderKeepsTheRefusalWhenHookSettingsAreUnreadable`.
  On main:
  - `job_lint_level_test.go:59: render at job-lint "warn": exit 1, want 0`
  - `job_lint_level_test.go:59: render at job-lint "off": exit 1, want 0`
  - `job_lint_level_test.go:80: exit 1, want 1 with the settings error`

  On the head: `--- PASS` for both tests.
- `internal/hook/overlimit_level_test.go`: `TestOverLimitDenyFollowsThePlanGuardLevel` (pretooluse and permissionrequest, each at default, warn, off and block).
  On main: `overlimit_level_test.go:43: plan-guard "warn": deny=true want false`, and the same for the default and off, for both events.
  On the head: `--- PASS: TestOverLimitDenyFollowsThePlanGuardLevel (1.02s)`, all 8 subtests PASS.
- `internal/hook/pretooluse_lock_now_test.go`: `TestKr2PreToolUseHistoryDoesNotWaitForSessionLock`.
  On main (`git stash push` of history.go, activity.go and files.go):
  ```
  --- FAIL: TestKr2PreToolUseHistoryDoesNotWaitForSessionLock (2.00s)
      pretooluse_lock_now_test.go:28: PreToolUse took 2.002745255s with the lock held, want under 1 s
      pretooluse_lock_now_test.go:31: 0 skip lines in the log, want 1:
          grid hook: history skipped: session lock busy after 2s
          grid hook: hook PreToolUse slow: 2002 ms
  ```
  On the head: `--- PASS: TestKr2PreToolUseHistoryDoesNotWaitForSessionLock (0.05s)`. The other kr2 tests, `TestKr2HistoryRecordsWhenSessionLockFree`, `TestKr2PostAfterSkippedPreWritesNothing` and `TestKr2FastHookLogsNoSlowLine`, all PASS.

## What changed

- `internal/job/jobcmd/command.go`: render reads the `job-lint` level.
- `internal/hook/hook.go`: the over-limit deny reads the `plan-guard` level, and its comment is reworded.
- `internal/hook/history.go`: PreToolUse takes the session lock without waiting.
- `internal/session/activity.go`, `files.go`: `UpdateActivityNow` and `lockedNow`.
- `docs/settings.md`: the job-lint row, and the over-limit sentence in the plan-guard text.
- Tests: three new test files, plus the kr2 edits in row 3a.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Two behaviours were not run in a live Claude Code session, only in unit tests:

- A call over 1 MiB at the default level now gets a warning instead of a deny.
- A PreToolUse no longer pauses while another session holds the lock.

## Open

- `needs-you` and `session-check` stay NOT BUILT until a spec line names what each level drives (row 1c).
- PostToolUse still has a session-lock wait without a bound in a path other than `bashHistory`. The 5 s test bound was hit when the lock was held. This is outside this brief and needs a follow-up finding.

DONE
