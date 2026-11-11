# C-lt-hook-log-slow-unit proof table

Head: d08d6ca1 (test commit; this report is committed after it). One new file: internal/hook/lt_log_slow_test.go. No production change.

| Row | Result |
|---|---|
| 1 Fast branch, PreToolUse and PostToolUse, start -slowHook/2, buf empty | HOLDS. `go test -count=1 -run TestLtLogSlow -v ./internal/hook/` exit 0; `--- PASS: TestLtLogSlowFastLogsNothing` (lt_log_slow_test.go:17) |
| 2 Slow branch, 3 s back-dated, one line matching the regexp, event equal to input, ms >= 3000 | HOLDS. `--- PASS: TestLtLogSlowSlowWritesOneLine` (:30) |
| 3 Name normalisation "pretooluse" -> `PreToolUse`; nil Stderr no panic | HOLDS. `--- PASS: TestLtLogSlowNormalisesName` (:49). The nil case asserts no panic only; it has no buffer to check. |
| 4 Mutant `if d <= 0 {` at hook.go:70 | HOLDS. Failing lines: `lt_log_slow_test.go:23: PreToolUse: fast call wrote "grid hook: hook PreToolUse slow: 1000 ms\n", want nothing`, same for PostToolUse, `--- FAIL: TestLtLogSlowFastLogsNothing`, exit 1. Reverted with `git checkout -- internal/hook/hook.go`; `git diff main --stat` names only internal/hook/lt_log_slow_test.go. |
| 5 All three `--- PASS`, no `--- FAIL` | HOLDS, exit 0. `make lint` exit 0 (go vet). `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| Importers of internal/hook (go list): internal/cli, internal/packaging (cmd, lib, cmd/grid sit above) | HOLDS. `go test -count=1 ./internal/hook/ ./internal/cli/ ./internal/packaging/ ./internal/packaging/cmd/ ./lib/ ./cmd/grid/` all ok, exit 0 |
| Walk tests (plugin) | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| New tests fail on base | HOLDS in the form the brief sets: there is no production change, so the mutant (row 4) is the failing run. |
| RealPlugin tests | Not applicable; no test of this lane is named RealPlugin. |
| No guard, gate or cap added | HOLDS. Test file only. |
| TestKr2FastHookLogsNoSlowLine unchanged | HOLDS. `git diff main --stat` shows no other file. |

What changed: new test file, three tests, helpers `ltLogSlowLine` and `ltLogSlowEvents`. Nothing sleeps, and no assertion is on test duration.

NOT VERIFIED on a live screen: none; no UI.

Verdict: DONE
