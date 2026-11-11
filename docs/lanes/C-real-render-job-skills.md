# C-real-render-job-skills: a Job's skills are named in its brief as grid:<name>

Base: main cd84f424ad766a589bcc3c5ff1477ae4060d05ff (no `.Skills` read anywhere in internal/job; no render golden covers a Job with skills, `sample` in render_test.go has none).

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. A Job with `skills: ["plan-jobs"]` renders `grid:plan-jobs`; fails on main | HOLDS | With `internal/job/render.go` checked out at cd84f424 (`git checkout cd84f424 -- internal/job/render.go`), `go test ./internal/job/ ./internal/cli/ -run "JobSkill\|PluginSkill\|WithoutSkills\|TestTaskNewFirstJobBriefNamesItsSkill" -count=1` exit 1: `render_job_skills_test.go:53: the brief never names grid:plan-jobs:` (the brief shows only the metadata line `skills: ["plan-jobs"]`). Restored with `git checkout HEAD -- internal/job/render.go`; same command on the lane exits 0 (`TestJobSkillsAreNamedAsGridSkills` PASS). |
| 2. The plan-jobs line carries the SKILL.md front-matter description, read from the embed | HOLDS | internal/job/render_job_skills_test.go `TestJobSkillLineCarriesTheSkillDescription` reads it with `gridplugins.SkillFor("plan-jobs")` (embed.go `//go:embed skills/*/SKILL.md`), not a copy. On base: `render_job_skills_test.go:81: want one line ending in the front matter description "How the orchestrator writes ...", got []`. `TestEveryPluginSkillRendersItsDescription` does the same for all nine skills in skills/. Lane: exit 0. |
| 3. `["grid:plan-jobs","no-such-skill"]`: `grid:plan-jobs` once, `grid:no-such-skill (not in this plugin)`, no error | HOLDS | `TestJobSkillsNameMatchesAreNotDoubledAndMissingOnesAreMarked` (counts in the body after `## Intent`, because the front-matter metadata echoes the raw `skills` field as written, which is unchanged); also `plan-jobs` and `grid:plan-jobs` together list once. Render error is checked by `renderSkills` (t.Fatal). Lane: exit 0. |
| 4. `skills: []` and no skills key render byte-identical to main | HOLDS | Fixtures captured from the base render before any change: internal/job/testdata/render_job_skills/no-skills-key.md (`skills: null`) and empty-skills.md (`skills: []`; the metadata prints the two differently, so each has its own fixture). `TestJobWithoutSkillsRendersAsBefore` compares byte for byte and asserts no `### Skills`. Lane: exit 0. |
| 5. First Job `grid task new` posts renders with `### Skills` naming grid:plan-jobs | HOLDS | internal/cli/render_job_skills_first_job_test.go `TestTaskNewFirstJobBriefNamesItsSkill`: `newTaskHarness` + `newTask` (temp GRID_HOME, `grid task new`), then `grid job render <task> J-1` and reads jobs/J-1/brief.md; no claude. On base: `render_job_skills_first_job_test.go:25: 0 Skills sections in the first Job's brief`. Lane: exit 0. |
| 6. `go test ./internal/job/... ./internal/cli/ -count=1` exit 0 with render_role_test.go unchanged; `make lint` exit 0 | HOLDS | Packages changed: root `gridplugins` (embed.go) and internal/job. Importers (`go list` of Imports): root ← internal/job; internal/job ← internal/cli, job/jobcmd, job/lint, jobgraph, orchestrator, lease, hook, ports; hook ← packaging; cli ← cmd/grid. `go test . ./internal/job/... ./internal/cli/ ./internal/jobgraph/ ./internal/orchestrator/ ./internal/lease/ ./internal/hook/ ./internal/ports/ ./internal/packaging/... ./cmd/grid/ -count=1` exit 0, every package ok. `git diff cd84f424 --stat -- internal/job/render_role_test.go` is empty. `make lint` (go vet ./...) exit 0. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0; `gofmt -l .` prints nothing. |
| New tests fail on base, pass on lane | HOLDS | Rows 1, 2, 3, 5 fail on base as pasted above; row 4 is a no-change guard and passes on both. |
| No existing test edited or deleted | HOLDS | Only new files added; render_role_test.go and every other test file are untouched. |
| No guard, gate, cap or block on agents | HOLDS | The block is text in the brief; an unknown skill is listed and marked, never refused. |
| Touches only the allowed files | HOLDS | embed.go (embed + `SkillFor`), internal/job/render.go (`skillsBlock`, `skillDescription`, one call after `### Instructions`), the two new test files, the fixtures, this report. Not the app, agents/*.md or internal/settings/catalog.go. |
| No personal paths, hosts or emails in the diff | HOLDS | Nothing of the kind was added. |
| Commits: one sentence, no attribution lines | HOLDS | One commit: "Name a Job's skills as grid:<name> in its brief so the agent loads them." |

## What changed

- embed.go: `//go:embed skills/*/SKILL.md` and `SkillFor(name)`, the same shape as `AgentFor` (plain names only, no path characters).
- internal/job/render.go: a `### Skills` block straight after `### Instructions` (also for roles that have no agent file, where it follows the title). One line per skill: `- grid:<name>: load it with the Skill tool before you start. <description>`. A name the plugin lacks becomes `- grid:<name> (not in this plugin): load it with the Skill tool before you start`. A leading `grid:` is stripped before the line is built, so it is never doubled; repeated names list once. No skills gives no block, so the output is the same as before.
- Tests and fixtures as in the table.

Notes: the front-matter metadata at the top of the brief still echoes `skills` as written (`skills: ["plan-jobs"]`); that is unchanged and is why the "once" check counts the body only. The description line is the whole SKILL.md description, which for plan-jobs is about 300 characters.

## NOT VERIFIED on a live screen

- That a real claude run with the grid plugin loaded calls the `Skill` tool with `grid:plan-jobs` after reading this line (needs a live run; the brief text is proven, the agent behaviour is not).
- That the plugin loads in the worker's claude (I-worker-plugin-dir; not part of this lane).
