# A-darwin-case-alias-tests

Proof table. Head: see the report. Touches: internal/guard/case_test.go and internal/hook/tooluse_test.go only.

| Row | Status | Proof |
|---|---|---|
| 1. guard test sets only-writer, plan-guard, host-guard to block, with the v-4 comment | HOLDS (compiled, internal/guard/case_test.go:73-78) | `GOOS=darwin go test -c -o /dev/null ./internal/guard/` exit 0 |
| 2. hook test sets only-writer and plan-guard to block on its grid folder | HOLDS (compiled, internal/hook/tooluse_test.go:174-179) | `GOOS=darwin go test -c -o /dev/null ./internal/hook/` exit 0 |
| 3. held-task row keeps its place, compares alias cwd with exact cwd `f.task` | HOLDS (compiled, internal/guard/case_test.go:94 and helper :126-137) | b10002db "Remove the plan hold ..." removed the hold the row pinned; 62e8f7fc dropped its tests. No other row changed. |
| 4a. `go test ./internal/guard/ ./internal/hook/ -count=1` | HOLDS | exit 0: `ok .../internal/guard 1.024s`, `ok .../internal/hook 2.901s` |
| 4b. the two functions still skip on Linux | HOLDS | `-run CaseAliases -v`: `--- SKIP: TestCaseAliasesAreJudgedLikeTheirTargets (0.00s)` and `--- SKIP: TestCaseAliasesOfTheGridHomeAreDenied (0.00s)` |
| 4c. `go vet ./internal/guard/ ./internal/hook/` | HOLDS | exit 0 |
| 4d. `make lint` | HOLDS | exit 0 (runs `go vet ./...`); `gofmt -l internal/guard internal/hook` prints nothing |
| 4e. darwin test-compile of both packages | HOLDS | `GOOS=darwin go test -c -o /dev/null` for each: exit 0 |
| 5. windows test-compile of both packages | HOLDS | `GOOS=windows go test -c -o /dev/null` for each: exit 0 |
| Packages importing the changed ones (go list) | HOLDS | `internal/cli`, `internal/guard`, `internal/hook`, `internal/packaging`. `go test ./internal/cli/ ./internal/packaging/ -count=1` exit 0 (36.7s, 4.0s) |
| New tests fail on base, pass on the lane | NOT DONE on Linux | No new test: the two existing functions are edited in place, as the brief says, and they skip here. Base failures are those quoted in the brief (macOS: 8 guard rows `"", want deny`; 2 hook rows warn not deny). |
| No existing test edited to pass, none deleted | HOLDS | Only the two named functions changed; the only row touched is the held-task row (brief row 3). |
| No guard, gate, cap or hold added; no non-test file changed | HOLDS | `git diff --stat origin/main..` lists the two test files and this doc |
| No attribution lines, no personal paths | HOLDS | commit message is one sentence, none added |

## What changed

- case_test.go: `TestCaseAliasesAreJudgedLikeTheirTargets` sets the three guards to block before the table. The held-task row now uses `sameAsExactCwd(f.task, alias(f.task, "projects"))`: it answers "deny" when the alias cwd and the exact cwd give the same EvaluatePlan behaviour, otherwise it names both verdicts. Other rows are untouched.
- tooluse_test.go: `TestCaseAliasesOfTheGridHomeAreDenied` sets only-writer and plan-guard to block on its grid folder before the loop.

## macOS host run: for the orchestrator

`go test ./internal/guard/ ./internal/hook/ -run 'CaseAliases|RealNative' -count=1 -v` (expected: 0 fail).

## NOT VERIFIED

Everything that executes the two functions: they run only on a case-folding volume. The sandbox proves compilation for darwin and windows and the Linux skip, not the assertions. The held-task comparison (both cwds give "" per the planner's probe) is expected to pass on macOS but was not run there.
