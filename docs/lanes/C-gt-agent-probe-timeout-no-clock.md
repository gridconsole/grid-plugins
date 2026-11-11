# C-gt-agent-probe-timeout-no-clock

TestProbeOutputTimeout now cancels the child when its marker file appears, so no deadline decides the result.

## Proof table
| Row | Result |
|---|---|
| 1. `go test -count=1 -run '^TestProbeOutputTimeout$' -v ./internal/agent/` | HOLDS, exit 0: `--- PASS: TestProbeOutputTimeout (0.01s)` / `PASS`. `-count=10`: `ok ... 0.214s`, exit 0 |
| 2. grep `time\.(Since\|Sleep)\|WithTimeout` in runner_test.go | HOLDS: prints only lines 26 (helper), 73, 87, 190 (other tests); none inside TestProbeOutputTimeout |
| 3. Break `return stdout.String(), 0` at runner.go:144 | HOLDS: `--- FAIL: TestProbeOutputTimeout (0.11s)` / `runner_test.go:138: output "", exit 0, context context canceled`; restored with `git checkout -- internal/agent/runner.go` |
| 4. `go test -count=1 ./internal/agent/` | HOLDS, exit 0 (ok 5.2s). `gofmt -l internal/agent` prints nothing. `go vet ./internal/agent/` exit 0 |
| Only runner_test.go changed | HOLDS (`git diff main --stat` after commit lists only that file) |
| Importers tested | `go list` shows only internal/cli imports internal/agent; `go test -count=1 ./internal/cli/` ok (60.9s) |
| Windows / macOS builds | `GOOS=windows go vet ./internal/agent/` and `GOOS=darwin go vet ./internal/agent/` exit 0 |
| New test fails on base | NOT DONE: the old test passes on an idle box; it only failed under load, which I did not reproduce. The break row shows the new test still catches the pinned behaviour |
| No guard/gate/cap on agents | HOLDS: test-only change |

## What changed
internal/agent/runner_test.go, TestProbeOutputTimeout only:
- `context.WithCancel` replaces `WithTimeout`.
- New helper `gtProbeCancelWhenStarted` polls for the marker on a 10 ms ticker and then cancels. A 60 s bound cancels only if the helper never started; the test then fails on "helper did not start".
- Asserts `code != 0`, `errors.Is(ctx.Err(), context.Canceled)`, marker present, ProcessState set, and `Process.Wait` errors.
- Elapsed-time assertion dropped.

## NOT VERIFIED
No live screen involved. The load-induced failure itself was not reproduced.
