# C-gt-land-timeout-no-clock

| Row | Result |
|---|---|
| 1. `go test -count=1 -run '^(TestFailureTimeoutAndEnvironment\|TestTimeoutKillsDescendants)$' -v ./internal/land/` | HOLDS, exit 0: `--- PASS: TestFailureTimeoutAndEnvironment (0.13s)`, `--- PASS: TestTimeoutKillsDescendants (0.03s)`. `-count=10`: `ok ... 1.987s` |
| 2. `git grep -n -E 'time\.(Since\|Sleep)' -- internal/land/land_test.go internal/land/process_unix_test.go` | HOLDS: no output |
| 3. Kill `cmd.Process.Pid` at both process_unix.go:15 and :25 | HOLDS: `--- FAIL: TestTimeoutKillsDescendants (30.39s)` / `process_unix_test.go:102: descendant survived timeout`. Restored with `git checkout`. |
| 4. land.go:249 "failed" instead of "timed-out" | HOLDS: `--- FAIL: TestFailureTimeoutAndEnvironment/timeout` (`land_test.go:196: result {... Status:failed ...}`) and `--- FAIL: TestTimeoutKillsDescendants` (`process_unix_test.go:78`). Restored. |
| 5. `go test -count=1 ./internal/land/` | HOLDS: ok. `git diff main --stat`: land_test.go and process_unix_test.go only. `gofmt -l internal/land` prints nothing. `go vet` and `GOOS=windows go vet` exit 0, as does `GOOS=darwin go vet`. |
| Importers' tests | HOLDS: `go list` shows only internal/cli imports internal/land (cmd/grid and lib reach it through cli). `go test -count=1 ./internal/cli/`: ok. |
| Rules: small commits, no attribution, no guard on agents, no test deleted or renamed, no skips | HOLDS |
| Break proofs left clean | HOLDS: `git status --short` shows only the two test files. |

## What changed
- land_test.go: removed the `time.Since` elapsed check and its `start` variable. The "timed-out" status stays the proof.
- process_unix_test.go: added `gtLandReadyCtx`, `gtLandPid` and `gtLandReady`. The Done channel closes when `ready` exists and `bg.pid` holds a whole line, and Err then returns `context.DeadlineExceeded`. The test passes it in with `s.Timeout = 60s`. It asserts "timed-out", waits for the bg.pid process to be gone (failure bound 30 s, "descendant still alive"), then asserts `survived` is absent. The Sleep is dropped.

## Notes
- The helpers poll with a 5 ms ticker; no sleep or elapsed assertion is in the tests.
- The "timeout" row of TestFailureTimeoutAndEnvironment still uses its 30 ms timeout, which the brief keeps. The status is the proof.
- Break row 3 fails through `survived` appearing after the 30 s bound, so it takes about 30 s.

## NOT VERIFIED on a live screen
Nothing; there is no UI.
