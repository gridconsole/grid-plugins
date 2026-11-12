# C-sb-skills-block-brain

Base: 7a4bc0c2d6c9a1ef1c7bec89e38f9bca07767c29 (lane branch before this work). Base check: `skillsBlock(names []string)` at internal/job/render.go:410 knew only `gridplugins.SkillFor`; the Render call is internal/job/jobcmd/command.go:166; RenderOptions at render.go:17-26 had no brain or settings field. Nothing to stop on.

Verdict: NOT DONE (one row, see row 7). The feature works; the wiring the brief names trips an existing test that the Touches list does not let me fix.

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1 | TestSbBrainSkillListed red on base, green here | HOLDS. Base: the test file does not compile (`o.BrainSkill undefined (type *job.RenderOptions has no field or method BrainSkill)`, internal/job/sb_skills_block_brain_test.go:32), and the wiring test on base renders `- grid:house-style (not in this plugin): load it with the Skill tool before you start`. Here: `go test -count=1 -v -run TestSb ./internal/job/` prints `--- PASS: TestSbBrainSkillListed`, exit 0. Line asserted: `- house-style (brain skill): read <path> before you start. Our house style`; no `(not in this plugin)`, no SKILL.md body. |
| 2 | TestSbSkillOffLeftOut | HOLDS. `--- PASS: TestSbSkillOffLeftOut`. plan-jobs line unchanged, no research line, no `### Skills` when both off, unknown name keeps render.go's `(not in this plugin)` line. |
| 3 | TestSbSkillForOrder | HOLDS. `--- PASS: TestSbSkillForOrder` (internal/brain). Project beats home; `../x`, `a/b`, `a.b`, `a\b`, empty answer false. |
| 4 | TestSbRenderReadsSettingsAndBrain on givenFixture | HOLDS. `--- PASS: TestSbRenderReadsSettingsAndBrain` (internal/job/jobcmd): writes projects/demo/settings.json `{"skills":{"research":false}}` and a home knowledge/skills/house-style/SKILL.md, runs `render T-01 1 --json`, finds the brain line and no research line in the saved brief, body not pasted. Extra: TestSbRenderReadsPersonalSettings does the same with the personal settings.json. On base the same run printed the research line and `grid:house-style (not in this plugin)` (red, base run pasted in the work log). |
| 5 | `go test -count=1 -v -run TestSb ./internal/brain/ ./internal/job/...` and `go test -count=1 ./internal/job/... ./internal/brain/` | HOLDS, exit 0. Existing render_job_skills_test.go and TestSa tests unchanged and green. |
| 6 | `make lint`, `make cross`, diff names only Touches | HOLDS. `make lint` exit 0; `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile). `git diff 7a4bc0c2 --stat`: sb_skill_for.go, sb_skill_for_test.go, jobcmd/command.go, jobcmd/sb_render_skills_test.go, render.go, sb_skills_block_brain_test.go; all in Touches. |
| 7 | Tests of every package that imports brain or job (go list -deps list below) | NOT DONE for one test: `go test -count=1 ./internal/cli -run TestSettingsStoreChokePoint` exits 1: `internal/cli/settings_choke_test.go:61: ../../internal/job/jobcmd/command.go: settings.Store bypasses settingsStore resolver adapter` (twice, one per `settings.Store{...}` the brief asks for at command.go:166). All other 32 packages pass. |
| 8 | Walk tests | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (ok). `go test -count=1 ./internal/job/lint/` exit 0 (ok). The app-side walks are not run here (plugin repo). The one red walk-like test is TestSettingsStoreChokePoint, row 7. |
| 9 | No guard, gate, cap or hold on agents | HOLDS. Nothing blocks an agent: an off skill is left out of a list, an unknown skill keeps its line. |
| 10 | New tests fail on base, pass here | HOLDS, see rows 1 and 4. No existing test edited or deleted. |
| 11 | Windows and macOS builds | HOLDS, row 6. |

Packages checked (importers of brain, job or jobcmd, from `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./...`): cmd/grid, approvefix, brain, braincmd, cli, env, feed, guard, hook, job, job/frontmatter, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, packaging, packaging/cmd, people, ports, projectenv, question, routine, signal, task, updates, usage, workflow, lib. All ok except cli's TestSettingsStoreChokePoint (the rest of cli: the TestHs/TestIl walks above pass; the full cli run before the fix attempt failed only on that test).

## What changed

- internal/brain/sb_skill_for.go: `Store.SkillFor(name) (md, path, ok)` reads knowledge/skills/<name>/SKILL.md at project then home, through safefile (no symlinks, 1 MiB cap, as ReadConfig does); `path` is the absolute file path; names with `/`, `\` or `.` answer false.
- internal/job/render.go: `RenderOptions.BrainSkill` and `.SkillOff` (nil = none / nothing off); `skillsBlock(names, brainSkill, off)`: off names are left out; plugin skill keeps its line; else a brain skill prints `- <name> (brain skill): read <path> before you start. <description>`; else the old `(not in this plugin)` line. The body is never pasted.
- internal/job/jobcmd/command.go (render case): reads `settings.Store{Home, Project: w.project}.Effective()`; an error starting `unknown project` with a project set falls back to `settings.Store{Home}`; any other error fails the render; passes `BrainSkill: w.store.SkillFor` and `SkillOff` (present and false).

## NOT DONE: TestSettingsStoreChokePoint (internal/cli/settings_choke_test.go)

The test walks every production file and allows `settings.Store` only inside cli's `settingsStore` adapter, which resolves the caller (guests, unregistered folders). The brief tells jobcmd to build `settings.Store{...}.Effective()` itself, so the test goes red. I did not edit that test, change internal/settings or cli (outside Touches), or hide the reference behind an alias: the test exists to keep every settings read caller-aware. Needs an owner or orchestrator choice, either:
1. add a read-only reader in internal/settings (like `ReadHookLevels(home, project)`, store.go) for skill state, list it in the test's `exports` map, and have jobcmd call it; or
2. allow jobcmd/command.go in the test, with the reason written next to it.
Either is a small follow-up lane; the Render side (render.go, SkillFor) needs no change for it.

## NOT VERIFIED on a live screen

Nothing here is a screen. Not run: a real worker rendering a brief for a claude agent that then loads the brain SKILL.md; the `unknown project` fallback branch (the fixture project has a record, so no test reaches it).
