# Q-tree-orphan-jobs: `grid job tree` names orphaned Jobs

Base: main 330256ce (origin/main, same sha). Branch lane-Q-tree-orphan-jobs.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Test that fails on main | HOLDS | Commit e5eacf3b holds only `TestTreeNamesAckedJobWithNoExitPastItsLease`; run on main code (write.go unchanged) `go test -count=1 ./internal/orchestrator/ -run TestTreeNamesAckedJob` exit 1: `tree_orphan_test.go:36: J-1 acked, lease long past, no worker-exit: want orphan no_exit, got row map[assign:... attempts:[map[attempt:1 fence:2 state:running]] ... status:running ...]` (state `running`, no `orphan`). On the lane the same test exits 0. The other new tests call `TreeAt`/`Orphan`, which main lacks, so they fail to compile there. |
| 2. never_picked_up | HOLDS | `TestOrphanNeverPickedUp` (since = Created; now == Created gives none), `TestOrphanDatedFromAfterClose` (since = closing control `at` of the after Job), `TestOrphanNotWhileWaiting` (open after, open timer gate, Person Job: no orphan). internal/orchestrator/write.go `orphanOf`; readiness comes from the same `Ready` code via `readyAt` (ready.go). |
| 3. never_acked | HOLDS | `TestOrphanNeverAcked`: none at until-1s, `never_acked` since until at until+1s. |
| 4. no_exit | HOLDS | `TestOrphanNoExit`: none before the lease; `no_exit` since `LeaseExpires`; with a result and no worker-exit none; with a worker-exit none. |
| 5. Closed / retried | HOLDS | `TestOrphanClosedAndRetried` (Cancelled, Rejected none; attempt 2 judged, attempt 1's lease ignored, detail names attempt 2); `TestOrphanDatedFromAfterClose` (Accepted J-1 with a stale lease: none). |
| 6. cli text and JSON | HOLDS | `TestJobTreePrintsOrphanLine` (internal/cli/tree_orphan_test.go): text prints `  orphan never_picked_up since <t>: <detail>` under J-1 only; `--json` has `orphan` on J-1 and not on J-2. `TestOrphanAbsentRowJSONUnchanged`: a non-orphan row marshals byte-identical to the old `{*Job, attempts}` shape. cli line: internal/cli/orchestrator.go:186-188. |
| 7. Named test commands | HOLDS | `go test -count=1 ./internal/orchestrator/ ./internal/cli/ -run 'Tree|Orphan'` exit 0; `go test -count=1 ./internal/orchestrator/` exit 0 (47.8s); `go vet ./internal/orchestrator/ ./internal/cli/` exit 0. |
| Importers tested | HOLDS | Non-test importers of internal/orchestrator (grep of imports; `go list` import graph): internal/cli only (cmd/grid imports cli). `go test -count=1 ./internal/cli/ ./cmd/grid/` exit 0 (cli ok 52.0s, cmd/grid no test files). |
| Lint | HOLDS | `make lint` (go vet ./... and gofmt) exit 0 after `gofmt -w` on write.go. |
| Windows / macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No guard/gate/cap/hold | HOLDS | Read-only report; nothing cancels, reassigns or posts. |
| No test edited or deleted | HOLDS | `git diff origin/main --stat`: only two new test files; no existing test file changed. |
| No personal paths/hosts/emails | HOLDS | Added code and docs carry none. |
| New tests in new files | HOLDS | internal/orchestrator/tree_orphan_test.go, internal/cli/tree_orphan_test.go. |

## What changed

- internal/orchestrator/write.go: `Tree(s)` is `TreeAt(s, time.Now())`; `TreeAt` takes a clock; new `Orphan{Kind,Since,Detail}` and `TreeJob.Orphan` (`json:"orphan,omitempty"`); `orphanOf` judges the latest attempt only; closed Jobs have none.
- internal/orchestrator/ready.go: `Ready` is now a wrapper over `readyAt`, which also returns the ready-since times; no behaviour change to Ready (whole orchestrator package passes).
- internal/cli/orchestrator.go: one `  orphan <kind> since <since>: <detail>` line under a Job in `job tree`.
- Two new test files.

## Notes

- `never_picked_up` follows the brief literally: `now` later than the ready time, no grace. A Job posted a moment ago reads as an orphan until the next pass assigns it.
- `since` is written with `contract.FormatTime` (UTC, millisecond Z form).
- If the Feed has a ready-graph error (cycle, missing gate), `Tree` shows no `never_picked_up` rather than failing; the other two kinds are unaffected.

## NOT VERIFIED on a live screen

- The dev UI display of the orphan line / `orphan` JSON field; run only through the cli test harness.
