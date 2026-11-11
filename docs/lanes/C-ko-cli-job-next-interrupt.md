# C-ko-cli-job-next-interrupt

The `job next` Ctrl-C test sends SIGINT on the child's lock-wait event, not after a file poll.

Base: main `21f0273607ae73e7aa8494035743afecdd19f8fa` (`git merge main` says: Already up to date).
Touches: `internal/cli/orchestrator_test.go` only. No production file.

## Base check

On the `job next` branch of `runOrchestrator` (internal/cli/orchestrator.go) the lines before `signalContext` take no `.orchestrator.lock`:
the flag parse, `resolver(env).Resolve` (:147), `feed.EnsureTaskID` (:161), `taskstore.FeedIdentity` (:165),
`feedWriter` / `orchestratorWriter` (:290, :292) and `configureTickStore` (:299) none of them reach `Store.locked` or `Store.exclusive`.
The only product open of `.orchestrator.lock` is internal/orchestrator/store.go:136, in `exclusive`,
reached from `Store.locked` (store.go:112-114) at `Tick` (internal/orchestrator/tick.go:23), which `Next` calls (store.go:202).
`signalContext` is at orchestrator.go:303 and `s.Next` at :305. So a child that reports a wait on `.orchestrator.lock` has its SIGINT handling in place.

One fact the test relies on: the CLI's `feed.Store` (orchestrator.go:172) has no `Context`, so the child's lock wait is a plain `LockExclusive`
(store.go:142 passes a nil context). The test therefore sends SIGINT first and releases the lock after it; the child then runs one pass,
and `pause(ctx)` (store.go:206) returns `context.Canceled`, which the CLI maps to 130 (orchestrator.go:306). Had SIGINT arrived before `signalContext`, the child would die by the signal and not exit 130, so the test still tells the two apart.

## Proof table

| Row | Result |
| --- | --- |
| 1. `git grep -n 'time\.Sleep' -- internal/cli/orchestrator_test.go` prints nothing | HOLDS. It printed nothing, run twice (the tool shows no status; `git grep` exits 1 on no match). Main printed `:341`. |
| 2. `go test -count=1 -v -run 'TestJobNextInterruptExits130\|TestTwoProcessesJobNext' ./internal/cli/` PASS for both, no SKIP | HOLDS, exit 0: `--- PASS: TestTwoProcessesJobNext (0.92s)`, `--- PASS: TestJobNextInterruptExits130 (0.67s)`, `ok ... 1.668s`. |
| 3. `go test -count=3 ./internal/cli/` | HOLDS, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/cli  235.342s`. Also `go test -count=20 -run TestJobNextInterruptExits130 ./internal/cli/`: `ok ... 9.378s`. |
| 4. `git diff main --stat` names only internal/cli/orchestrator_test.go | HOLDS: `internal/cli/orchestrator_test.go \| 115 +++++++++++++++++++++++++++++++-------`, `1 file changed, 94 insertions(+), 21 deletions(-)`. `git status --short` is clean. That stat is of the code commit; the next commit adds only this report (`docs/lanes/C-ko-cli-job-next-interrupt.md`), which the runbook asks for. |
| 5. Walk tests after `git merge main` | HOLDS, exit 0: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` gave `ok ... 0.084s`; `go test -count=1 ./internal/job/lint/` gave `ok ... 0.303s`. Run on head after the merge (main had nothing new). |
| 6a. `make lint` | HOLDS, exit 0 (`go vet ./...` printed nothing). `gofmt -l internal/cli` printed nothing. |
| 6b. `make cross` | HOLDS, exit 0 (background task notice `completed (exit code 0)`; log has `cross: windows/amd64 vet`, `windows/amd64 test compile`, `cross: darwin/arm64 test compile`). |
| Rule: tests of changed package and importers | HOLDS, exit 0. Changed package: `internal/cli` (row 3). Importers by `go list` (Imports, TestImports, XTestImports): `cmd/grid`, `contract/formats`, `internal/account`, `internal/braincmd`, `internal/golden`, `internal/jobgraph`, `internal/mcp`, `internal/packaging`, `internal/question`, `internal/settings`, `lib`. `go test -count=1` on all of them: every package `ok` (`cmd/grid` has no test files). |
| Rule: repo-wide walk tests (plugin) | HOLDS: see row 5. The app-only walk tests (`make webui-test` and others) do not apply to this plugin repo. |
| Rule: new tests fail on base, pass on lane | N/A as a new test. This rewrites `TestJobNextInterruptExits130`, which passed on base too; it only changes what the test waits on. Row 1 is the evidence the poll is gone, and the child prints the line only when its lock call is about to wait. I did not edit any existing test to make it pass and deleted no test. |
| Rule: `*RealPlugin*` tests | N/A: no row names one and none was touched. No SKIP in the rows above. |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS: only test code changed. `fslock.OnBlockForTest` already exists on main (internal/fslock/fslock.go:33) and is called only from a `_test.go` file, which `TestIfLwSeamOnlyInTests` (internal/fslock/if_wait_hook_test.go:211) allows; `go test ./internal/fslock` is not needed since no fslock file changed. |
| Rule: no sleep, poll loop, elapsed-time assertion or timeout as proof | HOLDS. The two `time.After(10 * time.Second)` cases in `TestJobNextInterruptExits130` are the failure bound of a select whose other cases are the event (`output.found`, or `exited`); neither is the proof. |
| Rule: commits and public repo | HOLDS: one commit with a one-sentence message, no attribution lines; no personal path, host or email in the diff. |

## What changed (internal/cli/orchestrator_test.go)

- `TestJobNextProcess`: when `GRID_TEST_NEXT_BLOCKED=1` is set, calls `koJnAnnounceBlock()` before `Run`. That sets `fslock.OnBlockForTest` with a hook that, for a path ending in `.orchestrator.lock`, writes `koJnBlockedLine` to stderr and returns (it never blocks). Without the variable nothing changes, so `TestTwoProcessesJobNext` is untouched.
- New helpers, all prefixed `koJn`: `koJnBlockedLine` (the fixed line), `koJnAnnounceBlock` (child hook), `koJnOutput` (an `io.Writer` that keeps the child's stdout and stderr and closes `found` when the text holds the line), `koJnHold` (takes `.orchestrator.lock` in the Feed folder with `fslock.OpenNoFollow` and `fslock.LockExclusive`, as store.go:136-142 opens it; released by the returned function or `t.Cleanup`).
- `TestJobNextInterruptExits130`: keeps the first `job next`, so J-1 is assigned and the child's pass assigns nothing and goes to its wait. The `state.json` removal and the poll loop are gone. The parent holds the lock, starts the child with `GRID_TEST_NEXT_BLOCKED=1`, waits on a select (`output.found` / child exit fails the test / 10 s failure bound), sends SIGINT, releases the lock, waits for the exit (same failure bound). The assertions stay: exit code 130 and no `context canceled` in the output.
- Imports added: `sync`, `internal/fslock`.

## NOT VERIFIED on a live screen

Nothing here needs a screen. A real Ctrl-C from a terminal against the built `grid` binary was not run: the proof is the in-process child that receives `os.Interrupt` from the parent.

The `go-conventions` skill named in the brief was not in this session's skill list; the code follows the style of the neighbouring tests in the package (gofmt clean, `go vet` clean).

DONE
