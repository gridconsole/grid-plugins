# P-r23-result-shape

Findings: /srv/gridwork/tmp/hand/prev/U-chk-plugin-rev-skills.md rows 4, 5, 11, 12. Base: origin/main 0f360eaa.

| Row | Result |
|---|---|
| 1. Row 4 confirmed | CONFIRMED on main: `agent-templates/grid-reviewer.md:7` tools line `Read, Grep, Glob, Bash, Edit, Skill`; `:12` "Your only write is the verdict. Use Bash only for ... `grid job show`, `grid feed read`, and the one `grid job result`"; `skills/review-job/SKILL.md:26` posts each finding with `grid job post --type found`. |
| 1. Row 5 confirmed | CONFIRMED on main: `agent-templates/grid-builder.md:12` "one line under `Noticed`", `:20` "first line is `Blocked:`", `:24` "a `Blocked:` line", `:40` refusal "end the turn" with no result. `outcome` is the result key (`contract/schemas/result.json`, `skills/run-job/SKILL.md:27-47`). |
| 1. Row 11 confirmed | CONFIRMED on main: `agent-templates/grid-operator.md:14` "exactly `BLOCKED`", `:16` "post `Blocked:`", `:26` "first line is `DONE`, `PARTIALLY DONE` or `BLOCKED`"; none is a key of the result body. |
| 1. Row 12 confirmed | CONFIRMED on main: `agents/reviewer.md:6` tools `Read, Grep, Glob, Bash`, `:24` names "the `questions` skill"; `agents/checker.md:9` skills list `browser`, `review-job`; neither lists `questions`. |
| 2. Row 4 fixed, test | `TestR23ReviewerTemplatePostsFoundWithoutEdit` (internal/contract/r23_result_shape_test.go). On base: `grid-reviewer.md: the tools: line has Edit, but the reviewer never edits the work` and `the "Use Bash only" sentence does not name `grid job post --type found``. On head: PASS. |
| 2. Row 5 fixed, test | `TestR23WorkerTemplatesUseResultKeys`. On base: `grid-builder.md: still says "first line is `Blocked:`"`, `still says "under `Noticed`"`, `never names the result's `outcome` key`, `the refusal paragraph ends the turn without posting a result with `outcome``. On head: PASS. |
| 2. Row 11 fixed, test | Same test. On base: `grid-operator.md: still says "Its first line is `DONE`"`, `still says "`PARTIALLY DONE`"`, `still says "exactly `BLOCKED`"`, `does not list run-job under skills:`. On head: PASS. |
| 2. Row 12 fixed, test | `TestR23ReviewerAndCheckerCanUseQuestions`. On base: `agents/reviewer.md: skills: does not list questions`, `agents/checker.md: skills: does not list questions`, `agents/reviewer.md: tools: has no Skill`. On head: PASS. Base run: `go -C <detached worktree of origin/main with only the new test copied in> test -count=1 ./internal/contract/ -run TestR23`, exit 1. Head run: `go test -count=1 ./internal/contract/ -run TestR23 -v`, 3 PASS, exit 0. |
| 3. go test of changed packages and importers | HOLDS: `go test -count=1 ./internal/contract/ ./internal/job/lint/ ./internal/orchestrator/ ./internal/braincmd/` exit 0; `go test -count=1 ./internal/cli/ ./internal/packaging/ ./internal/settings/ ./internal/hook/ ./internal/mcp/ ./internal/guard/ .` exit 0. Only a test file and markdown files changed, so no package has a new importer; these are the packages whose tests read the changed agent and skill files. |
| 3. make lint | HOLDS: `make lint` (go vet ./...), exit 0 |
| 3. Walk tests | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| Windows and macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| `git fetch origin && git merge origin/main` | HOLDS: already up to date |
| No existing test edited | HOLDS: only the new r23_result_shape_test.go was added. A first try that gave the checker the `Skill` tool failed `TestBeWorkerSpawnToolsFromAgentFile`; the checker's tools line was put back and the new test requires `Skill` on the reviewer only. A first try left `questions` without Bash lines and failed `TestImBashEveryJobAgentSkillCarriesTheLines`; the lines were added to `skills/questions/SKILL.md`. |
| No guard, gate or cap added | HOLDS: text and front matter only |
| No attribution in commits | HOLDS |

## What changed
- `agent-templates/grid-builder.md`: `Noticed` becomes `proposed_jobs` or `decisions_needed`; a blocked Job and a refused command post a result whose `outcome` starts `Blocked:` with the open rows `not met`; the result body is the one the run-job skill shows.
- `agent-templates/grid-operator.md`: `DONE`, `PARTIALLY DONE`, `BLOCKED` first lines replaced by the run-job result body (`outcome` starts `Blocked:` when blocked, rows `met` or `not met`); `skills: run-job` added so the body shape is loaded.
- `agent-templates/grid-reviewer.md`: no `Edit` tool; Bash line names `grid job post --type found`; found records posted before the verdict.
- `agents/reviewer.md`: `Skill` tool and `questions` skill. `agents/checker.md`: `questions` skill.
- `skills/questions/SKILL.md`: one-command Bash lines (needed once a worker agent preloads it).

## NOT VERIFIED on a live screen
No live agent run was made: that a spawned reviewer or checker loads the `questions` skill from `skills:` is not seen in a running session.
