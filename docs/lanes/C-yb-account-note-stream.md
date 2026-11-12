# C-yb-account-note-stream: `grid account note --stream <attempt log>`

Base: main 2b47f621. `git grep -n rate_limit_event main -- internal/` has no hit; the `note` help listed no `--stream` (internal/cli/account.go:20-21 on base). Code commit: f5ace0e3.
Commit identity: the repo's configured user "Grid lane". No attribution lines in any commit (owner's rule).

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. account tests: evidence line gives seven_day 87, resets 1794708000, source "stream", `windowsFor` answers "stream" with a 7d window Used 87; both types kept; later line replaces earlier; no event writes no file; bad JSON line skipped; newer snapshot kept byte for byte (os.Chtimes); no-`source` snapshot reads "statusline" | HOLDS | `go test -count=1 -v -run TestYbSt ./internal/account/` exit 0: PASS for TestYbStEvidenceLineGivesAStreamSnapshot, TestYbStFiveHourAndSevenDayKeepBoth, TestYbStLaterLineOfOneTypeReplacesAnEarlierOne, TestYbStNoEventWritesNoFile, TestYbStBadJSONLineAmongGoodOnesIsSkipped, TestYbStSnapshotNewerThanTheLogIsKeptByteForByte, TestYbStSnapshotWithNoSourceKeyStillReadsStatusline. Code: internal/account/yb_stream.go, windows.go:30-33 and :168-171. |
| 2. cli tests: `account note --dir <d> --stream <log>` exits 0, prints nothing, snapshot file holds `"source":"stream"`; missing file exits 1, stderr starts `grid account note: ` | HOLDS | `go test -count=1 -v -run TestYbSt ./internal/cli/` exit 0: PASS TestYbStAccountNoteStreamKeepsTheWindowsAndPrintsNothing, TestYbStAccountNoteStreamMissingFileExitsOne. On base (main in a scratch worktree, same two tests copied in) both FAIL: `note --stream: 2 out="" err="flag provided but not defined: -stream\nUsage of grid account note: ..."` and `2 "flag provided but not defined: -stream ..."`. The account tests fail to build on base the same way (`unknown field Source in struct literal of type Snapshot`, with windows.go at main). The scratch worktree is removed; `git status --short` is clean. |
| 3. `make generate` clean on a second run; `"--stream"` in contract/commands.json under note; contract and doctor golden tests | HOLDS | `make generate` ran twice, `git status --short` empty after the second (the first run's contract/commands.json change is in f5ace0e3; the doctor golden did not change). `git grep -n '"--stream"' -- contract/commands.json` -> contract/commands.json:4528 (inside the note flags, 4509-4536 on base). `go test -count=1 -run 'TestContractCommands\|TestDoctorGolden' ./internal/cli/` exit 0. |
| 4. `go test -count=1 ./internal/account/ ./internal/cli/` | HOLDS | exit 0 (`ok internal/account 0.340s`, `ok internal/cli 51.530s`). Importers of internal/account from `go list`: internal/cli only. |
| 5a. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0 |
| 5b. `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0 |
| 5c. `make lint` | HOLDS | exit 0 (`go vet ./...`); `gofmt -l internal/account internal/cli` prints nothing |
| 5d. `make cross` | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package ok |
| 5e. `git diff main --stat` names only the Touches | HOLDS | contract/commands.json, internal/account/{windows.go,yb_stream.go,yb_stream_test.go,testdata/yb_stream_evidence.log}, internal/cli/{account.go,yb_note_stream_test.go}, plus this report |
| Rule: no attribution lines | HOLDS | `git log` messages carry none |
| Rule: no guard, gate, cap or hook on agents | HOLDS | nothing added that blocks an agent. The only refusal is a usage error (exit 2) for `--stream` combined with `--then` or `--show`, which are stdin modes. |
| Rule: no existing test edited or deleted | HOLDS | `git diff main --stat` shows no test file modified; all test files are new |
| Rule: repo-wide walk tests (plugin) | HOLDS | rows 5a and 5b above |
| Rule: `*RealPlugin*` tests | not applicable | no row names one |
| Rule: public repo, no personal paths, hosts or emails | HOLDS | the evidence log is the verbatim line from the brief; it holds a session id and uuid only |

## What changed

- `internal/account/windows.go`: `Snapshot` gains `Source string json:"source,omitempty"`. `windowsFor` answers `s.Source`, and "statusline" when it is empty, so every snapshot on disk today reads as before.
- `internal/account/yb_stream.go`: `NoteStream(env, log, dir)` reads the log line by line (no line size limit), keeps the last `rate_limit_event` per `rateLimitType` (`five_hour`, `seven_day`; others ignored), Used = utilization x 100 (rounded to 2 decimals so 0.87 gives 87), ResetsAt = resetsAt, At = the log's mtime in UTC RFC3339, Source "stream". A line that is not JSON is skipped. An event with no `utilization` is skipped (real streams carry `status: allowed` events with no number). No usable event: nothing is written. An existing snapshot whose At is later than the log's mtime is left untouched.
- `internal/cli/account.go`: `note --stream <path>` calls `NoteStream` and reads no stdin; help text names it. `hsIpTable` is unchanged.
- `contract/commands.json` regenerated by `make generate`.

Behaviour to know: a stream snapshot replaces the whole file, so a newer stream snapshot with only `seven_day` drops an older status-line `five_hour`. The brief did not ask for a merge, so none was made.

## NOT VERIFIED on a live screen

- Settings in the app showing the stream numbers: needs the app and WAIT-C-yb2-worker-notes-windows, which adds the caller. No claude run, no live worker.
