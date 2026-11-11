# C-eo-plugin-gate-words

Base: bb3c1340 (main; d528178a is an ancestor).

| Row | Result |
|---|---|
| Base check: `grid question ask` exists | HOLDS: skills/questions/SKILL.md:17, internal/cli/question_job_test.go:17 (ask is in the cli) |
| 1. Fails on base with only the two new tests | HOLDS: `go test -count=1 ./internal/cli/ ./internal/contract/ -run TestDtGateWords` exit 1. Lines: `help_golden_test.go:16: help differs from the golden file` (want `raise the ship gate`, got `wait for the person`); `agent_templates_words_test.go:18: grid-planner.md:14 says gate`, `:21 ... :27/:53/:59 says question add`, `:18 grid-planner.md:61`, `grid-curator.md:3 says gate`, `:28 does not name "grid question ask" / "Person" / "decisions_needed"` |
| 2. Words test passes | HOLDS: TestDtGateWordsTemplates passes, `go test ... ./internal/contract/` ok |
| 3. help.golden equals `grid help` | HOLDS: TestDtGateWordsHelpGolden passes (ok, ./internal/cli/). Diff is more than line 23: the golden was stale against the base table (removed stub rows views/ask/machine/deploy/sync/ports, added open/claude, global flags block, deliver wording). Content is the exact `grid help` output |
| 4. Diff names only allowed files | HOLDS: two templates, help.golden, two new tests, plus this report doc (docs/lanes) |
| 5. Tests `./internal/cli/ ./internal/contract/ ./internal/hook/ ./internal/settings/` | HOLDS: all ok, exit 0 |
| 5. `make lint` | HOLDS: exit 0 (go vet ./...) |
| Windows / macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin ...` exit 0 |
| Importing packages | Only test files and templates/golden changed; no Go package API changed, so no dependants beyond the four packages tested |
| Existing tests edited / deleted | None |
| No guards added | HOLDS: wording only |
| No attribution lines, no personal paths | HOLDS |

## What changed
- agent-templates/grid-planner.md :14, :27, :53, :54, :59, :61: no "gate"; `grid question ask` (lease holder posts a person Job, `after` names the Job id or `gate:Q-<n>`), `decisions_needed` without the lease; withdraw and list kept; `Needs you:` and probe sentences unchanged.
- agent-templates/grid-curator.md:3: "Use at deliver and at an end review".
- testdata/golden/help.golden regenerated from `grid help` (the UPDATE_GOLDEN env prefix was refused in this shell, so the file was written with the exact output the test printed; the test now compares byte for byte and passes).
- New tests: internal/cli/help_golden_test.go, internal/contract/agent_templates_words_test.go.

## NOT VERIFIED on a live screen
Nothing UI.
