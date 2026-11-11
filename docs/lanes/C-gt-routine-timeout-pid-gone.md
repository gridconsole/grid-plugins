# C-gt-routine-timeout-pid-gone

| Row | Result |
|---|---|
| 1. `go test -count=1 -run '^TestProcessEnvironmentAndTimeout$' -v ./internal/routine/` | HOLDS, exit 0: `--- PASS: TestProcessEnvironmentAndTimeout (0.08s)`; `-count=10` ok |
| 2. `git grep -n -E 'time\.(Since\|Sleep)' -- internal/routine/execute_test.go` | HOLDS, no output |
| 3. Break: process_unix.go:15 kills `cmd.Process.Pid` | HOLDS: `execute_test.go:76: descendant still alive when the run returned: pid 426086` / `--- FAIL ... (32.13s)`; restored with `git checkout -- internal/routine/process_unix.go` |
| 4. `go test -count=1 ./internal/routine/` | HOLDS, ok |
| 4. `git diff main --stat` | HOLDS: only internal/routine/execute_test.go |
| 4. gofmt -l; go vet; GOOS=windows go vet; GOOS=darwin go vet | HOLDS, all empty and exit 0 |
| Importers of routine (go list) | cli, cmd/grid, lib: `go test -count=1 ./internal/routine/ ./internal/cli/ ./cmd/grid/ ./lib/` ok (cmd/grid has no tests) |
| No clock, no Skip, no deleted test, no production edit, no guard | HOLDS |
| RealPlugin tests | not named by a row |

## What changed
The timeout part of TestProcessEnvironmentAndTimeout now runs `sleep 60 & echo $! > $GRID_HOME/child.pid; wait`. The withTimeout seam cancels once child.pid holds a whole line. The runCommand seam calls proctree.Run, then waits for the pid to be gone (signal 0 fails or /proc state Z) before cleanupProcess runs. The 30 s bound only reports "descendant still alive when the run returned". The environment part and the "timed out" assertion are unchanged. Helpers: gtRoutineReadPid, gtRoutineAlive.

## NOT VERIFIED
Nothing needs a live screen. On macOS there is no /proc, so a zombie there counts as alive until reaped (vet only, not run).
