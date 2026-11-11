# C-ko-hook-lockorder-event

Base: main 21f02736 (dc301fce is an ancestor). Base check holds: `internal/hook/lockorder_test.go:67` is `time.Sleep(500 * time.Millisecond)` on base (`git grep -n 'time\.Sleep' HEAD -- internal/hook/lockorder_test.go` printed `HEAD:internal/hook/lockorder_test.go:67`), and `task.AppendHistoryWhy` opens `.task.lock` with fslock (`internal/task/history.go:133` OpenNoFollow, `:138` LockExclusive). The event exists: `internal/fslock/fslock.go:33` OnBlockForTest, `:47-57` lockWaiting.

## Proof table

| Row | Result |
| --- | --- |
| 1. no `time.Sleep` in `internal/hook/lockorder_test.go` | HOLDS. `git grep -n 'time\.Sleep' -- internal/hook/lockorder_test.go` printed nothing (the shell tool shows a no-match grep as "no output"; the base file printed `:67`). |
| 2. `go test -count=1 -v -run 'TestHookAndAttachLockOrderFinishes' ./internal/hook/` | HOLDS, exit 0: `--- PASS: TestHookAndAttachLockOrderFinishes (0.06s)`, `--- PASS: TestHookAndAttachLockOrderFinishes/bash_history (0.06s)`, `--- PASS: TestHookAndAttachLockOrderFinishes/brain_read (0.01s)`. |
| 3. `go test -count=3 ./internal/hook/` | HOLDS, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/hook  31.331s`. |
| 4. `git diff main --stat` names only `internal/hook/lockorder_test.go` | HOLDS: `internal/hook/lockorder_test.go | 51 +++++---` (1 file changed) before this report was added; the report adds only `docs/lanes/C-ko-hook-lockorder-event.md`, which the runbook asks for. |
| 5. walk tests after `git merge main` | HOLDS. `git merge main`: `Already up to date.` `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'`: `ok  .../internal/cli  0.124s`, exit 0. `go test -count=1 ./internal/job/lint/`: `ok  .../internal/job/lint  0.289s`, exit 0. Also `go test -count=1 ./internal/fslock/` (holds TestIfLwSeamOnlyInTests, the walk that keeps OnBlockForTest to tests): `ok  0.133s`. |
| 6a. `make lint` | HOLDS, exit 0 (`go vet ./...`, no output after it). gofmt -l internal/hook printed nothing. |
| 6b. `make cross` | HOLDS, exit 0 (`[exited with code 0]`): the output shows `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`, with no FAIL. The command ran past the shell's 600 s foreground limit and finished in the background. |
| Packages that import hook (go list, imports, test imports and external test imports): | `internal/cli`, `internal/golden`, `internal/packaging`. `go test -count=1 ./internal/cli/`: `ok  92.016s`; `go test -count=1 ./internal/golden/ ./internal/packaging/`: `ok  1.293s`, `ok  7.156s`; hook itself above. All exit 0. |
| New test fails on base, passes on lane | Not a new test: the rewrite changes how an existing test waits, and the brief gives no failing base case (on base the test passes when the VM is fast enough). Nothing was weakened: the history check, the 10 s bounds and the P4 failure branch are as before. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS: test file only, no production file touched. |
| Test deleted or edited to pass | None deleted. `lockOrderFinishes` is edited (that is the brief); `TestHookAndAttachLockOrderFinishes` is not changed. |
| Repo-wide walk tests of the app (`make webui-test`, TestFvArgvCollects, grid-server mounts, slices Imports) | Not applicable: the plugin repo has none of them; the plugin walks are row 5. |

## What changed

`internal/hook/lockorder_test.go` only.

- New helper `koHlOnTaskLock(t)`: sets `fslock.OnBlockForTest` with `t.Cleanup` restoring it, and returns a channel (buffer 1). The hook compares `filepath.Base(path)` to `.task.lock` and sends without blocking (`select` with `default`).
- `lockOrderFinishes`: after the test holds `.task.lock` and before `f.start`, it calls the helper. The 500 ms sleep is now a `select` on the channel (the hook is about to wait on `.task.lock`), on `hookDone` (the hook ended without waiting; its log is kept and the history check still runs) and on a 10 s `time.After` as the failure bound. That failure path releases the lock, gives the hook a bounded drain, and `t.Fatal`s with a P4 message.
- The final step takes the hook's log from `hookDone` only when the first select did not already take it.
- The test does not call `t.Parallel`.

The lock order the test proves is unchanged: the hook has reached `.task.lock` and waits there, then the attach side takes the session lock and must finish.

## NOT VERIFIED

- Nothing needs a live screen; the change is in a Go test.
- Not run: a mismatch proof that the test fails when the hook holds the session lock while waiting for `.task.lock` (the brief asks for none).
