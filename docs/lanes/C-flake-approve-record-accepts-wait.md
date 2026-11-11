# C-flake-approve-record-accepts-wait

| Row | Result |
|---|---|
| 1. Poll ends only when approve Job exists AND J-1 has one accept; 30 s deadline kept; message names the missing one; later assertions untouched | HOLDS: internal/cli/approve_record_test.go:390-406; diff is 7 added, 2 removed lines, all inside the loop |
| 2. Same shape elsewhere in internal/cli/*_test.go | None. `grep arAccepts` shows the other calls are direct reads after synchronous commands; the Sleep loops in orchestrator, open_start, signal_act wait for a child to start, not for a second record |
| 3. `make test "GOFLAGS=-count=1 -v -run=TestApproveRecordWorkerPass"` | HOLDS, exit 0: `--- PASS: TestApproveRecordWorkerPassMovesTheReceiptOfATaskOnlyRead (0.49s)` / `ok .../internal/cli 0.521s` |
| 3b. same with -count=20 | HOLDS, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/cli 0.348s` |
| 4. `make lint` | HOLDS, exit 0 (go vet ./...) |
| 5. git log | pasted in report |
| Importers of internal/cli (go list) | cmd/grid (no tests), lib. `go test -count=1 ./internal/cli ./lib ./cmd/grid` exit 0 |
| Windows / macOS builds | `GOOS=windows go vet ./internal/cli` and `GOOS=darwin go vet ./internal/cli` exit 0 |
| New test fails on base | NOT APPLICABLE: no new test; this fixes a timing race in an existing test, which cannot be made to fail deterministically on base |
| No production file, no guard added | HOLDS: only approve_record_test.go changed |

## What changed
The wait loop now also requires the accept to exist, and the timeout message says whether the Job or the accept was missing.

## NOT VERIFIED
The fix under real load (landing box load 40-70); the race was not reproduced here.
