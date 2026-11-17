# P-codex-windows-read

`grid account windows` answers the Codex "5h" and "7d" windows from the session logs Codex already writes. Before this, `windowsFor` gave every Codex account Source "none" and the note "Codex use is not read yet", so the app and the Settings Accounts page had no Codex reading.

## Source of a Codex reading

Quoted from the brief's Facts (read by the orchestrator on codex-cli 0.167.0 on 16 Nov 2026; the builder did not read other accounts' homes):

- Files: `<CODEX_HOME>/sessions/YYYY/MM/DD/rollout-<ISO time>-<uuid>.jsonl`, one JSON object per line. Lines can be long (max 70355 bytes in the last 20 files): no default bufio.Scanner buffer.
- Rate-limit lines are `{"timestamp":"2026-11-09T02:54:26.916Z","type":"event_msg","payload":{"type":"token_count","info":{...},"rate_limits":{...}}}`.
- A real reading (09 Nov 02:54Z): `"rate_limits":{"limit_id":"codex","limit_name":null,"primary":{"used_percent":100.0,"window_minutes":10080,"resets_at":1794778709},"secondary":null,"credits":{"has_credits":false,"unlimited":false,"balance":"0"},"individual_limit":null,"spend_control_reached":null,"plan_type":"pro","rate_limit_reached_type":null}`. `resets_at` is absolute Unix seconds; `used_percent` is 0-100.
- The newest files hold only lines like `"rate_limits":{"limit_id":"premium",...,"primary":null,"secondary":null,...}` (19 such lines, all newer than the last real one). They carry no window: skip them and go back to the newest line with a non-null primary or secondary, across older files if needed.
- In all 29600 real lines on the four accounts only `window_minutes` 10080 occurs (pro plan, no 5-hour window seen). Map by length: up to 360 minutes is "5h", 1440 or more is "7d"; primary and secondary are positions, not meanings.
- CODEX_HOME defaults to `~/.codex`; the account folders `windowsFor` gets already point there for a Codex account.

Only what Codex already writes is read: no new process, no network call, no wrapper. The fixtures in `internal/account/cx_codex_readings_test.go` keep these shapes: `cxRealLimits` (line 23) is the real reading verbatim, `cxPremium` (line 24) is the null-window shape, and the first test puts a premium line after the real one in the same file and a premium-only file newer than the file with the reading. A five-hour window was never seen on a real account, so the two-window fixtures are derived from the same shape with a 300 minute window (`TestCxFiveHourAndSevenDayBothRead`, `TestCxPositionsAreNotMeanings`).

## Proof table

