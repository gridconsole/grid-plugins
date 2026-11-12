# C-se-step-who-role: lane proof table

Base: plugin main 2b47f621. Lint.Options is at lint.go:23-26, validate at :125-140 (base), lintOptions at command.go:611-613 (base). No base lint rule checks the role against agents.

Verdict: CLOSED BY C-se2-step-who-fixtures: the fixture home gets the grid-writer agent (jobtest.New writes it), so the four jobcmd tests stay green with no test or golden edited. At C-se: the feature is built and its three new tests pass, but row 4 fails: four existing jobcmd tests go red, and the cause is outside Touches.

| Row / rule | Result |
|---|---|
| 1. TestSeRoleWithoutAgentWarns red on base, green here | HOLDS. Base (lint.go checked out from 2b47f621): `go test -count=1 -run TestSe ./internal/job/lint/` exit 1, `se_role_agent_test.go:43:62: unknown field KnownRole in struct literal of type lint.Options`. Lane: `--- PASS: TestSeRoleWithoutAgentWarns`. Finding list with KnownRole nil, role nobody (the base behaviour): `[]`. With KnownRole: one finding, rule job.role-agent, severity warning, line 6, HasErrors false. |
| 2. TestSeRolesKnown | HOLDS. `--- PASS: TestSeRolesKnown` (builder, Builder, grid-builder, person, Person: no finding). |
| 3. TestSeLintBrainAgent on givenFixture | HOLDS. `--- PASS: TestSeLintBrainAgent`: lint --task T-01 --json: nobody gives exit 0 and one job.role-agent; researcher none; auditor one before and none after `<GridHome>/knowledge/agents/grid-auditor.md` is written. |
| 4. TestSe tests pass; `go test -count=1 ./internal/job/... ./internal/workflow/` exit 0 | CLOSED BY C-se2-step-who-fixtures: the fixture home gets the grid-writer agent. At C-se: the three TestSe tests print `--- PASS:`. `internal/workflow` ok. `internal/job/jobcmd` FAILS in four existing tests, which I did not edit: TestRenderLintShow (command_test.go:68), TestLintJobRecordJSON (lint_json_test.go:65), TestOWOutputWriteFails (output_writers_test.go:66), TestRenderLintErrorExitsOne (rm_plan_approval_exit_test.go:37). Cause: the shared fixture testdata/job/task/plan.md:8 has `role: writer`, no plugin, catalog or brain agent runs `writer`, and these tests expect zero findings. Input: lint of the rendered fixture Job. Output: `job.role-agent line 6 warning: No agent runs role writer ...`. Exit code is still 0 (warning only). |
| 5. git grep sentence | HOLDS. `git grep -n 'job.role-agent\|is the Job.s role' -- skills/plan-jobs/SKILL.md agents/orchestrator.md` prints skills/plan-jobs/SKILL.md:60 and agents/orchestrator.md:25 (one line each). |
| 6. make lint, make cross, diff stat | `make lint` exit 0 and `make cross` (darwin/arm64, windows/amd64) exit 0: HOLD. Diff stat names only the Touches files: HOLDS (git status clean after my experiment, see below). |
| Importers' tests | Importers by go list: lint is imported by jobcmd and cli; jobcmd by cli; root gridplugins (agents/*.md) by job and jobcmd; job by hook, lease, orchestrator, migrate, ports, jobgraph, mcp, cli. Ran `go test -count=1 . ./internal/job/... ./internal/workflow/ ./internal/cli/ ./internal/hook/ ./internal/packaging/... ./internal/orchestrator/ ./internal/lease/ ./internal/jobgraph/ ./internal/mcp/ ./lib/ ./cmd/...`: all ok except jobcmd (row 4). |
| Walk tests (plugin) | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (ok). `internal/job/lint` ok in the runs above. |
| No guard, gate or cap | HOLDS. A warning only; HasErrors and every exit code are unchanged. |
| No attribution lines; no personal paths, hosts or emails | HOLDS. |
| No existing test edited or deleted | HOLDS. |
| New tests only in the new files | HOLDS (se_role_agent_test.go, se_lint_role_test.go). |

## Why row 4 cannot be finished inside Touches

I tried the one-line fix, `role: writer` to `role: builder` in testdata/job/task/plan.md:8. The four jobcmd tests went green, but it cascaded into other existing tests that assert role writer: TestBriefGolden and TestJobWithoutSkillsRendersAsBefore (internal/job, golden testdata/golden/job-brief.golden:6 and the render inputs golden), TestPlanPublicationChecksReadSnapshot and TestPlanPublicationRetriesWithoutResettingJobs (internal/orchestrator). I reverted it (`git checkout`). The fixture is outside Touches, so I did not keep it.

Options for the owner or orchestrator, in my order of preference:
1. Re-brief to allow moving the fixture role from `writer` to `builder` in testdata/job/task/plan.md, testdata/golden/job-brief.golden and the goldens and assertions that name `writer`. The lint behaviour is unchanged.
2. Re-brief so the jobcmd fixture tests lint with a role the plugin knows.
3. Let lint treat `writer` as a known role (a catalog or agent addition). I did not do this; it is a product decision.

## What changed

- internal/job/lint/lint.go: Options.KnownRole; validate adds `job.role-agent` (warning, on the role line) when KnownRole is set and the normalized role is neither `person` nor known.
- internal/job/jobcmd/command.go: lintOptions sets KnownRole to a new `(*world).knownRole` method placed next to it (plugin agents, catalog, then project and home brain agent names, `grid-<role>` or `<role>`). It needs the `gridplugins` and `settings` imports. The method is a small step beyond "lintOptions only", in the same file next to lintOptions; the Render call at :154 is not touched.
- skills/plan-jobs/SKILL.md:60 and agents/orchestrator.md:25: one sentence each.
- New tests: internal/job/lint/se_role_agent_test.go, internal/job/jobcmd/se_lint_role_test.go.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: `make realplugin` (no row names it) and the app-side walk tests (plugin repo only).
