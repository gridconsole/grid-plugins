# P-hookevents-keep-3h: keep at least 3 hours of hook events on a busy account

Why: plugin check 2026-11-16T0250Z issue 1. The event log kept 3 files of 256 KiB (about 7,500 lines); a lane account writes about 200 lines a minute, so the log covered about 40 minutes, less than the hourly check window.

## What changed

- `internal/hookevents/hookevents.go:23-31`: `Limit = 2 << 20`, `Files = 3` (was `256 << 10`). At most 6 MiB an account, about 60,000 lines at about 100 bytes a line, over 3 hours at 250 lines a minute. The comment gives the figure and the reason.
- `internal/hookevents/hookevents_test.go` (new): three tests.
- `internal/hook/ruleruns_test.go:169`: outside the brief's Touches list, forced by the change. `TestRuleRunsLogIsBounded` hardcoded `const limit = 256 << 10` to seed the active file at the limit; with the new Limit it failed (see the proof table). The one line is now `const limit = 2 << 20`. The test is not weakened: it still checks the same shift, the dropped oldest copy and the fresh active file. The file does not import `hookevents` on purpose (it pins the file format), so the number is written out.

## Proof table

| Row | Result |
|---|---|
| A1. Limit and Files keep 3 hours; comment updated | HOLDS. `hookevents.go:23-31`, `Limit = 2 << 20`, `Files = 3`. `TestLimitKeepsThreeHoursAtBusyRate` PASS. |
| A2. More than Limit bytes written; `Read` (the read `grid hook events -all` uses, `internal/cli/hook_events.go:34`) returns lines from `.1` and `.2`, in time order | HOLDS. `TestReadReturnsShiftedCopiesInTimeOrder`: writes about 5 x Limit bytes one `Append` at a time; checks `.`, `.1`, `.2` exist, newest line is in `events.jsonl`, oldest line read is in `events.jsonl.2`, lines come newest first with no gap, `Read` holds at least 45,000 lines (3 h at 250 a minute), and the oldest lines were dropped. `go test -count=1 -v ./internal/hookevents/`: PASS, exit 0. |
| A3. Files never above Files x Limit plus one line | HOLDS. `TestFilesStayWithinFilesTimesLimitPlusOneLine`: samples the folder every 97 appends over about 5 x Limit bytes; never more than `Files` files, no file above Limit plus one line (120 bytes), total never above `Files` x Limit plus one line, and the log did fill at least `Files-1` x Limit. PASS. |
| A4. `go test ./internal/hookevents/ ./internal/logshift/` | HOLDS. `ok .../internal/hookevents`; `.../internal/logshift [no test files]` (the package has no tests; `hookevents` tests drive `logshift.Shift` and `Names`). Exit 0 on the first (the failure in that call came from `internal/hook`, see below). |
| A4. `make lint` | HOLDS. Exit 0 (`go vet ./...`, no output). Run on the final head. |
| New tests fail on base | HOLDS. With `hookevents.go` stashed back to `256 << 10`: `FAIL TestLimitKeepsThreeHoursAtBusyRate: Files x Limit holds about 7864 lines, need 45000 for 3 hours at 250 a minute` and `FAIL TestReadReturnsShiftedCopiesInTimeOrder: Read returned 8932 lines, 3 hours at 250 a minute is 45000`. The A3 test passes on base because the bound is a property of the mechanism, not of the number; it scales with `Limit`. |
| Importers' tests (`go list`: `hookevents` is imported by `internal/hook` and `internal/cli`; through them `cmd/grid`, `lib`, `internal/packaging`, `internal/packaging/cmd`) | HOLDS. `go test -count=1 ./internal/hookevents/ ./internal/hook/ ./internal/cli/ ./internal/packaging/... ./cmd/grid/ ./lib/`: all ok (`internal/cli` 145 s, `lib` 44 s). `internal/hook` first FAILED `TestRuleRunsLogIsBounded` ("copies not shifted ... the fresh log holds 3238 lines") because of the hardcoded 256 KiB; after the one-line change above it passes (`ok internal/hook 9.775s`). |
| Windows build | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| macOS build | HOLDS. `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. (The other walk tests in the runbook are app-repo tests.) |
| Merge origin/main before last run | HOLDS. `git fetch origin && git merge origin/main`: Already up to date. |
| `make realplugin ...` guest walk tables | not run, not asked (app repo only). |
| Whole-module test, cross-repo set | not run, not asked (orchestrator at landing). |

## Open items

- `contract/README.md:667` still says "at 256 KiB ... (about 7,500 lines)". It is outside this brief's Touches; it should say 2 MiB and about 60,000 lines. `docs/lanes/P-hook-events-read.md:22,32` describe the old figures as history and are left as they are.
- The hookevents tests append about 130,000 lines one by one; about 3.5 s each alone, longer under load (33 s when run beside the `internal/hook` and `lib` packages).

## NOT VERIFIED on a live screen

Nothing here is UI. No hook was run against a real Grid home.
