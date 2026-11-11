# C-pcov4-orchestrator-events-tick

Tests only: one new file, `internal/orchestrator/orchestrator_events_tick_test.go`. No production code, no existing test edited.

## Proof table

| Row | Result |
|---|---|
| 1 coverage before / after | HOLDS. Before (untouched clone): `go test -count=1 -coverprofile=… ./internal/orchestrator/` exit 0, `coverage: 85.3% of statements`; `go tool cover -func … \| tail -1` -> `total: (statements) 85.3%`. After: same command, exit 0, `coverage: 87.9% of statements`. After is above before. Per function after: progress 100.0%, recoverMemory 100.0%, wake 97.4%, park 90.0%, checkAfterTargets 90.9%, exclusive 76.5%, Tick 73.8%. |
| 2 progress shapes | HOLDS. `TestEventsTickProgressShapes` (orchestrator_events_tick_test.go:42). string, []any and map give one value apart from the no-worktree value; `../w1` equals the no-worktree value; `w1@x` differs from none and from `w1`; a new commit changes the `w1` and `w1@x` values and not the `../w1` one. progressMemo nil. Passes, exit 0. |
| 3 progress memo | HOLDS. `TestEventsTickProgressMemo` (:76). Memo holds the id after call one; a commit does not change call two. |
| 4 park | HOLDS. `TestEventsTickPark` (:90): `no owner` asserts the exact message "crash loop for J-1: task owner is required (--owner or GRID_OWNER)" and recordCount unchanged; `question reused` asserts one question before and after, status Parked, reason `needs_person`. |
| 5 wake :186 | HOLDS with a change from the brief (see Outcome). `TestEventsTickWakeRejectedHistory` (:148): a Rejected Job with history record status "rejected" -> `Wake{Job:"J-1", Event:"h1", N:1}`; status "done" -> nil. |
| 6 recoverMemory | HOLDS. `TestEventsTickRecoverMemory` (:173): checkpoint, usage_limit, cancelled -> Failures empty; done -> `["e0"]`; stale control with Re "r1" -> `["e0","r1"]`, Failure "r1". |
| 7 Tick and exclusive | HOLDS with a change from the brief (see Outcome). `TestEventsTickRefusals` (:208): Writer.Lease "job" -> `errors.Is(err, feed.ErrStaleLease)`, Rows empty; TaskDir a regular file -> "task is not a folder", fn not called. |
| 8 checkAfterTargets | HOLDS. `TestEventsTickCheckAfterMalformed` (:237): body `after: 7` -> nil. No `write_behaviour_test.go` on main to reuse. |
| 9 go test | HOLDS. `go test -count=1 ./internal/orchestrator/` ok (exit 0). `GOOS=windows go vet ./...` exit 0. `GOOS=darwin go vet ./internal/orchestrator/` exit 0. `GOOS=windows go test -c -o /dev/null ./internal/orchestrator/` exit 0. `make lint` exit 0 (`go vet ./...`). `gofmt -l internal/orchestrator` empty. |
| Importers | HOLDS. `go list` shows `internal/cli` as the only package importing internal/orchestrator; `go test -count=1 ./internal/cli/` ok (exit 0). |
| New tests fail on base | NOT APPLICABLE. These tests pin behaviour that exists on main; on base the file does not exist, so nothing fails. Nothing was asserted that the code does not do. |
| No existing test edited / deleted | HOLDS. `git diff --stat main` shows one added file and this report. |
| No guard, gate, cap, hold or hook added | HOLDS. No non-test file changed. |
| No personal paths, hosts, emails | HOLDS. Test names, ids and the git identity from `testgit` (example.com) only. |
| Windows / macOS builds | Compiled only: needs Windows / macOS run for the tests themselves (`GOOS=windows go test -c`, then run the binary on Windows). The new file has no build tag and no unix-only call; it needs `git` (the repo-making tests skip without it). |

## Outcome (where the code differs from the brief)

- Row 5: the brief says the wake event names the record. In `wake`, a record in `State.Wakes` becomes a candidate whatever its status (events.go:210-217), so with "done" the Wake would still come back. The test therefore gives the Job's history only and no wake event, which isolates the :202-206 path. Asserted: status "rejected" -> Wake, status "done" -> nil.
- Row 7: `Store.locked` calls `Feed.WithLease(s.Writer, nil, …)` before `exclusive`, and `leaseLock` refuses any lease other than "task" when the attempt is nil (feed/store.go:125). So `Writer.Lease == "job"` returns `ErrStaleLease` from `WithLease`, and tick.go:24-26 is not reached through `Tick` (the line stays uncovered; it looks unreachable while `locked` passes a nil attempt). The test asserts the observable result: `ErrStaleLease` and no Rows.

## What changed

- `internal/orchestrator/orchestrator_events_tick_test.go` (new, 7 tests, helpers `eventsTickProgress`, `eventsTickRepo`, `eventsTickPtr`).
- `docs/lanes/C-pcov4-orchestrator-events-tick.md` (this report).

## NOT VERIFIED on a live screen

None; no UI. Not run: the whole-module test and the cross-repo set (the orchestrator runs them at landing). The rebase onto lane-M-plugin-held-f-e was not needed: it was not on main; if it lands first, check that `jobStatus` and `recordCount` still exist and rerun row 9.
