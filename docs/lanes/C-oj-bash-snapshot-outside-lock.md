# C-oj-bash-snapshot-outside-lock

The Bash history hook walks and hashes the task folder before it takes the
task's session lock. Repo: plugin. Branch lane-C-oj-bash-snapshot-outside-lock.

Commits on base 792b8d6e (7a4bc0c2 is an ancestor of it):

- 581b384f Add the nil ojSnapshotForTest seam to snapshot and the tests that hold the Bash walk outside the session lock.
- cac7d1f9 Walk and hash the task folder before the session lock in the Bash history hook.

## Base check

Base sha 792b8d6e. `git merge-base --is-ancestor 7a4bc0c2 HEAD` exits 0. On base
internal/hook/history.go :111-128 reads:

```
111	ctx, cancel := context.WithTimeout(context.Background(), historyLockWait)
112	err = store.UpdateActivityContext(ctx, in.SessionID, func(a *session.Activity) error {
113		lines = nil
114		if in.HookEventName == "PreToolUse" {
115			snap, err := snapshot(dir)
...
125		after, err := snapshot(dir)
```

`snapshot(dir)` is inside the func given to `UpdateActivityContext`, at :115 and :125.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 `go test -count=1 -race -v -run 'TestOjSo' ./internal/hook/` | HOLDS | exit 0 on the lane (cac7d1f9): `--- PASS: TestOjSoUsageNotBehindWalk (0.41s)` with `--- PASS: TestOjSoUsageNotBehindWalk/PreToolUse` and `/PostToolUse`. |
| 1 failing on base | HOLDS | On 581b384f (base plus only the nil seam and its call at the top of `snapshot`, walk not moved) the same command exits 1: `oj_snapshot_outside_lock_test.go:157: RecordUsage of another session waits for sessions/.lock while the PreToolUse walk is running: the walk runs inside the session lock`, the same line for PostToolUse, then `--- FAIL: TestOjSoUsageNotBehindWalk/PreToolUse` and `--- FAIL: TestOjSoUsageNotBehindWalk/PostToolUse`. Check out 581b384f to see it. |
| 2 `--- PASS: TestOjSoTwoSessionsOwnChanges` | HOLDS | Same command, exit 0, `--- PASS: TestOjSoTwoSessionsOwnChanges (0.01s)`. Two sessions (main and a subagent `oj-helper`), tool use ids `oj-use-1` and `oj-use-2`, Pre run together, one file written, Post run together: history.md holds one `file written · changed.md` line per session; after the Pres each session holds a snapshot under its own id only, after the Posts neither holds any (`ojSoActivity`). The sessions differ in writer on purpose: `task.AppendHistory` drops a line that repeats the one before it (task/history.go:181-185), and the first run of the test with the same writer for both found one line, not two. |
| 3 `go test -count=1 -race -run 'TestBashHistory\|TestSnapshot\|TestHistoryWriteOfOwnHistoryFileIsSkipped\|TestKr2' -v ./internal/hook/` | HOLDS | exit 0. A `--- PASS:` line for all 11 tests of bash_snapshot_test.go (EarlyReturnsStoreNothing with 4 subtests, PreAloneAndPostWithoutPre, EditedWrittenRemoved with 2 subtests, MoreThanTen, ExactlyTen, SnapshotSkipsOwnFiles, SnapshotKeys, SnapshotFileCountLimitPin, LimitAtPreToolUse, LimitAtPostToolUse, TestHistoryWriteOfOwnHistoryFileIsSkipped) and 5 of kr2_history_wait_test.go (HistorySkipsWhenSessionLockBusy, HistoryRecordsWhenSessionLockFree, PostAfterSkippedPreWritesNothing, SlowHookLogsOneLine, FastHookLogsNoSlowLine). No existing test edited: `git diff origin/main --stat` shows history.go and the new test file only. |
| 4 `go test -count=1 -race ./internal/hook/ ./internal/session/` | HOLDS | exit 0: `ok .../internal/hook 34.995s`, `ok .../internal/session 12.573s`. internal/session is not changed. |
| 5 `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| 5 `make cross` | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package `ok`. |
| 5 `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0, `ok .../internal/cli 0.092s`. |
| 5 `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0, `ok .../internal/job/lint 0.208s`. |

## Runbook rules

| Rule | Result | Proof |
|---|---|---|
| Packages that import the changed one | HOLDS | `go list` (imports and test imports) of `internal/hook`: `internal/cli`, `internal/packaging`, `internal/golden`. `go test -count=1 ./internal/cli/ ./internal/packaging/ ./internal/golden/` exits 0: cli 132.189s, packaging 11.440s, golden 0.899s. |
| Windows and macOS builds | HOLDS | `make cross` above. |
| New tests fail on base and pass on the lane | HOLDS | Row 1 and row 2 lines above. `TestOjSoUsageNotBehindWalk` is the one that fails on the seam-only base. `TestOjSoTwoSessionsOwnChanges`, `TestOjSoPostWithoutBeforeDoesNotWalk` and `TestOjSoPostSkipsWhenSessionLockBusy` pass on both: they hold what must stay the same when the walk moves, they do not prove the move. |
| No existing test edited or deleted | HOLDS | `git diff origin/main --stat`: `internal/hook/history.go`, `internal/hook/oj_snapshot_outside_lock_test.go` (new). |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | Work moved, none added. `historyLockWait`, the 2000-file and 32 MB walk bounds and every log line are unchanged (history.go:25 and :237 of the lane). No second bounded wait. |
| Repo-wide walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0. The app-side walk tests of the runbook do not apply to the plugin repo. |
| Seam only in tests | HOLDS | `ojSnapshotForTest` is declared in history.go, called once at the top of `snapshot`, and assigned only in oj_snapshot_outside_lock_test.go. The test sets it before it starts a goroutine and puts it back after `wg.Wait()`. `go test -race -count=20` of the three fast tests exits 0. |
| Test rules (no TestMain, no sleep, no wall-clock wait, no elapsed assertion in the block proof) | HOLDS | The block proof waits on `entered`, `queued` (fslock.OnBlockForTest on `sessions/.lock`) and RecordUsage's result, bounded by `kr2Bound`. `TestOjSoPostSkipsWhenSessionLockBusy` waits the 2 s `historyLockWait` as kr2 test (a) does and asserts no elapsed time. |
| Report ends with DONE | HOLDS | Last line. |
| UI rows | n/a | No UI. |
| `*RealPlugin*` tests | n/a | None named. |

## What changed

internal/hook/history.go, `bashHistory`:

- PreToolUse: `snapshot(dir)` runs before the lock. An error logs `history snapshot` and returns, so nothing is stored and the lock is never taken. Inside the lock stays `a.Snapshots[in.ToolUseID] = snap`.
- PostToolUse and PostToolUseFailure: the walk runs before the lock, and only when the tool use has a stored `before`. Inside the lock stay the read of `before`, the compare, the list of lines and `delete(a.Snapshots, ...)`. A walk error logs `history snapshot` and writes nothing.
- `ojSnapshotForTest func()` (nil in production) is called at the top of `snapshot`.

## Choice named by the brief: how PostToolUse finds out it has a `before`

I chose a check before the walk, made with `store.Activity(in.SessionID)`, which reads the activity file without the lock (the file is replaced whole by `atomicJSON`, and that read is how every reader already gets it). It is a hint only: the walk happens when the hint says a `before` exists, and the read under the lock decides, so a snapshot that is gone by then (`!ok`) or a hint that found none (`after == nil`) records nothing. Not taken, and why:

- A short lock first and then the walk: a second bounded wait per Post, so a worst case of 4 s where today it is 2 s, and the lock is queued behind twice under load.
- Walk first, then one lock: one wait, but a Post whose Pre was skipped under lock pressure (the load this lane is for) walks the whole folder for nothing, and a Post with no `before` in an over-limit folder would log the limit error where today it logs nothing. It would also make the comment at bash_snapshot_test.go:95 ("returns before it walks the folder") untrue.

With the hint, the lock is still taken by every Post, as on base: the busy-lock skip line, the single `historyLockWait` and the write of a first activity record (`Last` of a session with none) are as before. `TestOjSoPostWithoutBeforeDoesNotWalk` pins that a Post without a stored snapshot makes no walk.

## Differences from base that a reviewer should know

- A PreToolUse whose walk fails now logs `history snapshot: <error>` even when the lock is busy; base would have logged `history skipped` in that case and never walked. A PreToolUse for an invalid session id now walks before the lock call rejects the id; the log line is the same.
- The Pre snapshot is taken up to `historyLockWait` (2 s) before it is stored, so a file another session of the same task writes in that window counts as a change of this tool use. Other sessions' writes during a Bash call were already counted as its changes.
- A Post whose walk fails now leaves the stored `before` where it is, as on base (the closure returned the error before `delete`).

## NOT VERIFIED on a live screen

Nothing here has a screen. Not run: 30 real agents in one task; the effect on the wait of the other users of the lock (`RecordUsage` is proven in the test, `recordSession`, the job message and the worker-exit record take the same lock through `UpdateActivity` and are not run behind a walk in a test of their own).

## Landing

C-oi-posttooluse-under-load edits internal/hook/events.go and pe_job_message.go. This lane touches neither; history.go and the new test file do not overlap.
