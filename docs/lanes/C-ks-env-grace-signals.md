# C-ks-env-grace-signals

Lane C-ks-env-grace-signals, plugin repo, branch lane-C-ks-env-grace-signals. Keep-the-floor form of the brief (owner question Q2 at its default).

Base: cf5b259f8cb6fdb48e374cd9f112f0e893bf64c3 (the brief names b561251f; the clone's main is later). `git grep -n -E 'time\.Sleep|time\.Since' -- internal/env/env_test.go` on the base printed `:172`, `:248`, `:252`, as the brief says.

Code head: a808ee9e3fa9aeb3f55ed120a6d794f050437e4d (one commit, `internal/env/env_test.go` only). The report commit on top adds only this file.

## What changed

`internal/env/env_test.go`, inside `TestUpHealthTimeoutHonorsStopGrace`, plus two package-level helpers:

- `helper.ready` is now a FIFO made with `syscall.Mkfifo` in `m.Dir` before `m.start` (:238). The helper's `: > helper.ready` blocks until the test opens the FIFO for reading, and the script reaches it only after both traps are set. `ksEgAwaitReady` (:286) opens the FIFO in a goroutine and selects on the open against `ksEgHangCtx` (:269), the hang bound copied from the shape of `iqWkHangCtx` (app `iq_stop_wait_test.go:27-40`). The 5 s stat loop with `time.Sleep(10ms)` is gone.
- The helper records instead of ignoring: `trap 'echo INT >> signals.log' INT; trap 'echo TERM >> signals.log' TERM; : > helper.ready; while :; do sleep 1; done` (:225). After `Up` returns `context.DeadlineExceeded` the test reads `signals.log` and expects exactly `INT\nTERM\n` (:259-260), then `assertEmpty` as before.
- `started` (:247), `stopElapsed` (:249) and the `stopElapsed < 2*m.Grace` check with its comment (:254-255) are unchanged. The L97R floor stays.
- No product file changed. `TestDevServerLifecycle` and its floor (:172) are untouched.

## Proof table

| Row / rule | Result | Evidence |
|---|---|---|
| 1. no `time.Sleep` in env_test.go | HOLDS | `git grep -n 'time\.Sleep' -- internal/env/env_test.go` printed nothing, exit 1 (base printed :248) |
| 1. L97R pin stays | HOLDS | `git grep -n 'stopElapsed < 2\*m.Grace' -- internal/env/env_test.go` printed one line: `internal/env/env_test.go:254` |
| 2. PASS, no SKIP | HOLDS | `go test -count=1 -v -run 'TestUpHealthTimeoutHonorsStopGrace' ./internal/env/` exit 0: `--- PASS: TestUpHealthTimeoutHonorsStopGrace (0.53s)`, `ok .../internal/env 0.655s`, no `--- SKIP` |
| 3. product mutation | HOLDS | `ports.StopRecord(r, m.grace())` changed to `ports.StopRecord(r, 0)` at env.go:83; same command exit 1: `env_test.go:255: stop phase took 2.848862ms, want at least 200ms (two grace periods)` and `env_test.go:260: signals.log = "" (read error open .../signals.log: no such file or directory), want "INT\nTERM\n"` and `--- FAIL: TestUpHealthTimeoutHonorsStopGrace (0.12s)`. Restored with `git checkout -- internal/env/env.go`; `git status --short` then showed only ` M internal/env/env_test.go`. `git diff main --stat` before the report commit: `internal/env/env_test.go | 77 +++++---, 1 file changed, 63 insertions(+), 14 deletions(-)` |
| 4. ladder mutation | HOLDS | TERM trap changed to `trap '' TERM`; same command exit 1: `env_test.go:260: signals.log = "INT\n" (read error <nil>), want "INT\nTERM\n"` and `--- FAIL: TestUpHealthTimeoutHonorsStopGrace (0.33s)`. Restored; the final diff holds the real trap (:225) |
| 5. `go test -count=3 ./internal/env/` | HOLDS | exit 0: `ok .../internal/env 24.764s` |
| 5. extra flake check | HOLDS | `go test -count=40 -run 'TestUpHealthTimeoutHonorsStopGrace' ./internal/env/` exit 0: `ok .../internal/env 14.537s` |
| 5. `git merge main` before the last run | HOLDS | `Already up to date.` |
| 5. walk: cli | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` exit 0: `ok .../internal/cli 0.162s` |
| 5. walk: job lint | HOLDS | `go test -count=1 ./internal/job/lint/` exit 0: `ok .../internal/job/lint 0.235s` |
| 6. `make lint` | HOLDS | exit 0 (`go vet ./...`, no output) |
| 6. `make cross` | HOLDS | exit 0; darwin/arm64 and windows/amd64 vet and test-compile both `ok` for `internal/env` and every other package |
| Importers' tests | HOLDS | importers of `internal/env` from `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...`: `internal/cli` (direct); `cmd/grid` and `lib` reach it through `internal/cli`. `go test -count=1 ./internal/env/ ./internal/cli/ ./cmd/grid/ ./lib/` exit 0: env 9.617s, cli 63.646s, cmd/grid no test files, lib 18.140s |
| App-side walk tests | N/A | plugin repo; the runbook's plugin walk tests are the two rows above |
| `*RealPlugin*` tests | N/A | no row names one; nothing skipped is claimed as passing |
| New test fails on base | Mutation form | the brief is test-only, so there is no product change on which the new checks could fail at base; rows 3 and 4 are the failing runs that show the new FIFO/ladder checks and the kept floor catch a broken product or helper |
| No test deleted, no other test edited | HOLDS | only `TestUpHealthTimeoutHonorsStopGrace` is edited, as the brief names; no test removed |
| No guard, gate, cap or hold on agents | HOLDS | test file only |
| No sleep, poll loop or new elapsed-time assertion | HOLDS | the only wait is a select of the FIFO open against the hang bound; the one elapsed check is the L97R floor, unchanged |
| Public repo hygiene | HOLDS | no personal paths, hosts or emails added |
| Commits | HOLDS | one-sentence messages, no attribution lines |

## Notes for the reviewer

- The `signals.log` check needs the shell to run each trap within one Grace (100 ms) of its signal. The FIFO handshake guarantees the traps are installed first, so the residual risk is only scheduling. The comment at env_test.go:181-183 says an earlier signal-log assertion in `TestDevServerLifecycle` was dropped for that reason; that one had no handshake. 40 repeated runs, 3 package runs and the importer run all passed here.
- Failure path: if the helper never opens the FIFO, `ksEgAwaitReady` fails with its own message at the hang bound, and its cleanup opens the FIFO non-blocking for writing so the reader goroutine does not leak.

## Diff scope

`git diff main --stat` on the code head a808ee9e names only `internal/env/env_test.go` (row 3). The lane head adds one more file, this lane report `docs/lanes/C-ks-env-grace-signals.md`, which the runbook requires; no product file is in the diff (`env.go` is clean after the row 3 mutation).

## NOT VERIFIED

- The test has not run on macOS; there `/bin/sh` is bash in POSIX mode, and only the compile (`make cross`) was checked. The traps use the same POSIX syntax.
- No live screen is involved; there is no UI row.

Verdict: DONE
