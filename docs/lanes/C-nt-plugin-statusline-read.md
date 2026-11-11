# C-nt-plugin-statusline-read: `grid account statusline --json` with no word reads

Base: 00413d82 (plugin main). On base, with the new test file copied in, `go test -count=1 -v -run TestNtSl ./internal/account/` fails:

    nt_statusline_read_test.go:36: read: exit 2: usage: grid account statusline on|off [--json]
    --- FAIL: TestNtSlReadAfterOn
    nt_statusline_read_test.go:45: read: exit 2: usage: grid account statusline on|off [--json]
    --- FAIL: TestNtSlReadChangesNothing
    nt_statusline_read_test.go:104: read of "{not json": exit 2, want 1: ... usage: grid account statusline on|off [--json]
    --- FAIL: TestNtSlReadRefusesBadFile

## Proof table

| Row | Result |
|---|---|
| 1. TestNtSlReadAfterOn red on base, green on lane | HOLDS. Base failure above. Lane: `go test -count=1 -v -run TestNtSl ./internal/account/` exit 0, `--- PASS: TestNtSlReadAfterOn` |
| 2. TestNtSlReadChangesNothing | HOLDS. Same run, `--- PASS: TestNtSlReadChangesNothing`: no file gives on:false exit 0 and the file is still absent; on, off, own command answer on, off, off with kept = their command; bytes equal before and after each read; plain output `account statusline off: <path>` |
| 3. TestNtSlReadRefusesBadFile | HOLDS. `--- PASS: TestNtSlReadRefusesBadFile`: `{not json` and `"type":"static"` exit 1 with the write's reasons ("is not valid JSON", "statusLine is not a command status line"), bytes unchanged |
| 4. `go test -count=1 ./internal/account/ ./internal/settings/ ./internal/cli/` | HOLDS, exit 0 (also ./internal/doctor ./internal/guard ./internal/hook, the other importers of account or settings per go list; internal/updates has no tests). Includes TestHsIpWalksEveryCommand (walked 39 commands, 39 table entries) and the contract tests of internal/cli. `git merge main`: Already up to date |
| 5. `make lint` / `make cross` | HOLDS, both exit 0 (go vet; darwin/arm64 and windows/amd64 vet and test compile) |
| New tests fail on base, pass on lane | HOLDS, see the paste above |
| No existing test edited to pass; no test deleted | One edit, named in the brief: ei_statusline_test.go in TestStatusLineRefusals. The old assertion pinned `account statusline` with no word to exit 2, which the read replaces. The file at that point holds `{not json` (written by the line above), so the read would exit 1 there, not 0 with "on":false. I put `h.writeSettings("{}")` before the call and the call is now `statusline --json`, expecting exit 0 and `"on":false`. The `maybe` assertion is kept at exit 2. The bad-file read is covered by TestNtSlReadRefusesBadFile |
| Walk tests | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok (also TestSettingsStoreChokePoint ok); `go test -count=1 ./internal/job/lint/` ok |
| No guard, gate, cap or hold added | HOLDS: the read only reports |
| No personal path, host or email in the repo | HOLDS: tests use t.TempDir |
| Edits inside Touches | HOLDS with one deviation, below |

## What changed

- internal/account/statusline.go: `StatusLineRead(env Env) (StatusLineResult, error)`. On is true only when the statusLine command is Grid's (unwrapNote); Kept is the wrapped command, or the person's own.
- internal/cli/account.go: no word means read; help line `statusline [on|off] [--json]` plus one line that no word reads; the usage line for a bad word is now `usage: grid account statusline [on|off] [--json]` (exit 2 stays).
- internal/account/nt_statusline_read_test.go: the three tests.
- internal/account/ei_statusline_test.go: the one assertion above.

## Deviation: no new settings helper

The brief asks for a read-only helper beside EditClaudeStatusLine in internal/settings/claude_statusline.go. I wrote one (ReadClaudeStatusLine), and `TestSettingsStoreChokePoint` (internal/cli/settings_choke_test.go) failed: `unreviewed settings export ReadClaudeStatusLine`. That test pins every exported settings function to a list. Fixing it means editing a test outside Touches, which the rules forbid, so I removed the helper and put claude_statusline.go back to base. `StatusLineRead` calls the already-listed EditClaudeStatusLine with a change function that returns the command unchanged, or "remove" when there is no statusLine. Both cases return before any write (claude_statusline.go:54-55), and a missing file is read as `{}`, so no file is created. The tests check the bytes and the absent file. If the owner wants a named read helper, the follow-up is one export entry in settings_choke_test.go plus the helper.

## NOT VERIFIED on a live screen

Nothing here is a UI. The app's Settings > Devices switch reading this answer is the app brief WAIT-C-nt-app-statusline-read; not run here.
