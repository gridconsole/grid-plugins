# C-iq-ports-group-stop-wait

Base: lane started at ee8a96c1 (checked out HEAD; the brief names main 91fadeff). process_unix_test.go:132 held `case <-time.After(2 * time.Second):` in TestStopAfterGroupLeaderExits, so the brief applies.

## Proof table

| Row | Result |
|---|---|
| 1. no `time.After` in the test; both subtests pass | HOLDS. `git grep -n 'time.After' -- internal/ports/process_unix_test.go` prints nothing. `go test -count=1 -v -run 'TestStopAfterGroupLeaderExits\|TestIqPs' ./internal/ports/`: ok; `--- PASS: .../StopRecord`, `.../Stop`, `.../StopRecordIgnoringInterruptAndTerm`. |
| 2. mutation `signal(pid, sig)` at process_unix.go:81 | HOLDS. `go test -count=1 -timeout 90s -run '^TestStopAfterGroupLeaderExits$/^StopRecord$' ./internal/ports/` gave `--- FAIL: .../StopRecord (81.11s)` / `process_unix_test.go:108: group child survived shutdown` (from the test at 81 s = 90 s less a tenth, not a timeout panic). Reverted with `git checkout -- internal/ports/process_unix.go`. Survivors: the cleanup `syscall.Kill(-pid, SIGKILL)` in iqPsOrphanGroup (iq_stop_wait_unix_test.go:62) kills the group; my direct process listing was refused in the sandbox, so NOT directly observed. |
| 3. mutation: SIGKILL dropped from ladder, new trap subtest | HOLDS. `go test -count=1 -timeout 90s -run '^TestStopAfterGroupLeaderExits$' ./internal/ports/`: only `.../StopRecordIgnoringInterruptAndTerm (80.39s)` FAILS with `process_unix_test.go:108: group child survived shutdown`; StopRecord and Stop pass. Unmutated code passes the subtest (row 1). Reverted; subtest kept. |
| 4. TestIqPsLateEOF | HOLDS. iq_stop_wait_unix_test.go TestIqPsLateEOF: SIGSTOP to the group (leader gone), StopRecord, EOF, `hang.Err()` nil. `--- PASS: TestIqPsLateEOF (0.39s)`. |
| 5. TestIqPsHangDeadline | HOLDS. Checks the ctx deadline is in [D-1min, D]. PASS. Also TestIqPsHangEnd checks the 1/10 and 1 minute cap on fixed times. PASS. |
| 6. diff names two files; package ok; lint | HOLDS. `git diff main --stat`: iq_stop_wait_unix_test.go, process_unix_test.go only (mutations reverted, `git status --short` clean). `go test -count=1 ./internal/ports/`: ok. `make lint`: exit 0. |
| Importers of internal/ports (go list) | internal/cli, internal/env. `go test -count=1 ./internal/cli/ ./internal/env/`: ok both. |
| Windows / macOS builds | `GOOS=windows CGO_ENABLED=0 go build ./...` ok; `GOOS=darwin CGO_ENABLED=0 go build ./...` ok; darwin vet of internal/ports ok. |
| New tests fail on base | Covered by mutation rows 2 and 3 (the base wait could not tell a dead survivor from a hung one; the new tests fail on the broken ladder). The new tests need the new helper, so they do not compile on base. |
| Existing tests not edited to pass / none deleted | Only TestStopAfterGroupLeaderExits changed, as the brief allows; its body now uses the helper in the new file. No test deleted. |
| No guard, gate, cap on agents; no production file | HOLDS. No production file touched. |
| *RealPlugin* tests | None named in the brief. |

## What changed
- internal/ports/iq_stop_wait_unix_test.go (new): `iqPsSurvivor` (`sleep 2147483647 & sleep 0.3`), `iqPsHangCtx`, `iqPsHangEnd`, `iqPsOrphanGroup`, TestIqPsHangDeadline, TestIqPsHangEnd, TestIqPsLateEOF.
- TestStopAfterGroupLeaderExits: waits on `exited` and the hang context; survivor never ends by itself; third subtest ignores INT and TERM.

## NOT VERIFIED
- Leftover processes after the mutation runs were not listed (process listing refused in the sandbox); the cleanup kill is the only evidence.
- TestIqPsHangDeadline skips if run with `-timeout 0` (no test deadline).
- The decision that the one-minute cap limits the margin subtracted from the deadline (row 5 reading), not the wait length.
