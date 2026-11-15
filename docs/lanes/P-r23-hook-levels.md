# P-r23-hook-levels: stored hook levels take effect

Base: origin/main (0f360eaa; `git merge origin/main` on the lane was already up to date). Findings: U-chk-plugin-rev-cli row 2; U-chk-plugin-rev-skills rows 15 and 16.

## Proof table

| Row | Result |
| --- | --- |
| 1a. cli row 2: `job-lint`, `needs-you`, `session-check` stored, no reader | confirmed on main. `grep -rn` over non-test Go for the three names finds only the catalog (`internal/settings/catalog.go:31`, `:99`) and docs/settings.md:121-122, :163-165 ("has no effect yet", "currently changes no runtime behavior"); `docs/lanes/L64.md:18-23` says "None" for all three. |
| 1b. fix `job-lint` | HOLDS. `internal/job/jobcmd/command.go` (render, `jobLintLevel`): block refuses (default, as main), warn says each finding on stderr and renders, off skips lint; unreadable settings keep the refusal and say why. Doc line updated at docs/settings.md:121, and the job-lint words dropped from :163-165. |
| 1c. fix `needs-you` | NOT DONE. No Go code consumes it: there is no Notification event in `hooks/hooks.json` or `internal/hook/events.go`, and no bridge POST in non-test `internal/`. The only needs-you code is the Feed's `needs-you.json` projection (`internal/feed/fold.go:131`), which the app reads as authoritative; a level that skips it leaves a stale file. Wiring it is a design choice for the owner, not a refit of existing code. Its "has no effect yet" line stays, because dropping it would be false. |
| 1d. fix `session-check` | NOT DONE. The old check (`docs/old-plugin-map.md:30`, `golden/session-check/*`) was the retired `session-check.mjs`; no Go code does it, and `hooks/` holds only hooks.json. A new SessionStart warning at default `warn` would change every session's context, a new behaviour with its own tests. Its doc line stays. |
| 2. skills row 16, over-limit deny follows plan-guard level | HOLDS. `internal/hook/hook.go` (`overLimitDecision`): block denies, warn (the default) warns and lets the call through, off is silent; no Grid home or unreadable settings keep the deny. Existing `TestOversizedEventIsRefusedForGuardedTools` is unchanged and still passes (it runs with no Grid home). Doc sentence added at docs/settings.md:163-167. |
| 3. skills row 15, PreToolUse history lock wait | Measured, not changed. `go test -v -run 'TestKr2HistorySkipsWhenSessionLockBusy\|TestKr2SlowHookLogsOneLine' ./internal/hook/` shows the wait is the 2.0 s `historyLockWait` and only while another session holds `sessions/.lock`; with the lock free the record takes milliseconds (`TestKr2HistoryRecordsWhenSessionLockFree`, `TestKr2FastHookLogsNoSlowLine`). The ceiling is 2 s against a 30 s hook timeout, so a tool call is never cut off. Shortening it would change 4 existing tests that pin 2 s (`kr2_history_wait_test.go:159,24,271`), which the lane may not edit; a follow-up lane can set the new bound and those tests together. |
| 4. test per fixed row, failing on main, passing on head | HOLDS. See below. |
| 5. go test of changed packages and importers | HOLDS. `go list` importers of `internal/hook` and `internal/job/jobcmd`: `internal/cli`, `internal/packaging`. `go test -count=1 ./internal/hook/ ./internal/job/jobcmd/ ./internal/cli/ ./internal/packaging/`: all four `ok` (exit 0). |
| 6. `make lint` | HOLDS. Output tail is `go vet ./...` with no finding. |
| 7. Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin CGO_ENABLED=0 go build ./...` both exit 0. |
| 8. Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `ok  internal/cli 0.438s`. `go test -count=1 ./internal/job/lint/`: `ok 0.277s`. |
| 9. `git fetch origin && git merge origin/main` before the last run | HOLDS. Merge was clean and a no-op (main had not moved). |
| 10. No guard, gate, cap or hold added | HOLDS. Both changes loosen a refusal main already had; no level adds one. |
| 11. Tests of the packages the brief forbids | `make realplugin` guest walk tables are app-repo checks: not run, not asked. |

## Tests

- `internal/job/jobcmd/job_lint_level_test.go`: `TestRenderFollowsTheJobLintLevel` (default, block, warn, off) and `TestRenderKeepsTheRefusalWhenHookSettingsAreUnreadable`.
  On main (`git stash` of the two production files):
  - `job_lint_level_test.go:59: render at job-lint "warn": exit 1, want 0`
  - `job_lint_level_test.go:59: render at job-lint "off": exit 1, want 0`
  - `job_lint_level_test.go:80: exit 1, want 1 with the settings error`
  On the head: `--- PASS: TestRenderFollowsTheJobLintLevel` (4 subtests) and `--- PASS: TestRenderKeepsTheRefusalWhenHookSettingsAreUnreadable`.
- `internal/hook/overlimit_level_test.go`: `TestOverLimitDenyFollowsThePlanGuardLevel` (pretooluse and permissionrequest, each of default, warn, off, block).
  On main: `overlimit_level_test.go:43: plan-guard "warn": deny=true want false` (and the same for the default and off, both events). On the head: all 8 subtests pass.

## What changed

- `internal/job/jobcmd/command.go`: render reads the `job-lint` level.
- `internal/hook/hook.go`: the over-limit deny reads the `plan-guard` level.
- `docs/settings.md`: job-lint row and the inactive-names paragraph.
- Two new test files.

## NOT VERIFIED on a live screen

Nothing here is a UI row. A real Claude Code session sending a call over 1 MiB at the default level now gets a warning instead of a deny; that was not run in a live session.

## Open

`needs-you` and `session-check` (rows 1c, 1d) need an owner decision on what each should do before they can be wired; the PreToolUse wait bound (row 3) needs the four kr2 tests changed together.
