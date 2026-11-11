# C-if-lock-wait-event

Base: ee8a96c1 (`git diff main` shows only this lane's files). Two lock-wait tests now wait on an event from the lock call instead of 150 ms and 100 ms of quiet.

## Proof table

| Row | Result |
|---|---|
| 1. no `time.After(100\|150 ms)` in the two test files | HOLDS. `git grep -nE 'time.After\((100\|150) \* time.Millisecond\)' -- internal/signal/move_test.go internal/orchestrator/orchestrator_test.go` prints nothing (exit 1 = no match) |
| 2. TestIfLwHook | HOLDS. `go test ./internal/fslock -run TestIfLw -count=1` exit 0. It covers LockExclusive and LockShared against an exclusive holder (hook called once with the path, before the call returns; the call returns only after the holder unlocked, shown by a channel closed before Unlock), an uncontended call (hook not called), the hook restored to nil (counter stays 0, call still returns only after Unlock) and restore of the previous hook. On base it fails to compile: see "New tests fail on base" below |
| 3. scratch: no localdata.Lock in Move | HOLDS. With move.go:45-49 removed (and `_ = localdata.Lock` to keep the import), `go test ./internal/signal -run '^TestMoveWaitsForAddLock$' -count=1` exit 1: `move_test.go:113: move bypassed signals lock: <nil>` / `FAIL`. Reverted with `git checkout -- internal/signal/move.go` |
| 4. scratch: Write path without the shared task.json lease | HOLDS. Write takes task.json shared twice: `internal/lease/task.go:88` (TaskStore.Read("task"), called from write.go:41 because the worker lease is "J-1") and `internal/feed/store.go:113` (leaseLock, reached through write.go:50 -> store.go:114 WithLease with the task guard; the order ready.go:3-6 describes). With both LockShared calls removed, `go test ./internal/orchestrator -run '^TestWorkerWriteWaitsForTaskLeaseBeforePassLock$' -count=1` exit 1: `orchestrator_test.go:983: worker wrote while task lease was exclusively held: <nil>`. Reverted with `git checkout -- internal/lease/task.go internal/feed/store.go`. `git diff main -- internal/signal/move.go internal/orchestrator/ ':!*_test.go'` prints nothing |
| 5a. `go test ./internal/fslock ./internal/signal ./internal/orchestrator ./internal/localdata -count=1` | HOLDS, exit 0: fslock ok 0.153s, signal ok 0.285s, orchestrator ok 120.944s, localdata ok 0.097s |
| 5b. `go test ./internal/signal ./internal/orchestrator -run 'TestMoveWaitsForAddLock\|TestWorkerWriteWaitsForTaskLeaseBeforePassLock' -count=20` | HOLDS, exit 0: signal ok 0.350s, orchestrator ok 2.360s |
| 6a. `make lint` | HOLDS, exit 0 (`go vet ./...`, `gofmt -l .` empty) |
| 6b. `GOOS=windows go vet ./internal/fslock/ ./internal/signal/ ./internal/orchestrator/` | HOLDS, exit 0 |
| 6c. `GOOS=darwin go vet ./internal/fslock/` | HOLDS, exit 0. Also `make cross` (vet and test compile for darwin/arm64 and windows/amd64 of every package) exit 0 |
| 6d. TestIfLwSeamOnlyInTests, scratch call from localdata.go | HOLDS. `_ = fslock.OnBlockForTest` added in localdata.Lock: `go test ./internal/fslock -run '^TestIfLwSeamOnlyInTests$' -count=1` exit 1: `if_wait_hook_test.go:256: test seam OnBlockForTest named outside tests: internal/localdata/localdata.go`. Reverted with `git checkout -- internal/localdata/localdata.go`. (While building, the test also caught my own doc comments in fslock_unix.go and fslock_windows.go; they now say "the wait hook of fslock.go".) |
| 6e. `go test -race ./internal/fslock ./internal/signal ./internal/orchestrator -run 'TestIfLw\|TestMoveWaitsForAddLock\|TestWorkerWriteWaitsForTaskLeaseBeforePassLock' -count=1` | HOLDS, exit 0: fslock ok 1.715s, signal ok 1.273s, orchestrator ok 1.781s |
| Tests of every importer | HOLDS, exit 0. Importers of internal/fslock by `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...`: account, artifact, brain, cli, doctor, env, feed, hook, intent, land, lease, localdata, migrate, orchestrator, plan, ports, proof, question, session, settings, task, usage, worker, workflow (all under internal/). `go test -count=1` on those plus internal/signal (which imports localdata): all ok (cli 537.444s, migrate 94.897s, hook 62.900s, doctor 54.525s, task 53.313s, brain 33.400s, feed 29.223s, others under 25 s); orchestrator ok in 5a |
| New tests fail on base, pass on lane | HOLDS. TestIfLwHook and TestIfLwSeamOnlyInTests on a detached checkout of main with the new test file copied in (checkout removed after): `go test ./internal/fslock -run TestIfLw -count=1` exit 1: `if_wait_hook_test.go:100:16: undefined: OnBlockForTest` / `if_wait_hook_test.go:167:8: undefined: onBlock` / `FAIL github.com/gridconsole/grid-plugins/internal/fslock [build failed]`. The signal and orchestrator tests use the same seam, so they do not compile on base either; the scratch rows 3 and 4 show them failing for the right reason. Pass on the lane: rows 2, 5, 6e |
| No existing test edited to pass, none deleted | HOLDS. The two existing tests keep every assertion and message; only the quiet window became a wait on the hook or `done`. `done` first fails with the old message |
| No sleep, elapsed-time assertion or timeout as proof | HOLDS. The only deadline is `ifLwDeadline` (30 s), which bounds a wait and fails by name ("timed out waiting for Move to reach the signals lock", "...the writer to queue on task.json", "...the hook of <call>") |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS. None added; the hook is nil in production and only a test sets it |
| RealPlugin tests | Not applicable: no row names one |
| Commits | One commit, one sentence, no attribution lines. Public repo: no personal paths, hosts or emails in the diff |

## What changed

- `internal/fslock/fslock.go`: `OnBlockForTest(f func(path string)) (restore func())`, kept in an `atomic.Pointer` that is nil in production, and `lockWaiting(f, try, block)`. With no hook it is `block()`, today's single blocking call, after one atomic Load. With a hook it first tries without waiting; on ErrLocked it calls the hook with `f.Name()`, then blocks as before. A nil argument clears the hook; `restore` puts back the hook that was set before.
- `fslock_unix.go`: LockShared and LockExclusive go through `lockWait` -> `lockWaiting`; `tryFlock` (LOCK_NB, EWOULDBLOCK -> ErrLocked) is shared with TryLockExclusive.
- `fslock_windows.go`: the same with `lockWait` and `tryLockFile` (LOCKFILE_FAIL_IMMEDIATELY, ERROR_LOCK_VIOLATION -> ErrLocked), shared with TryLockExclusive. `fslock_other.go` is unchanged (its calls still return ErrUnsupported and never call the hook).
- `internal/fslock/if_wait_hook_test.go`: TestIfLwHook and TestIfLwSeamOnlyInTests (build tag: unix and windows).
- `internal/signal/move_test.go` (TestMoveWaitsForAddLock only) and `internal/orchestrator/orchestrator_test.go` (the :940 test only): set the hook for the lock path they hold (`localdata.Safe(home, ".signals.lock")`, `.../lease/task.json`), with a non-blocking send; wait on `done` (old message), the hook (proof) or the named deadline. Each file gains a package-level `ifLwDeadline`. The orchestrator test's later `time.After(time.Second)` stays.

## Notes

- Linux flock conversion: a failed LOCK_NB request on a descriptor that already holds the other lock type drops the old lock, as the blocking request does before it waits, so the try-then-block path matches the blocking call. Only a test with the hook set takes that path.
- The nil-hook half of TestIfLwHook proves "returns only after Unlock" by a channel; it cannot prove the goroutine reached the flock (there is no event with the hook nil), which is what the hook-set half proves.
- The hook is called on the goroutine that is about to wait, so a hook function must not block; the doc comment says so.

## NOT VERIFIED on a live screen

Nothing here has a screen. Not run: the Windows tests (only vet and test compile through `make cross`); TestIfLwHook on Windows runs on a Windows machine only.

DONE
