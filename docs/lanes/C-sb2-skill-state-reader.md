# C-sb2-skill-state-reader

Base: e079ebb6 (lane-C-sb-skills-block-brain head). Finishes C-sb-skills-block-brain, whose row 7 was NOT DONE on TestSettingsStoreChokePoint.

Verdict: DONE.

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1 | `git merge origin/main`, name each conflict | HOLDS. Two files conflicted, both resolved keeping both sides: internal/job/render.go (RenderOptions: kept `BrainSkill`, `SkillOff` and main's `Answers` plus the `Answer` type) and internal/job/jobcmd/command.go (render case: kept main's `waitedAnswers`/`Answers` and the lane's `BrainSkill`/`SkillOff`). |
| 2 | One exported read-only reader in a new internal/settings file | HOLDS. internal/settings/skill_state.go:16 `ReadSkillsOff(home, projectName string) ([]string, error)`: personal settings.json, then the project's, laid over the defaults (all catalog skills on); returns the names that are false, sorted. Writes nothing, takes no lock (uses `readDocument`, as `Effective` does). A project name or id with no record reads the personal level (the fallback C-sb row 3 asked for); any other error is returned. It never names the `Store` type, so the choke test needs no change beyond its exports list. |
| 3 | The job command calls it, not settings.Store | HOLDS. internal/job/jobcmd/command.go (render case) calls `settings.ReadSkillsOff(e.GridHome, w.project)`; `grep -n 'settings\.Store' internal/job/jobcmd/command.go` is empty. |
| 4 | Name added to the exports list, nothing else in settings_choke_test.go | HOLDS. internal/cli/settings_choke_test.go:102, one entry `"ReadSkillsOff": "skill_state.go"`; `git diff origin/main --stat` shows that file at 1 line changed. |
| 5 | `go test ./internal/cli/ -run 'TestSettingsStoreChokePoint\|TestHsIpWalksEveryCommand' -count=1 -v` | HOLDS, exit 0: `--- PASS: TestHsIpWalksEveryCommand (0.00s)` and `--- PASS: TestSettingsStoreChokePoint (0.40s)`, `PASS`, `ok .../internal/cli 0.548s`. Red on the base head: `settings_choke_test.go:61: ../../internal/job/jobcmd/command.go: settings.Store bypasses settingsStore resolver adapter` (twice), `FAIL`. |
| 6 | Every row of briefs/C-sb-skills-block-brain.md | HOLDS. `go test -count=1 -v -run TestSb ./internal/brain/ ./internal/job/...`: `--- PASS: TestSbSkillForOrder`, `TestSbBrainSkillListed`, `TestSbSkillOffLeftOut`, `TestSbRenderReadsSettingsAndBrain`, `TestSbRenderReadsPersonalSettings`. `make lint` exit 0, `make cross` exit 0 (row 6). Row 7 of docs/lanes/C-sb-skills-block-brain.md (the choke test) is now green, row 5 above. Diff against base names only the Touches files plus skill_state.go, the exports line and this doc. |
| 7 | `go test ./internal/settings/ ./internal/cli/ -count=1` | HOLDS: `ok internal/settings 7.6s`, `ok internal/cli 205.2s` (run in one command with the importers below, exit 0). |
| 8 | `make lint`, `make cross` | HOLDS: `make lint && echo LINT_EXIT_0` printed `go vet ./...` then `LINT_EXIT_0`; `make cross` ran darwin/arm64 and windows/amd64 vet and test compile, every package ok, exit 0. |
| 9 | Tests of changed packages and importers | HOLDS, all ok (exit 0): settings, brain, braincmd, cli, job, job/frontmatter, job/jobcmd, job/lint, account, doctor, guard, hook, jobgraph, lease, migrate, orchestrator, people, ports, projectenv, routine, task, workflow; updates and job/jobtest have no tests. List from `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...` filtered on importers of settings, brain, job, jobcmd. |
| 10 | Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok (inside row 9). App-side walks do not apply (plugin repo). |
| 11 | No guard, gate, cap or hold on agents | HOLDS. A read of two settings files; an off skill is left out of a list. |
| 12 | New tests fail on base, pass here; no existing test edited or deleted | HOLDS for the choke test (red on base, green here; its only edit is the exports entry the brief allows). The reader is exercised end to end by TestSbRenderReadsSettingsAndBrain (project file) and TestSbRenderReadsPersonalSettings (personal file). No test deleted. |
| 13 | Windows and macOS builds | HOLDS, row 8. |

## What changed

- internal/settings/skill_state.go (new): `ReadSkillsOff`.
- internal/job/jobcmd/command.go: the render case asks `settings.ReadSkillsOff` instead of building `settings.Store`; the merge kept main's `Answers`.
- internal/job/render.go: merge only, `Answers` and `BrainSkill`/`SkillOff` side by side.
- internal/cli/settings_choke_test.go: one exports entry.

## Notes

- I first wrote internal/settings/skill_state_test.go (personal, project name, project id, no-record fallback, broken settings.json errors, empty home errors, nothing written; all passed) and removed it before committing, because the brief names one new file in internal/settings. The no-record fallback therefore has no committed test: the jobcmd fixture's project has a record. A follow-up lane may add that test.

## NOT VERIFIED on a live screen

Nothing here is a screen. Not run: a real worker rendering a brief and an agent loading the brain SKILL.md.
