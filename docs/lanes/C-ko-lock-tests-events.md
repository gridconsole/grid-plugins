# C-ko-lock-tests-events

Brain and localdata lock tests wait on lock events from the children, not on marker polls and a 2 ms sleep.

Base: lane branch cut from main at `21f0273607ae73e7aa8494035743afecdd19f8fa` (main is the same sha; `git merge main` says "Already up to date."). The brief names main `dc301fce`; main has moved since. Every line the brief quotes holds on this base: brain/lock_test.go waitFor sleep :79, marker waits :101 and :122, `time.Sleep(time.Hour)` :41; localdata/lock_test.go waitUntil sleep :37, marker wait :74, `time.Sleep(2 * time.Millisecond)` :108, hour sleep :27; the hook `fslock.OnBlockForTest` at internal/fslock/fslock.go:33, called by `lockWaiting` (:47-57) from both `locked` (internal/brain/store.go:161) and `LockExisting` (internal/localdata/localdata.go:177).

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1 | no `time.Sleep` but the hour helpers | HOLDS. `git grep -n 'time\.Sleep' -- internal/brain/lock_test.go internal/localdata/lock_test.go \| grep -v 'time.Sleep(time.Hour)'` prints nothing (grep exit 1 = no line left). The two lines left are the helper children, brain/lock_test.go:59 and localdata/lock_test.go:33. |
| 2 | four tests PASS, no SKIP | HOLDS. `go test -count=1 -v -run 'TestTwoProcessesProposeAtOnce\|TestKilledWriterLeavesNoStaleLock' ./internal/brain/` exit 0: `--- PASS: TestTwoProcessesProposeAtOnce (0.05s)`, `--- PASS: TestKilledWriterLeavesNoStaleLock (0.01s)`. `go test -count=1 -v -run 'TestKilledWriterDoesNotBlockTheNextWrite\|TestLockSerializesWriters' ./internal/localdata/` exit 0: `--- PASS: TestKilledWriterDoesNotBlockTheNextWrite (0.00s)`, `--- PASS: TestLockSerializesWriters (0.00s)`. No SKIP line in either. |
| 3 | `go test -count=3 ./internal/brain/ ./internal/localdata/` | HOLDS, exit 0: `ok .../internal/brain 8.568s`, `ok .../internal/localdata 0.139s`. Also with `-race` and `-count=3`: exit 0 (`ok brain 46.261s`, `ok localdata 1.454s`). |
| 4 | diff names only the two files | HOLDS. `git diff main --stat` before the report commit: `internal/brain/lock_test.go`, `internal/localdata/lock_test.go`, 2 files changed, 209 insertions, 49 deletions. The report commit adds `docs/lanes/C-ko-lock-tests-events.md`, which the runbook requires; no other file. |
| 5 | walk tests after `git merge main` | HOLDS. `git merge main`: "Already up to date." `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok internal/cli 0.118s`). `go test -count=1 ./internal/job/lint/` exit 0 (`ok internal/job/lint 1.731s`). Also `go test -count=1 ./internal/fslock/` exit 0, which holds `TestIfLwSeamOnlyInTests` (test files may name the seam). |
| 6 | `make lint`, `make cross` | HOLDS. `make lint` exit 0 (`go vet ./...`; `gofmt -l internal/brain internal/localdata` prints nothing). `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |

### Rules

| Rule | Result |
|------|--------|
| small commits, one-sentence messages, no attribution lines | HOLDS. One code commit, "Make the brain and localdata lock tests wait on lock events from the children instead of marker polls and a sleep."; one report commit. No Co-Authored-By or session line. |
| new tests fail when the behaviour is wrong | HOLDS by mutation of the test file (these four tests passed on base with sleeps and markers, so there is no failing-on-base run). Brain: parent no longer holds the lock while the children start (the `locked(root, ...)` call replaced by a plain call), `go test -count=1 -v -run TestTwoProcessesProposeAtOnce ./internal/brain/` exit 1: `lock_test.go:179: timed out waiting for writer first to wait for the lock; the child wrote:` / `PASS` / `--- FAIL: TestTwoProcessesProposeAtOnce (10.03s)`. That is the failure the old marker wait could not show: the child started and was not waiting. Localdata: hook path suffix changed to `.nope.lock`, `go test -count=1 -v -run TestLockSerializesWriters ./internal/localdata/` exit 1: `lock_test.go:188: no other writer ever waited for the lock` / `--- FAIL: TestLockSerializesWriters (10.04s)`. Both reverted with `git checkout -- <file>`; `git status --short` printed nothing afterwards. |
| no existing test edited to pass, none deleted | HOLDS. The tests keep their names and checks (`inside` check, killed-writer checks, proposal-lost check). Removed are helpers, not tests: brain `waitFor`, `touch` and the `marker` field; localdata `waitUntil`. |
| no guard, gate, cap, hold or blocking hook on agents | HOLDS. Test files only. |
| tests of the changed packages and every importer | HOLDS, exit 0 for `go test -count=1` on: contract/formats, internal/brain, braincmd, cli, hook, job, job/jobcmd, job/jobtest (no test files), localdata, people, projectmd, projects, proof, routine, signal, task, updates (no test files), usage, worker, workflow, fslock. The list is from `go list` over Imports, TestImports and XTestImports of internal/brain and internal/localdata. |
| lint and Windows and macOS builds | HOLDS, see row 6. |
| repo-wide walk tests | The plugin ones HOLD (row 5). The app ones (`make webui-test` and the grid-server and slices walks) belong to the app repo and do not exist here: NOT RUN, not applicable to this repo. |
| *RealPlugin* tests | None involved. |
| public repo: no personal paths, hosts or emails | HOLDS. Only package-relative paths in the diff and this report. |

## What changed

- `internal/brain/lock_test.go`: the "propose" child sets `fslock.OnBlockForTest` before `Propose` and writes the line `KO-LOCK-BLOCKED` when the path ends in `.brain.lock` (lock_test.go:44-47); the "hold" child writes `KO-LOCK-HELD` inside the lock (:59). The parent reads the child's stdout and stderr through an `os.Pipe` line by line (`koLkStart`, `koLkPipe.read`), keeps the text for failure messages, and waits in `koLkWait` with a select on the event and the 10 s failure bound (:138-145). `TestTwoProcessesProposeAtOnce` now holds the lock until both children have said they are about to wait on it (:179), so the comment at the old :94-95 is now true. `TestKilledWriterLeavesNoStaleLock` waits for the held line (:200). The marker files, `touch`, `waitFor` and the 5 ms poll are gone. The hour sleep of the "hold" child stays.
- `internal/localdata/lock_test.go`: the helper child writes `KO-LOCK-HELD` after `Lock` in place of the marker file; the same pipe reader and `koLkWait` (:49-103); `TestKilledWriterDoesNotBlockTheNextWrite` waits on the line (:134). `waitUntil` is gone. `TestLockSerializesWriters` sets `fslock.OnBlockForTest` (restored with `t.Cleanup`, no `t.Parallel`) to feed a channel with one slot per writer for paths ending `.test.lock` (:158-166); the first writer to get the lock stays inside until that channel shows another writer waiting on it (:186-187, failure bound 10 s), the other seven leave at once, so an overlap is certain; the `inside` check stays. The 2 ms sleep is gone. The hour sleep of the helper child stays.
- New helpers all start with `koLk` (`koLkHeld`, `koLkBlocked`, `koLkSay`, `koLkPipe`, `koLkStart`, `koLkWait`); the pipe code is duplicated in the two packages because only these two files may change.
- No production file changed; no product hook was needed. `fslock.OnBlockForTest` already exists on main.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: the two tests on a real Windows or macOS machine; `make cross` only vets and compiles them there. The hook's Windows path (`LockFileEx` try, then block) is the product's, covered by internal/fslock tests that this lane does not touch.

DONE
