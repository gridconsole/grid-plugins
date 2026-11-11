# C-kr-planguard-off-hint

Plan-guard's off hint names the command that exists: `grid hook level plan-guard off`, with the env switch second.

## Proof table

| Row | Result | Proof |
| --- | --- | --- |
| Base check | HOLDS | On base `git grep -n PlanGuardOffCommand` printed nothing; plan.go:12 held the old hint. |
| 1 Red on the base | HOLDS | With plan.go unchanged, `go test -count=1 -v -run TestKrPlanGuard ./internal/guard/ ./internal/cli/` exit 1: `internal/guard/kr_planguard_hint_test.go:17:5: undefined: PlanGuardOffCommand`, `FAIL ... internal/guard [build failed]`, `internal/cli/kr_planguard_off_test.go:15:49: undefined: guard.PlanGuardOffCommand`, `FAIL ... internal/cli [build failed]`. With only the const added (old `off` text), the guard test runs and fails: `--- FAIL: TestKrPlanGuardOffHintNamesTheCommand` with `warning "plan-guard: A planner writes only inside ... To turn this guard off, start Claude Code with GRID_PLAN_GUARD=off." does not end with " To turn this guard off, run grid hook level plan-guard off (or start Claude Code with GRID_PLAN_GUARD=off)."` |
| 2 Hint text and constant | HOLDS | internal/guard/kr_planguard_hint_test.go:15 (literal hint) and :18-31 (`TestKrPlanGuardOffHintNamesTheCommand`: planner Write to `repo/file.go`, one warning, suffix is the literal; `PlanGuardOffCommand == "grid hook level plan-guard off"`). PASS. |
| 3 The command turns the guard off | HOLDS | kr_planguard_hint_test.go:33-55 (`TestKrPlanGuardOffCommandTurnsTheGuardOff`): one warning before; after `SetHook("plan-guard","off")` on the project and personal stores no deny and no warning. PASS. |
| 4 CLI runs the hint's words | HOLDS | internal/cli/kr_planguard_off_test.go: `hook level plan-guard off` exits 0 with "updated in personal"; `hook levels` exits 0 and prints "plan-guard off". `--- PASS: TestKrPlanGuardOffCommandRuns`. |
| 5 Nothing else moved | HOLDS | `go test -count=1 -v -run 'TestPlanGuard\|TestKrPlanGuard' ./internal/guard/` exit 0, all `--- PASS`, no FAIL (incl. TestPlanGuardFencesWarnByDefaultAndDenyAtBlock, 10 subtests). `go test -count=1 -v -run TestKrPlanGuard ./internal/cli/` exit 0, `--- PASS: TestKrPlanGuardOffCommandRuns`. `go test -count=1 ./internal/golden -run TestGoldenCorpus` exit 0. `go test -count=1 ./internal/hook/` exit 0. |
| 6 lint, cross, walks, diff | HOLDS | `make lint` (go vet ./...) exit 0. `make cross` (darwin/arm64 and windows/amd64 vet and test compile) exit 0. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok. `go test -count=1 ./internal/job/lint/` ok. `gofmt -l internal/guard internal/cli` empty. `git diff main --stat` names plan.go and the two new test files (the lane report under docs/lanes is a separate commit). |
| Importers of changed package | HOLDS | guard is imported by hook; hook by cli and packaging; cli by cmd/grid and lib (from `go list`). `go test -count=1 ./internal/guard/ ./internal/hook/ ./internal/cli/ ./internal/packaging/... ./cmd/grid/ ./lib/` all ok (cmd/grid has no test files). |
| Rules: tests new, none edited or deleted | HOLDS | Only new files; no existing test touched; no test deleted. |
| Rules: no guard, gate, cap or hold added | HOLDS | Only the hint text changed; plan.go:26 (the env read) unchanged. |
| Rules: RealPlugin tests | NOT APPLICABLE | No row names a *RealPlugin* test. |
| Rules: app walk tests | NOT APPLICABLE | Plugin repo; its walks are in row 6. |
| UI rows | NOT APPLICABLE | No UI change. |

## What changed

- internal/guard/plan.go: new exported `const PlanGuardOffCommand = "grid hook level plan-guard off"`; `off` is built as `" To turn this guard off, run " + PlanGuardOffCommand + " (or start Claude Code with GRID_PLAN_GUARD=off)."`. The three deny/warn sites (folder name, curator, planner) carry the new text; the deny and warn texts are otherwise the same.
- internal/guard/kr_planguard_hint_test.go and internal/cli/kr_planguard_off_test.go: new tests tying the hint to the real verb.

## NOT VERIFIED on a live screen

Nothing needs a live screen. The warning as shown inside Claude Code was not looked at; the tests read the hook output.

## Open items

- Existing TestPlanGuardFencesWarnByDefaultAndDenyAtBlock compares against `+ off`, so it follows the constant; the new literal test is what pins the text.
- The goldens in golden/known-differences.txt that carry the old hint are untouched (TestGoldenCorpus does not read the hint).
