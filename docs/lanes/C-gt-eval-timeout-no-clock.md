# C-gt-eval-timeout-no-clock

| Row | Result |
|---|---|
| 1. TestFailuresAreSaved -v | HOLDS, exit 0: `--- PASS: TestFailuresAreSaved (0.17s)` / `--- PASS: TestFailuresAreSaved/timeout (0.14s)`; `-count=5`: `ok ... 1.009s` |
| 2. no time.Since/Sleep in the test | HOLDS: `git grep -n -E 'time\.(Since\|Sleep)' -- internal/eval/runner_test.go` prints nothing (exit 1 = no match) |
| 3. runner.go:181 -> context.WithCancel | HOLDS: `--- FAIL: TestFailuresAreSaved/timeout (20.04s)` at runner_test.go:118 (status "failed", the row fails at the status check before the deadline check); restored with `git checkout -- internal/eval/runner.go` |
| 4. no WaitDelay, Cancel returns nil | HOLDS: `--- FAIL: TestFailuresAreSaved/timeout (20.04s)` at runner_test.go:124 "timeout did not stop the command" (exit 0 case); restored with git checkout; `git diff main --stat` after: only runner_test.go, 1 insertion, 2 deletions |
| 5. package test, diff, gofmt, vet | HOLDS: `go test -count=1 ./internal/eval/` ok (exit 0); stat names only internal/eval/runner_test.go; `gofmt -l internal/eval` prints nothing; `go vet ./internal/eval/` exit 0 |
| Importers tested | HOLDS: go list importers are internal/cli, internal/evalcmd, internal/land; `go test -count=1` on all three plus eval: ok |
| Windows/macOS builds | HOLDS: `GOOS=windows go vet ./internal/eval/` and `GOOS=darwin go vet ./internal/eval/` exit 0 |
| No test deleted/renamed/skipped, no guard added | HOLDS |
| RealPlugin tests | not applicable |

## What changed
internal/eval/runner_test.go, TestFailuresAreSaved: `time.Since(start) > 2*time.Second` became `r.AgentResult.ExitCode == 0`; the "deadline" check and failure text are kept. The now-unused `start := time.Now()` line was removed (it would not compile otherwise).

## NOT VERIFIED
Nothing needs a live screen. Lint beyond gofmt and go vet (a separate linter) was not run.

DONE