| Row | Check | Result |
|---|---|---|
| 1 | The source is quoted above; `cx_codex_readings_test.go:23-24` hold the real and the premium shapes; `TestCxRealReadingGivesSevenDayAndPremiumLineIsSkipped` (line 73) has a premium line newer than the codex line | HOLDS |
| 2 | `windowsFor` returns "5h" and "7d" for a Codex account: `internal/account/windows.go:151-153` calls `codexWindowsFor` (`codex_read.go:27`). `go test -count=1 -v -run TestCx ./internal/account` exit 0, all `--- PASS`, incl. `TestCxFiveHourAndSevenDayBothRead`, `TestCxPositionsAreNotMeanings`, `TestCxWindowsAnswersACodexAccountFromItsFolder` (through `Windows()`). Red on base: below | HOLDS |
| 3 | No reading keeps Source "none" and the note: `TestCxNoReadingKeepsTheNoneAnswer` (no folder, no dirs, no sessions, premium-only, empty file, non-rollout file name). The Claude branch of `windowsFor` is untouched: `git diff origin/main -- internal/account/windows.go` is one 5-line insertion before the `for _, d := range dirs` loop. `go test -count=1 ./internal/account` exit 0 (all pre-existing tests, incl. `TestNoteThenWindowsRoundTripsTheStatusLine`, `TestWindowsWithNoSourceSaysNoDataNotZero`, `TestYbSt*`) | HOLDS |
| 4 | `git diff origin/main --stat`: `internal/account/codex_read.go`, `windows.go`, `cx_codex_readings_test.go`, this doc. `make lint` exit 0 | HOLDS |
| 5 | Nothing added that guards, blocks or limits agents: the change only reads session files; no hook, no refusal, no cap on an agent, no file-count or age limit on the scan | HOLDS |
| Torn line | `TestCxTornLineIsSkipped`: a torn last line (no newline), a torn line between good ones, a file that is only a torn line | HOLDS |
| Older file next to a newer one | `TestCxNewestReadingByTimestampAcrossFiles`: newer file wins; newer premium-only file falls back to the older file; a resumed older-named file holding a later line wins; the latest line time beats the latest file time | HOLDS |
| Symlinked sessions folder | `TestCxSessionsFolderThatIsASymlinkIsRead`. Red before the fix (`filepath.WalkDir` does not descend a root that is a link): `cx_codex_readings_test.go:333: got {ID:codex Kind:codex Windows:[] AsOf: Source:none Note:Codex use is not read yet}` / `--- FAIL`; green after `filepath.EvalSymlinks` on the root in `codexRollouts` | HOLDS |
| Long lines | `TestCxLongLinesAreRead`: a 200000 byte line before the reading and a reading line of 200000+ bytes | HOLDS |
| Passed reset | `TestCxPassedResetGivesZeroUsed`: Used 0 at the reset second, kept one second before, ResetsAt kept | HOLDS |
| Importers | `go list` of the module: `internal/cli` imports `internal/account`; `cmd/grid` and `lib` import `internal/cli`. `go test -count=1 ./internal/account ./internal/cli ./cmd/grid ./lib` exit 0 (`cmd/grid` has no test files) | HOLDS |
| `make test` | `make test` exit 0: it cannot be scoped to one package, so it ran the whole module (`internal/account` ok, "temp leak check: no new matching entries"). `go test -count=1 ./internal/account` exit 0 | HOLDS |
| Lint | `make lint` (go vet ./..., gofmt -l .) exit 0 | HOLDS |
| Cross builds | `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile) | HOLDS |
| Walk tests | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok | HOLDS |
| Merge main | `git fetch origin && git merge origin/main`: already up to date, run before the last proof run | HOLDS |
| App-only walks (webui-test, realplugin guest walks) | not run, not asked (plugin repo) | not run, not asked |
| Cross-repo set | not run, not asked (orchestrator at landing) | not run, not asked |

### Red on base

The test file was added alone on base (no `codex_read.go`, no `windows.go` change) and `go test -count=1 -run TestCx ./internal/account` gave exit 1. Excerpt of the output (the first failures with their lines, then the names of later ones; the full run fails every test that expects a reading):

```
--- FAIL: TestCxRealReadingGivesSevenDayAndPremiumLineIsSkipped (0.00s)
    cx_codex_readings_test.go:86: got {ID:codex-cara Kind:codex Windows:[] AsOf: Source:none Note:Codex use is not read yet}
        want {ID:codex-cara Kind:codex Windows:[{Label:7d Used:100 ResetsAt:1794778709}] AsOf:2026-11-09T02:54:26Z Source:codex Note:}
--- FAIL: TestCxFiveHourAndSevenDayBothRead (0.00s)
    cx_codex_readings_test.go:97: got {ID:codex Kind:codex Windows:[] AsOf: Source:none Note:Codex use is not read yet}
        want windows [{Label:5h Used:31.5 ResetsAt:1794710000} {Label:7d Used:62 ResetsAt:1794778709}]
--- FAIL: TestCxPassedResetGivesZeroUsed (0.00s)
    cx_codex_readings_test.go:167: at the reset second: got []
        want [{Label:5h Used:0 ResetsAt:1794710000} {Label:7d Used:62 ResetsAt:1794778709}]
