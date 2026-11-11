# C-gt-orch-next-wait-no-clock

The two Next wait tests now count pauses and reach the real one-second tick through a seam. They carry no elapsed time.

Code commit: 7513445f ("Move the one-second Next tick into tickPause and test the two waits by counting pauses."). This report is a second commit on top of it.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. `go test -count=1 -run '^TestNext' -v ./internal/orchestrator/` | HOLDS, exit 0 | `--- PASS: TestNextWithoutWaitReturnsAfterOnePass (0.05s)`, `--- PASS: TestNextWaitReturnsAssignedJobBeforeTheTick (0.10s)`, `--- PASS: TestNextWaitStopsWhenContextIsCancelled (0.01s)`, `ok ... internal/orchestrator 0.243s` |
| 1. `go test -count=20 -run '^TestNext(WithoutWait\|WaitStops)' ./internal/orchestrator/` | HOLDS, exit 0 | `ok  github.com/gridconsole/grid-plugins/internal/orchestrator 1.306s` |
| 2. `git grep -n -E 'time\.(Since\|Sleep\|AfterFunc)' -- internal/orchestrator/load_next_test.go` | HOLDS | prints nothing (git grep exit 1 = no match) |
| 3. Delete the `case <-ctx.Done():` branch of `tickPause`; run `TestNextWaitStopsWhenContextIsCancelled` | HOLDS, FAIL seen, exit 1 | `--- FAIL: TestNextWaitStopsWhenContextIsCancelled (1.05s)`, `load_next_test.go:263: Next paused again after cancel`, `load_next_test.go:277: Next paused 2 times, want 1`. Restored with `git checkout -- internal/orchestrator/store.go`; `git status --short` printed nothing. |
| 4. Call the pause once in `Next` before returning when `wait` is false; run `TestNextWithoutWaitReturnsAfterOnePass` | HOLDS, FAIL seen, exit 1 | `--- FAIL: TestNextWithoutWaitReturnsAfterOnePass (0.06s)`, `load_next_test.go:222: TestNextWithoutWaitReturnsAfterOnePass: Next waited between passes`. Restored with `git checkout -- internal/orchestrator/store.go`; `git status --short` printed nothing. |
| 5. `go test -count=1 ./internal/orchestrator/` | HOLDS, exit 0 | `ok  github.com/gridconsole/grid-plugins/internal/orchestrator 68.580s` |
| 5. `git diff main --stat` after the restore (code commit) | HOLDS | `internal/orchestrator/load_next_test.go \| 32 +++`, `internal/orchestrator/store.go \| 31 +++`; 2 files changed, 42 insertions, 21 deletions. The only other file in the lane is this report, which the builder runbook requires. |
| 5. `gofmt -l internal/orchestrator` | HOLDS, exit 0 | prints nothing |
| 5. `go vet ./internal/orchestrator/` | HOLDS, exit 0 | prints nothing |
| 5. `GOOS=windows go vet ./internal/orchestrator/` | HOLDS, exit 0 | prints nothing |

## Rules

| Rule | Verdict | Proof |
|---|---|---|
| Packages that import the changed one | HOLDS | `go list` import map: `internal/cli` and `internal/migrate` import `internal/orchestrator`; `cmd/grid` and `lib` reach it through `cli`. |
| Tests of the importers | HOLDS, exit 0 | `go test -count=1 ./internal/migrate/ ./internal/cli/` printed `ok .../internal/migrate 16.369s` and `ok .../internal/cli 45.266s`; `go test -count=1 ./cmd/grid/ ./lib/` printed `cmd/grid [no test files]` and `ok .../lib 2.335s` |
| Lint | HOLDS, exit 0 | `make lint` (runs `go vet ./...`) printed nothing else |
| Windows build | HOLDS, exit 0 | `GOOS=windows go build ./...` |
| macOS build | HOLDS, exit 0 | `GOOS=darwin go build ./...` |
| New tests fail on base | PARTLY: the cancel test fails on base; the one-pass test cannot | With main's `store.go` in place, `go test -count=1 -run '^TestNext(WithoutWait\|WaitStops)' ./internal/orchestrator/` printed `load_next_test.go:267:10: undefined: tickPause` and `FAIL ... [build failed]` (exit 1). `TestNextWithoutWaitReturnsAfterOnePass` passes on base because base has no slow-pass failure to show on a fast machine; row 4 proves it can fail. Lane `store.go` restored with `git checkout HEAD -- internal/orchestrator/store.go`; `git status --short` printed nothing. |
| No existing test edited to pass, none deleted, none renamed, no t.Skip | HOLDS | Only the bodies of `TestNextWithoutWaitReturnsAfterOnePass` and `TestNextWaitStopsWhenContextIsCancelled` changed. Every non-clock assertion stays with its failure words: `empty feed gave %+v`, `err = %v, out = %+v`, `Assigned = %d`. |
| No guard, gate, cap or hold on agents | HOLDS | `tickPause` is the same one-second wait that was inline in `Next`; no new limit. |
| No clock in the two tests | HOLDS | Row 2 output is empty; both tests wait on nothing. |
| `*RealPlugin*` tests | N/A | No row names one; no plugin build was needed. |

## What changed

- `internal/orchestrator/store.go`: the inline one-second timer in `Next` moved into `func tickPause(ctx context.Context) error` (one-second timer, `ctx.Err()` on `ctx.Done()`, nil on the timer). `Next` uses `s.pause` when set and `tickPause` otherwise; behaviour is unchanged.
- `internal/orchestrator/load_next_test.go`:
  - `TestNextWithoutWaitReturnsAfterOnePass` sets `f.s.pause` to a counter and asserts 0 calls. The `time.Since` check is gone.
  - `TestNextWaitStopsWhenContextIsCancelled` sets `f.s.pause` to a func that cancels on the first call and returns `tickPause(ctx)`; a second call records `t.Error("Next paused again after cancel")`. It asserts `errors.Is(err, context.Canceled)`, `Assigned == 0` and exactly one pause. `time.AfterFunc` and the elapsed-time check are gone.

## Notes

- Residual race (pathological only): in the cancel test `ctx` is already done when `tickPause` reaches its `select`, while the one-second timer was created a statement earlier. The test could flake only if the goroutine stalled for a full second between those two statements, so the timer and `ctx.Done()` were both ready. The brief's shape keeps `tickPause` as it was; a pre-check of `ctx.Err()` would hide the break in row 3.
- With the `ctx.Done()` branch deleted (row 3) the broken test takes 1.05 s, because it then waits the real timer. The passing lane never waits.

## NOT VERIFIED on a live screen

Nothing: no UI row.
