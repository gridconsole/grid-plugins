# C-pg-closed-task-cancels

Brief: a stopped or closed task gets its open Jobs cancelled and nothing more spawned.
Base: main `00413d82`. Code head: `954817a5` (the proof table commit comes after it).

Base check, quoted at `00413d82`:

```
internal/orchestrator/stream.go
198	// A task whose Jobs are all closed is left alone unless this stream still
199	// holds its lease: then one more pass delivers the last wake and releases it.
200	if !pending && !hasOpenJob(snap) && !s.holds(ts) {
201		delete(s.skipped, dir)
202		return nil
203	}
204	if _, err := Ready(snap, s.now()); err != nil {
205		return s.skip(dir, taskID, SkipGraphInvalid, err)
206	}
207	w := feed.Writer{Author: feed.Author{ID: s.Member, Role: s.Role}, Worker: s.Holder(), Lease: "task"}
208	got, err := ts.AcquireTask(w, s.PID, s.PIDStart, s.probe())
209	if err != nil {
210		if errors.Is(err, lease.ErrTaskHeld) {
211			return s.skip(dir, taskID, SkipTaskHeld, err)
212		}
213		return s.skip(dir, taskID, SkipLeaseFailed, err)
214	}
215	w.Gen, w.Fence = got.Tenure.Gen, got.Tenure.Fence
internal/task/write.go
39	// NextState validates a move without touching disk. Stop means closed/stopped,
40	// as in the task menu; it records intent, not a claim that a worker was killed.
41	func NextState(current, action, requested string) (string, error) {
```

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Closed task with one ready Job: no spawn line, one cancel control | HOLDS | Test: internal/orchestrator/pg_closed_task_test.go `TestPgClosedTaskCancelsReadyJob`. Base (stream.go as at `00413d82`, new test file only): exit 1, `--- FAIL: TestPgClosedTaskCancelsReadyJob` / `pg_closed_task_test.go:133: a closed task printed a spawn line: [map[assign:01M76F7YRXVAR1TSRSF86NSF29 attempt:1 fence:3 gen:0 holder:worker@... id:J-1 task:01ARZ3NDEKTSV4RRFFQ69G5FAA task_dir:.../T-01-task task_epoch:0.2]]`. Lane: `go test -count=1 -v ./internal/orchestrator/ -run TestPgClosed` exit 0, `--- PASS: TestPgClosedTaskCancelsReadyJob`. The test reads the control: `{"target":"J-1","status":"cancelled","close_reason":"task stopped"}` (the body also carries `reason`, as every `Store.control` write does, store.go:182-191). |
| 2. Running, ready and person Jobs all cancelled; lease free; no orchestrator Job | HOLDS | `TestPgClosedTaskCancelsRunningAndPersonJobs`: J-1 spawned by a first pass (status assigned, granted), J-2 ready and person J-3 posted by the stream's tenure, `state: closed`, one pass. J-1, J-2, J-3 are Cancelled in `Load`; no spawn line; `ReleaseTask` by the stream's writer returns `lease.ErrTaskFree` and the tenure has no holder; the Feed holds exactly the Job records J-1..J-3 and none of role orchestrator. Base: exit 1, `pg_closed_task_test.go:156: the closed pass printed a spawn line: [map[... id:J-2 ...]]`. Lane: `--- PASS`. |
| 3. Second and third pass write no record | HOLDS | `TestPgClosedTaskLaterPassesWriteNothing`: two more passes print no line and a restarted stream (same pid) too; the Feed record count is unchanged. Base: exit 1, `pg_closed_task_test.go:179: pass 2 printed [map[spawn:map[... id:J-2 ...]]]`. Lane: `--- PASS`. The pass is skipped by stream.go:200 (no open Job, lease released). |
| 4. Reopen: spawn for the new Job only, no re-plan | HOLDS | `TestPgClosedTaskReopenSpawnsNewJobOnly`: closed pass cancels J-1; task.md rewritten to `state: queued`; J-2 posted; one pass prints one spawn line, for J-2; J-1 stays Cancelled; the Feed holds Job records J-1 and J-2 only. Base: exit 1 at the first step, `pg_closed_task_test.go:197: task lease held by worker@... (epoch 0.2)` (the base stream still held the lease, as it never released the closed task's). Lane: `--- PASS`. No re-plan: `guReplanPersonCancelled` (replan_person.go:21) posts only when a Posted or Ready Job waits on the cancelled person Job (`gkWaiting`), and the waiters were cancelled in the same pass. |
| 4b. Other states are the path of today | HOLDS | `TestPgClosedTaskOtherStatesSpawn` (queued, running, waiting, paused): one spawn line, J-1 not cancelled. Passes on base too (pins unchanged behaviour). |
| 5a. `go test -count=1 ./internal/orchestrator/ -run 'TestPgClosed\|TestStream\|Replan'` | HOLDS | `ok ... internal/orchestrator 10.883s`, exit 0, 0 fail. |
| 5b. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `ok ... internal/cli 0.076s`, exit 0. |
| 5c. `go test -count=1 ./internal/job/lint/` | HOLDS | `ok ... internal/job/lint 0.075s`, exit 0. |
| 6a. `make lint` | HOLDS | prints `go vet ./...`, exit 0. `gofmt -l internal/orchestrator` prints nothing. |
| 6b. `make cross` | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package `ok`. |
| 6c. `git diff main --stat` names only the Touches files | HOLDS | Lane code diff: internal/orchestrator/pg_closed_task.go (new), internal/orchestrator/pg_closed_task_test.go (new), internal/orchestrator/stream.go (3 lines). The only other file is this report, docs/lanes/C-pg-closed-task-cancels.md, which the runbook asks for. tick.go is untouched. |
| Rule: tests of changed packages and importers | HOLDS | Importers: `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./...` filtered for internal/orchestrator: cmd/grid, internal/cli, internal/mcp, internal/migrate, internal/orchestrator, lib. `go test -count=1 ./internal/orchestrator/ ./internal/mcp/ ./internal/migrate/ ./cmd/grid/ ./lib/` exit 0 (orchestrator 55.1s, mcp, migrate, lib ok, cmd/grid no test files). `go test -count=1 ./internal/cli` exit 0 (46.1s). |
| Rule: repo-wide walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok (rows 5b, 5c). The app walk tests are not this repo's. |
| Rule: new tests fail on base, pass on lane | HOLDS | Rows 1, 2, 3, 4: failing lines pasted. Row 4b pins unchanged behaviour. |
| Rule: no existing test edited or deleted | HOLDS | `git diff main --stat` shows no test file but the new one. |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS | The pass writes Cancelled controls of the existing shape and does not tick; no agent is refused or held, no limit added. A running child is not touched (open item 1). |
| Rule: lease code and app unchanged; the grant answer is written | HOLDS | Answer in the comment of internal/orchestrator/pg_closed_task.go:27-43 and in open item 1 below. No file of internal/lease or the app is touched. |
| Rule: small commits, one sentence, no attribution lines; public repo | HOLDS | `git log main..HEAD`. No personal paths, hosts or emails in the diff. |
| Rule: no sudo, nothing outside the clone | HOLDS | Only the clone and /srv/gridwork/tmp/hand were written. |

## What changed

- internal/orchestrator/pg_closed_task.go (new): `pgClosedRead` reads task.md `state` with `taskfile.ScalarFrontmatter`; `Store.pgClosedCancel` posts one cancelled control (`Store.control`, close reason "task stopped") per Job that is not Accepted, Cancelled or Rejected, under `Store.locked` (the task lock and the task lease) and reads the state again inside the lock; `Stream.pgClosedTask` runs it, turns an error into a skip line (SkipTaskHeld for a stale lease, SkipTickFailed otherwise), and releases the task lease as the end of `task()` does (stream.go:275-280) when no Job is open.
- internal/orchestrator/stream.go: in `task()`, after the lease and the receipt move, before `st.Tick()`: `if closed, err := s.pgClosedTask(...); closed { return err }`.
- internal/orchestrator/pg_closed_task_test.go (new): the five tests above.

Concurrency, for the reviewer: the cancel pass runs inside `Store.locked`, the same `.orchestrator.lock` plus lease check a tick takes, and a person's unleased post (`PostPerson`, `exclusive`) takes that lock too. The state is read before the lock only to skip the lock for open tasks, and read again inside it. `grid task reopen` writes task.md before it posts the plan Job (internal/cli/task_write.go:188-193), so a plan Job posted after a reopen is never in a snapshot taken under a lock that saw `closed`. A Job a person posts onto a task whose task.md still says `closed` is cancelled by the next pass (the lease is kept for that pass).

## NOT VERIFIED on a live screen

No UI row. Not run live: a real `worker next` supervisor with a running agent child when the task is stopped (open item 1). The proof is on a fixture home with the stream's own lease and Feed rules.

## Open items

1. **A Cancelled control does not end a Job's grant, so a running child runs on.** In the plugin a Job grant is `lease/<job>.json`, written by `TaskStore.GrantJob` (internal/lease/task.go:310) and ended only by `TaskStore.end` (:406), whose two callers are `ReleaseTask` (:295, the task lease) and `grant` (:350, a second grant of the same lease name). Nothing in internal/lease or internal/feed reads a Job's status (`grep -rn -i cancel internal/lease internal/feed --include=*.go --exclude=*_test.go`: feed/questions.go:155 is the only use of the Cancelled status; the other hits are comments on a cancelled lock wait). The app worker stops a child only when `WithLease` says the lease file no longer names it (kernel/worker/supervisor.go Recover :662-679, Tick :723-747), which stays true after the cancel. What does change: internal/orchestrator/write.go:79 refuses any post under a Cancelled Job's lease with `ErrStaleLease`, so the running child cannot post ack, result or exit. Fix belongs to a lease or app brief (end the Job grant on a Cancelled control, or have the worker stop a child whose Job is Cancelled). Not done here, as the brief says.
2. A reopen whose plan Job is posted while the stream still holds the task lease for a closed pass is fine (person posts are unleased). A person who posts a Job on a task that is still `closed` gets it cancelled by the next pass, by the brief's rule.
3. The control record carries `reason` as well as `close_reason` ("task stopped" in both), as `Store.control` writes every cancel.
