# C-if-env-reused-pid-event

Proof table (all run in the lane clone).

| Row / rule | Result |
|---|---|
| 1. `git grep -n 'time.After(50 \* time.Millisecond)' -- internal/env/env_test.go` prints nothing | HOLDS, no output |
| 2. Scratch edit: `matchesProcess` always true in `stopRecord` (internal/ports/process_unix.go:74-80); `go test ./internal/env -run '^TestDownDoesNotSignalReusedPID$' -count=1` | HOLDS, exit 1: `env_test.go:625: down signalled reused PID: INT` (`--- FAIL: TestDownDoesNotSignalReusedPID (0.09s)`). Reverted with `git checkout -- internal/ports/process_unix.go`; `git diff main --stat` names only env_test.go |
| 3. `go test ./internal/env -run '^TestDownDoesNotSignalReusedPID$' -count=20` | HOLDS, exit 0 (`ok ... 1.579s`) |
| 4a. `go test ./internal/env -count=1` | HOLDS, exit 0 |
| 4b. `make lint` | HOLDS, exit 0 (`go vet ./...`) |
| 4c. `git diff main --stat` | HOLDS, only `internal/env/env_test.go` (67 insertions, 3 deletions) |
| Importers of internal/env (`go list` imports): internal/cli | HOLDS, `go test ./internal/cli -count=1` exit 0 (66s) |
| Windows build `GOOS=windows CGO_ENABLED=0 go build ./...` | HOLDS, exit 0 |
| macOS build `GOOS=darwin CGO_ENABLED=0 go build ./...` and `go vet ./internal/env` | HOLDS, exit 0 |
| gofmt `gofmt -l internal/env` | HOLDS, no output |
| New test fails on base, passes on lane | HOLDS: row 2 failure line with the production check removed; row 3 passes with it |
| No existing test edited or deleted; assertEmpty kept | HOLDS; only TestDownDoesNotSignalReusedPID changed, `assertEmpty` still ends it (env_test.go, last line of the test) |
| No guard/gate/cap/hold on agents; no production file | HOLDS, diff is test-only |
| No sleep/elapsed-time proof | HOLDS: the 10 s timer in `ifErNext` only bounds a wait and fails with "timed out waiting for <what>" |
| No personal paths, hosts or emails; commit has no attribution lines | HOLDS |
| *RealPlugin* tests | not applicable, none named by the brief |

## What changed
`TestDownDoesNotSignalReusedPID` (internal/env/env_test.go) now:
- makes a fifo `child.fifo` in `m.Dir` with `syscall.Mkfifo`;
- runs a shell command that opens it once as fd 3, traps INT, TERM and WINCH (one line per trap), writes `ready`, then loops on a backgrounded `sleep` plus `wait`;
- reads the fifo in a goroutine (`ifErReadLines`) and waits for `ready` with a 10 s deadline (`ifErNext`);
- runs `Down` as before, sends the child PID SIGWINCH itself, and reads until `WINCH`;
- fails with `down signalled reused PID: <line>` on an `INT`/`TERM` line or on EOF, and with "child never answered the barrier signal" on timeout;
- keeps the deferred `StopRecord`, the `done` check (now non-blocking) and `assertEmpty`.

New package-level names: `ifErReadLines`, `ifErNext`. New import: `syscall`.

## NOT VERIFIED on a live screen
Nothing needs a screen. Not run: the whole-module test and the cross-repo set (the orchestrator runs them at landing).
