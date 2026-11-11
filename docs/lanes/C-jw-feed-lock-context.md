# C-jw-feed-lock-context: lane report

Result: NOT DONE. The production change is in and the existing tests of the five
touched packages pass, but the feed row (2) and orchestrator row (3) have no
passing test, and the race run, `make cross`, the importer list, the post-merge
`git grep` and the fails-on-base proof were not run. I used the call budget on
orientation reads (about 28 calls before the first edit) and did not stop at
call 10 to name the split. The split is at the bottom.

## Proof table

| Row | State | Command / place |
|---|---|---|
| 1. fslock: cancel returns `context.Canceled`; abandoned waiter unlocks, test waits on a hook, third fd then locks | HOLDS | `go test -count=1 -v -run TestJwLc ./internal/fslock/` exit 0: `--- PASS: TestJwLcCancelledWaitReturnsAndWaiterReleases` (both `LockExclusiveContext` and `LockSharedContext`), `--- PASS: TestJwLcLiveContextAndNilContextWaitAsBefore`, `--- PASS: TestJwLcDoneContextTakesNoLock`, `--- PASS: TestJwLcSeamOnlyInTests`. Test: `internal/fslock/jw_context_test.go`. It waits on `OnBlockForTest` and on the new `OnAbandonedWaitForTest` (`internal/fslock/jw_context.go`); the only timer is the 30 s failure path. It also checks the waiter closed its descriptor (`Close` returns `os.ErrClosed`). |
| 2. feed: cancelled Store Context while wake.lock is held returns `context.Canceled`, Feed bytes unchanged | NOT DONE | Production is wired: `Store.Context` field, `wakeLock`, `leaseLock`, the tenure lock in `Post` and `gateLock` in `internal/feed/store.go` and `internal/feed/gate.go` call `fslock.Lock*Context(s.Context, f)`. The test is drafted but fails on its input, not on the lock: `jw_lock_context_test.go:93: Post returned ($.wake: anyOf matched 0 branches: $.wake: expected null) while the wake lock was held`. A `post` record may not carry `wake`; the test must post the type the record schema allows a wake on. The draft is untracked at `internal/feed/jw_lock_context_test.go` and copied to `/srv/gridwork/tmp/hand/C-jw-feed-lock-context-feed-test.go.txt`; it is not in the bundle. |
| 3. orchestrator exclusive: same with `.orchestrator.lock` held | NOT DONE | Production is wired: `exclusive` in `internal/orchestrator/store.go` calls `fslock.LockExclusiveContext(s.Feed.Context, f)` and defers `f.Close()` only after the lock is held. No test written. The orchestrator package has no new-test file in Touches; I would add `internal/orchestrator/jw_lock_context_test.go` (feed's test cannot import orchestrator). |
| 4. Store with no Context behaves as before | HOLDS | `go test -count=1 ./internal/fslock/ ./internal/feed/ ./internal/orchestrator/ ./lib/ ./internal/cli/` exit 0: `ok internal/fslock 0.446s`, `ok internal/feed 6.203s`, `ok internal/orchestrator 187.964s`, `ok lib 49.675s`, `ok internal/cli 127.166s`. No existing test was edited. This ran on the commit that holds the production change and the fslock tests, before the feed draft existed. |
| 5a. five-package test with `--- PASS: TestJwLc...` | PARTIAL | Five packages pass (row 4). `TestJwLc` PASS lines exist for fslock only; feed and orchestrator have none. |
| 5b. `go test -race -count=1 -run TestJwLc ./internal/fslock/ ./internal/feed/` | NOT DONE | Chained after the failing feed draft, so it never ran. |
| 5c. `make lint` exit 0 | HOLDS | `make lint && go test ...` ran `go vet ./...` with no output and went on to the next command, so it exited 0 (the draft feed test was in the tree for that run). |
| 5d. `make cross` exit 0, name how the lock is taken on Windows | NOT DONE (not run) | The new file has no build tag and calls `LockExclusive`/`LockShared`/`Unlock`, so on Windows the waiter's goroutine takes the lock with the existing `LockFileEx` call (`lockHandle` in `internal/fslock/fslock_windows.go`); the caller's return never touches the handle. Not built for Windows or macOS in this run. |
| 5e. `git grep -n -E 'fslock\.Lock(Shared\|Exclusive)\(' -- internal/feed internal/orchestrator/store.go` after the merge | NOT DONE | Not re-run after the merge, and the merge was not done (see below). From the grep before my edit, the hits that stay plain are `internal/feed/cache.go:52` (`reader.lock`, held for one read-and-rewrite of `reader.json`; `Post` reads with `ReadOnly`, which takes no guard), `internal/feed/fold.go:71` (`fold.lock`, `SaveStateLiveness` only, not on the post path), `internal/feed/identity.go:27` (`MachineID`, once per home) and `internal/feed/identity.go:126` (`EnsureTaskID`, `.task.lock`, a package function with no Store and a short section). `Post` reaches `EnsureTaskID` only for a task that has no id yet. All four are a gap I did not close: a stuck holder would still block them. Converted: wake, lease, tenure, gate in `Post`; `exclusive` in orchestrator. |
| Merge `origin/main` before the last commit | NOT DONE | `git fetch origin` ran at the start; the lane was on `origin/main` 750ac8de then. `C-jw-record-id-whole-feed` and `C-jw-feed-post-job-lock` were not checked and not merged. When `job.lock` from `C-jd-ic-job-post-exists-fix` is in main, its wait must use `fslock.LockExclusiveContext(s.Context, f)` the same way. |
| Importer list of the changed packages (`go list`), tested | NOT DONE | Only the five packages above ran. The orchestrator runs the whole-module test at landing. |
| New tests fail on base and pass on the lane | NOT DONE | Not run on base. By construction the fslock test cannot compile on main: it names `LockExclusiveContext`, `LockSharedContext` and `OnAbandonedWaitForTest`, which main lacks. That is not a pasted failing run. |
| `lib/lib.go` doc sentence; `lib/ih_call_test.go` hit | HOLDS | Sentence "a call waiting on a Feed lock is not stopped by it" at `lib/lib.go:107` replaced by one that says a `feed post` or `feed handover` waiting on a Feed file lock stops with the context's error and writes nothing. `git grep -n "not stopped by it" -- lib` hit only `lib/lib.go:107`; `lib/ih_call_test.go` has no hit for it, nor for "Feed lock". |
| No guard, gate, cap or hold on agents; nothing removed | HOLDS | The change only lets a wait end on a context. No test deleted or edited. |
| One-sentence commit, no attribution | HOLDS | One commit, `git log -1 --format=%B`. |
| No `VAR=` command, no sleep, no elapsed-time assertion | HOLDS | Waits are channel events; the 30 s timer is the failure path. |

## What changed

- `internal/fslock/jw_context.go` (new): `LockExclusiveContext(ctx, f)` and `LockSharedContext(ctx, f)`. The lock is awaited on a goroutine. The call takes `f` over: with `nil` the lock is held and `f` is the caller's again; on any error (including `ctx.Err()`) the package closes `f`, at once for a failed lock, or by the abandoned waiter, which unlocks before it closes. A cancelled caller never closes or reuses the descriptor the waiter is blocked on. A nil or never-done context waits like `LockExclusive`. A context done before the call returns `ctx.Err()` without asking for the lock. If the lock and the cancel arrive together the lock result wins. `OnAbandonedWaitForTest` is the test hook for the end of an abandoned wait.
- `internal/feed/store.go`, `internal/feed/gate.go`: `Store.Context` (nil is Background); `leaseLock`, the tenure lock in `Post`, `wakeLock` and `gateLock` use the Context calls. The `defer f.Close()` before the lock is gone from those paths; on the tenure path it now follows the lock.
- `internal/orchestrator/store.go`: `exclusive` uses `s.Feed.Context`; `locked` reaches the lease through `Feed.WithLease`, which uses the same Context.
- `internal/cli/feed.go`: `feedStore` sets `Context: env.Context`.
- `lib/lib.go`: the doc sentence.

Behaviour change to know: a Store whose Context is already done now fails its `Post` with `context.Canceled` even when no lock is held (the tenure lock is always taken); before, only the streams honoured the Context.

## Open items and the split

1. Open item, not wired: `internal/cli/orchestrator.go:176` and `:554` and the other `feed.Store{...}` constructors leave `Context` nil, so `exclusive` honours a Context only on a Store built by `feedStore`. Not in Touches; I did not change it.
2. Part 2 is done in lane C-jw-feed-lock-context-tests: see `docs/lanes/C-jw-feed-lock-context-tests.md` for the feed and orchestrator tests, the merge of `origin/main`, the race run, `make cross`, the importer tests, the post-merge `git grep` and the base-failing run. The rows above that read NOT DONE are closed there.

## Open

- `feed.Store` is built with `Context` nil at `internal/cli/orchestrator.go:176` and `:554` (and by any other `feed.Store{...}` literal outside `feedStore` in `internal/cli/feed.go`). A call that goes through those Stores waits on the Feed locks and on `.orchestrator.lock` with no way out, as before this lane. Not fixed here: it is outside the lane's Touches and the brief of the tests lane says to write it down, not to fix it.
- Four plain `fslock.LockExclusive` calls remain in `internal/feed`: `cache.go:52` (`reader.lock`), `fold.go:71` (`fold.lock`), `identity.go:27` (`MachineID`) and `identity.go:126` (`EnsureTaskID`); and one in `internal/orchestrator/plan.go:42`. None is on the wait path of `Store.Post` for a task that already has an id, and none has a Store Context to pass; a stuck holder would still block them.

## Descriptor ownership

A descriptor closed while a flock blocks on it can have its number reused: the blocked call, or the waiter's later unlock, then runs against whatever file the kernel handed that number to. This lane does not let a cancelled caller close the descriptor a waiter is blocked on. `LockExclusiveContext` and `LockSharedContext` take the `*os.File` over (`internal/fslock/jw_context.go`): on `nil` the lock is held and the file is the caller's again; on any error, `ctx.Err()` included, the package closes it, at once when the lock failed, or by the abandoned waiter, which unlocks before it closes once it gets the lock. The `defer f.Close()` that sat before the lock on these paths (`internal/orchestrator/store.go:145`, the tenure lock in `internal/feed/store.go`) now follows a successful lock.

## NOT VERIFIED on a live screen

Nothing in this lane has a screen. No test or build was run for Windows or macOS.
