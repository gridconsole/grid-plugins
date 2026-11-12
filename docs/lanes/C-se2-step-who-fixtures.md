# C-se2-step-who-fixtures: the role-agent warning fits the jobcmd fixture

Stacked on C-se-step-who-role (its report: docs/lanes/C-se-step-who-role.md). Base: 2b47f621.

## Decision

1. The four red jobcmd tests (TestRenderLintShow, TestLintJobRecordJSON, TestOWOutputWriteFails, TestRenderLintErrorExitsOne) fail on the warning's presence in the findings, not on HasErrors: every one still exits 0 and `ok: true`, and each asserts an empty finding list. The code is right (a warning, exit unchanged); the fixture Job has role `writer`, which no plugin, catalog or brain agent runs.
2. Changing `testdata/job/task/plan.md` to a real role does not work without editing existing tests. I tried `builder`: jobcmd goes green, but TestBriefGolden and TestJobWithoutSkillsRendersAsBefore (golden lines `role: "writer"`) and TestPlanPublicationChecksReadSnapshot and TestPlanPublicationRetriesWithoutResettingJobs go red. The two orchestrator tests have `writer` in their code (plan_test.go:76,79 and plan_approval_test.go:16,20 swap `role: writer` for `role: builder` and assert `Role == "writer"`), so no regenerated golden can fix them, and the rules forbid editing them. I reverted the fixture.
3. Instead the fixture home gets the agent: `jobtest.New` writes `<GridHome>/knowledge/agents/grid-writer.md`, so `writer` is a role an agent runs in that home. No test, golden or plan fixture changed; no golden line changed.
4. Review note applied: `knownRole` skips `ConfigNames("project", ...)` when `w.project` is empty (command.go, `knownRole`). The old code already survived that case, because the brain's error took the `continue`. The skip makes it explicit and spares the call. TestSeKnownRoleNoProject covers it; it passes with or without the skip (I checked with the skip disabled), so it guards the behaviour, not the line.

## Files changed (this lane, over C-se's Touches)

- internal/job/jobtest/jobtest.go: the fixture home writes grid-writer (a test-fixture helper package; the fixture the warning changes).
- internal/job/jobcmd/command.go: the empty-project skip in knownRole.
- internal/job/jobcmd/se_lint_role_test.go: TestSeKnownRoleNoProject added.
- No golden, plan.md or existing test file changed.

## Proof table

| Row / rule | Result |
|---|---|
| `go test -count=1 ./internal/job/... ./internal/orchestrator/...` | HOLDS, exit 0: job ok, frontmatter ok, jobcmd ok, lint ok, orchestrator ok (127s) |
| C-se 1 TestSeRoleWithoutAgentWarns | HOLDS: `go test -count=1 -v -run TestSe ./internal/job/...` prints `--- PASS: TestSeRoleWithoutAgentWarns`. Base run (from C-se's report): compile error `unknown field KnownRole`, exit 1; base finding list for role nobody `[]` |
| C-se 2 TestSeRolesKnown | HOLDS: `--- PASS: TestSeRolesKnown` |
| C-se 3 TestSeLintBrainAgent | HOLDS: `--- PASS: TestSeLintBrainAgent` (nobody: exit 0 and one finding; researcher none; auditor one, then none after grid-auditor.md) |
| C-se 4 `go test ./internal/job/... ./internal/workflow/` | HOLDS, exit 0; skill_example_test.go and lint_test.go unchanged |
| C-se 5 git grep | HOLDS: agents/orchestrator.md:25 and skills/plan-jobs/SKILL.md:60, one line each |
| C-se 6 `make lint` | HOLDS, exit 0 (`go vet ./...`) |
| C-se 6 `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| C-se 6 diff names only Touches plus named fixtures | HOLDS: `git diff 2b47f621 --stat` lists agents/orchestrator.md, skills/plan-jobs/SKILL.md, command.go, lint.go, the two se test files, jobtest.go and the two lane reports. (`git diff main` is not used: main has moved and shows unrelated deletions.) |
| Importers of the changed packages | HOLDS, exit 0: `go test -count=1 ./contract/formats ./internal/cli ./internal/job/... ./internal/plan ./internal/orchestrator/... ./internal/workflow ./cmd/... .` (the test importers of jobtest from go list: contract/formats, internal/cli, internal/job, internal/job/jobcmd, internal/plan) |
| Walk tests | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| No guard, gate, cap or hold on agents | HOLDS: a warning only; exit codes unchanged |
| Existing tests edited or deleted | HOLDS: none |
| New test fails on base, passes here | TestSeKnownRoleNoProject does not compile on base (no `knownRole`); the C-se tests are shown in their rows above |
| RealPlugin tests | Not applicable: no row names one |

## Not verified on a live screen

Nothing here is UI. When C-sa-brain-agent-render lands and renders a brain agent into the brief, the fixture's grid-writer note may show in the job-brief golden; that lane rebases.
