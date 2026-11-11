# Q-tree-pickup-times

Branch lane-Q-tree-pickup-times, stacked on Q-fold-exit-code (merged in at b2a23eb6). Code commit 98b55cec. main is ecd80ef5 and lacks the fold exit code, so the "fails on base" run below is on b2a23eb6, the lane's parent, where only this lane's change is missing.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Test fails on base, no field holds t2 | HOLDS | `go test -count=1 -run 'TestTree(CarriesPickupTimes\|ReadyAt\|RetriedJobTimes\|PersonAndUnacked)' ./internal/orchestrator/` on b2a23eb6 plus the new test file: exit 1. `--- FAIL: TestTreeCarriesPickupTimes` / `tree_times_test.go:104: want assigned_at 2026-11-11T02:54:42.401Z acked_at 2026-11-11T02:54:42.414Z, got row map[... attempts:[map[attempt:1 fence:2 state:running]] ...]` (no assigned_at, acked_at, ready_at). The other three tests failed the same way (lines 144, 179, 202). |
| 2. attempt assigned_at, acked_at, exited_at; row ready_at (no after) and pickup_seconds = ack - ready | HOLDS | `TestTreeCarriesPickupTimes` passes (tree_times_test.go:93). Code: write.go `Attempt` fields, TreeAt fills them from the record `At` of assign, ack and worker-exit; `ready_at` from `since[id]`, the helper readyAt returns. exited_at is absent before a worker-exit. |
| 3. after: ready_at is the At of the record that closed J-1 | HOLDS | `TestTreeReadyAtIsTheAfterClose` passes: ready_at is nil while J-1 is open, then equals the At of J-1's accepted control record (not J-2's post time), and pickup_seconds is ack minus that close. |
| 4. Retried Job: own times per attempt; pickup uses latest ack and original ready_at | HOLDS | `TestTreeRetriedJobTimes` passes: attempt 1 has assigned, acked and exited; attempt 2 has assigned and acked and no exited_at; ready_at is the post; pickup_seconds is ack 2 minus the post. |
| 5. Person Job: ready_at, no pickup_seconds; no ack: no pickup_seconds | HOLDS | `TestTreePersonAndUnackedJobsHaveNoPickup` passes (a Person Job with no attempts has ready_at; an assigned, unacked J-1 has assigned_at and no acked_at or pickup_seconds). |
| 6. Text output byte-identical | HOLDS by construction | The text printer (internal/cli/orchestrator.go:180-199) reads only ID, Role, Status, DiscoveredFrom, attempt number/fence/state/exit/code/reason_missing and Orphan. It is untouched: `git diff --stat b2a23eb6..HEAD` shows only write.go and tree_times_test.go. `go test -count=1 ./internal/cli/` exit 0, including `TestJobTreePrintsOrphanLine` and `TestJobTreeShowsTheWorkerExitCode`, which assert the text lines. No compared-output file was made. |
| 7. orchestrator tests and vet | HOLDS | `go test -count=1 ./internal/orchestrator/` exit 0 (ok 31.7s); `go vet ./internal/orchestrator/ ./internal/cli/` exit 0. |
| Importers tested | HOLDS | `go list` import scan: only `internal/cli` imports `internal/orchestrator`. `go test -count=1 ./internal/orchestrator/ ./internal/cli/` exit 0. |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0; `gofmt -l internal/orchestrator` empty after gofmt -w. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Existing tests not edited, none deleted | HOLDS | diff touches only write.go and the new tree_times_test.go. |
| No guard, gate, cap or hold on agents | HOLDS | Read-model fields only; nothing blocks or limits. |
| New tests in a new file | HOLDS | internal/orchestrator/tree_times_test.go. |
| No personal paths, hosts or emails, no attribution lines | HOLDS | commit messages are one sentence with no trailer. |

## What changed

- `internal/orchestrator/write.go`:
  - `Attempt` gains `assigned_at`, `acked_at` and `exited_at` (RFC3339, omitempty). They are the record `At` of the assign, the ack and the worker-exit, formatted like the other tree times.
  - `TreeJob` gains `ready_at` (omitempty) and `pickup_seconds` (`*float64`, omitempty).
  - `ready_at` comes from the `since` map that `readyAt` returns, the same source the orphan rule uses. It is the post time, or the `At` of the record that closed the last Job it waited on.
  - A Job gets `ready_at` when it has no unmet after, or has an attempt. A Job still waiting on an open after or gate has none. A person's Job is held out of `Ready` by a wait with no target, and counts as ready.
  - `pickup_seconds` is the latest attempt's ack minus `ready_at`, absent without an ack.
- `internal/orchestrator/tree_times_test.go`: four tests (rows 1-5). They read the row through `json.Marshal`, so they check the shape `grid job tree --json` prints.

## Notes

- Retry question from row 4: after a retry, `pickup_seconds` still counts from the original ready time, so a crash and retry adds the whole first attempt to the latency. If the retry's own control record (the move back to ready) should restart the clock, that is a separate change; it was not added.
- A Job that was cancelled or rejected before it ever became ready has no `ready_at`.
- A Job still waiting on a gate or after has no `ready_at`, even though `since` holds a value for it.

## NOT VERIFIED on a live screen

- `grid job tree --json` against a real task Feed, and the dev UI reading the new fields. Only the Go tests and fixtures were run.

DONE