--- FAIL: TestCxTornLineIsSkipped/a_torn_last_line_without_its_newline (0.00s)
--- FAIL: TestCxNewestReadingByTimestampAcrossFiles/a_resumed_session:_the_older_named_file_holds_the_newer_line (0.00s)
--- FAIL: TestCxLongLinesAreRead (0.00s)
--- FAIL: TestCxEmptyFolderIsNotTheWorkingDirectory (0.00s)
--- FAIL: TestCxWindowsAnswersACodexAccountFromItsFolder (0.00s)
FAIL	github.com/gridconsole/grid-plugins/internal/account
```

(A first file name ending `_windows_test.go` made Go treat it as Windows-only, so a first run found no tests; the file was renamed before the red run above.) After the change the same run is `ok`. The no-reading cases and the unmapped-length subtests pass on base too, as they assert today's answer.

A run of the built binary on the builder's own Codex folder (a real rollout set) gave, for the default Codex account: `{"id":"codex","kind":"codex","windows":[{"label":"7d","used":0,"resetsAt":1794778709}],"asOf":"2026-11-09T02:55:05Z","source":"codex"}`. Used is 0 because that reading's reset time had passed on the day of the run, by the rule below.

## What changed

- `internal/account/codex_read.go` (new): reads `<folder>/sessions/**/rollout-*.jsonl`.
  - Newest means the line `timestamp`, not the file name: a resumed session appends to its first file, so an older-named file can hold a newer line. Files are scanned most recently written first (ties: path, later first), the reading with the latest line time wins, and the scan stops at the first file written before the best line found, since no line of it can be later. A file with no reading (premium lines) is passed over to an older one.
  - A line is read with `bufio.Reader.ReadBytes`, so there is no line size limit. A line without `"rate_limits"` and `"used_percent"` is not parsed.
  - A line counts as a reading when at least one of its primary and secondary windows is non-null, has `used_percent` and a length that maps. Mapping is by `window_minutes`: 1 to 360 is "5h", 1440 or more is "7d"; 361 to 1439, 0 or none is not read. The answer lists "5h" before "7d" whatever the slot. If both slots map to the same label, the primary is kept.
  - Torn line (not stated by the brief): a line that does not parse as JSON, such as the last line of a file being written, is skipped, and the last good line stands. A line with no parsable timestamp is dated by its file's modification time.
  - AsOf is the line's timestamp, UTC, to the second. Source is "codex", a value the contract schema's `source` enum already holds.
  - A `""` folder (a default account whose `~/.codex` does not exist) is skipped; `filepath.Join("", "sessions")` would read the working directory.
  - A `sessions` folder that is itself a link is resolved with `filepath.EvalSymlinks` before the walk.
  - The first folder of `dirs` with a reading wins, as in the Claude path.
  - `resetWindow` is the reset rule of the Claude path: a window whose `resets_at` has passed (now >= resets_at) has Used 0, ResetsAt kept.
- `internal/account/windows.go`: for kind Codex, `windowsFor` tries `codexWindowsFor` and falls through to today's answer (Source "none", the note) when there is no reading. The Claude lines are unchanged.
- `internal/account/cx_codex_readings_test.go` (new): the tests above.

## Open items

- `limit_id` is not filtered: the rule is a non-null window, as the brief says. Only "codex" lines with windows and "premium" lines with null windows were seen. If Codex ever writes a second limit id with windows, the newest line of either would win.
- Cost with no reading: a folder with no reading anywhere (for example an API-key sign-in, which writes no rate limits) is scanned in full on every call, since the brief says "across older files if needed" and no limit was set. The scan reads each rollout once, line by line, newest file first. If this proves slow, a file-count or age bound is a decision for the owner.
- The note stays "Codex use is not read yet" for a Codex account with no reading, as the brief says; the wording no longer fits and the owner may want it changed (the contract fixtures in `contract/fixtures/cmd-account-windows` carry the same text, outside this lane's files).
- The skill `go-conventions` named in the brief is not in this session's skill list, so it was not loaded. `grid:run-job` was loaded; the project has no Grid record, so no job result or checkpoint was posted and the bundle and this report stand.

## NOT VERIFIED

- A second Codex account folder on the builder's machine answers `none` on a real run of `grid account windows --json`, before and after the symlink fix. Listing its `sessions` folder was refused by the shell sandbox, so whether it holds rollout files with windows is not known; no sessions, or no window lines in them, would make `none` correct. The symlink case, the one cause found, is fixed and tested.

## NOT VERIFIED on a live screen

Nothing here is UI. The Settings Accounts bar the design shows was not drawn; the app reader of `account_windows` was not run against a Codex entry with windows.

DONE
