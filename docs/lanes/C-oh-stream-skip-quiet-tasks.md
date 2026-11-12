# C-oh-stream-skip-quiet-tasks

Base: `792b8d6e` (lane-Pint142 merge, equals origin/main). Base check: `internal/orchestrator/stream.go:201` was `snap, err := Load(fs)` and `:214` was `if !pending && !hasOpenJob(snap) && !s.holds(ts) {`; Pass read every task in full each pass, no memo. Not already done on main.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1 `TestOhSqQuietTaskNotRead` | HOLDS | `go test -count=1 -race -v -run 'TestOhSq' ./internal/orchestrator/` exit 0: `--- PASS: TestOhSqQuietTaskNotRead (6.25s)`. 21-task home (1 open with J-1, 20 closed with one Job each); first pass cancels each closed Job (`pgClosedStatus`) and frees its lease (`pgClosedLeaseFree`), second pass stores the memo, then J-2 is posted to the open task as the holder, the test holds all 20 `reader.lock`s, and Pass returns with no `OnBlockForTest` signal and one spawn line for J-2 on the open task. oh_quiet_task_test.go:171-190 |
| 1 on base | HOLDS | Test file committed alone on base (commit "Add tests that a task with only closed Jobs…"), same command, exit 1: `oh_quiet_task_test.go:183: the pass waited for the reader lock of task 1 (…/T-02-task/feed/.cache/reader.lock), a quiet task whose Feed folder did not change since its last read` / `--- FAIL: TestOhSqQuietTaskNotRead (9.70s)`. The only later test edit is `l.unlockAll()` on the pass-returned path, which the failing path does not reach. |
| 2 `TestOhSqAppendWakesTask` | HOLDS | same run: `--- PASS: TestOhSqAppendWakesTask (7.87s)`. A Job J-2 posted with `h.post` to quiet task 4 and `pgClosedSetState(..., "queued")` is read on the next pass (other 19 reader.locks held, no signal) and one spawn line names J-2 on that task. Base: `--- FAIL: TestOhSqAppendWakesTask` with the same reader-lock message. |
| 3 `TestOhSqLeaseWakesTask` | HOLDS | same run: `--- PASS: TestOhSqLeaseWakesTask (7.79s)`. `TakeTask` under one quiet task's `feed/lease/` (no Feed record) makes the pass wait for that task's reader.lock and for no other of the 20; the pass after it waits for none. Base: `--- FAIL: TestOhSqLeaseWakesTask` (waits for task 1). |
| 3 `TestOhSqPendingAlwaysRead` | HOLDS | same run: `--- PASS: TestOhSqPendingAlwaysRead (9.09s)`. Five steps: pending (read), not pending (read, the pending pass wrote), not pending (quiet: not read), pending with an unchanged Feed (read), pending (read); ReceiptMove ran 3 times. Base: `--- FAIL: TestOhSqPendingAlwaysRead`. |
| 4 `go test -count=1 -race ./internal/orchestrator/ ./internal/feed/` | HOLDS with a longer timeout | `go test -count=1 -race -timeout 40m ./internal/orchestrator/` exit 0: `ok internal/orchestrator 668.286s`; `go test -count=1 -race ./internal/feed/` `ok` (128.5s, from the first run of the row command). The row command itself, with the default 10m timeout, did not finish on this VM: the first run printed `panic: test timed out after 10m0s` inside the existing `TestStreamFiveReadyJobsFiveSpawnsInOnePass` while feed's package ran beside it, and the whole package takes 668s under `-race` here. The four new tests are about 31s of that; I have no base timing, and my other runs shared the CPU. The orchestrator's landing run should give the orchestrator package `-timeout 20m` or more. Also run: `go test -count=1 -race -v -run 'TestStream\|TestPgClosed\|TestWl\|TestOhSq' ./internal/orchestrator/` exit 0, 33 `--- PASS`, no FAIL (the stream, pg_closed_task, wl_first_job tests unchanged, plus the four new ones); the same pattern without `-race` exit 0 (`ok 7.115s`, 33 PASS). |
| 5 `make lint` | HOLDS | `make lint && make cross` exit 0 (`go vet ./...`). |
| 5 `make cross` | HOLDS | same command, exit 0: `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`, every package `ok`. |
| 5 walk `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok internal/cli 0.038s`). |
| 5 walk `internal/job/lint` | HOLDS | `go test -count=1 ./internal/job/lint/` exit 0 (`ok internal/job/lint 0.086s`). |
| Importers of the package | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./... \| grep internal/orchestrator` gives `internal/cli`, `internal/mcp`, `internal/migrate`; `go test -count=1 ./internal/cli/ ./internal/mcp/ ./internal/migrate/` exit 0 (cli 68.3s, mcp 0.208s, migrate 3.06s). |
| New tests fail on base, pass on lane | HOLDS | see rows 1-3 |
| No existing test edited or deleted | HOLDS | `git diff --stat origin/main..HEAD`: stream.go (+11), oh_quiet_task.go (new), oh_quiet_task_test.go (new); no deletions. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | the memo only skips a read of a task with no open Job and no held lease; it adds no limit, timer or count, and a changed key or pending receipt reads the task as before. |
| Repo-wide walk tests of the app (`make webui-test`, TestFvArgvCollects, TestEveryWiredMount…, slices Imports) | NOT APPLICABLE | the app repo is not this lane's repo; the plugin walk rows above are the ones the runbook names for the plugin. |

## What changed

- `internal/orchestrator/oh_quiet_task.go` (new): `ohSqFeedKey` digests (sha256) the name, size and mod time of every `feed/*.jsonl` and every entry of `feed/lease/` (not `feed/.cache/`); `Stream.ohSqCheck` answers the key and whether the task is quiet; `Stream.ohSqStore` keeps the key.
- `internal/orchestrator/stream.go`: one field `ohSqMemo`; in `task()` one `ohSqCheck` call after `feed.TaskID` and the `pending` read and before `Load` (a quiet task is dropped with `delete(s.skipped, dir)`, return nil); one `ohSqStore` call at the return that drops a task with no open Job and no held lease.
- `internal/orchestrator/oh_quiet_task_test.go` (new): the four tests.

Concurrency: the key is taken before `Load`, so a record or lease written after it shows in the next key and the next pass reads it; a key taken after `Load` could have hidden a record `Load` missed. A memo is kept only when the pass saw at least one Job (a task with no Job is the first-Job path, which reads task.md, not in the key), no open Job, no held lease and no pending receipt. A key that cannot be taken (list or stat error) clears the memo and reads the task as before. Writers append, so size changes on every Feed record; lease files are truncated and rewritten in place under flock, so their size and mod time change on a grant or release.

## Open items and notes

- The memo map is never pruned for a task folder that leaves the list (one 32-byte digest per folder; `s.skipped` has the same shape). Not a limit on agents.
- A lease file rewritten to the same size within one mod-time tick would not change the key; on Linux the tick is the kernel's file time, and a grant always changes the fence or holder text too. The memo is only kept while no lease is held by this stream.
- The new tests take 6-9 s each under `-race` (21 tasks, fsynced Feed writes).

## NOT VERIFIED on a live screen

Nothing here has a screen. Not run: a live worker with a day-old home; the timing of Pass with the memo against a real home.
