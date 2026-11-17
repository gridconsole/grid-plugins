# C-cp-first-run-offer: proof table

Base: 6fa1a6f1d40ff7ded7b51c091fbdaeacc4c90172 (lane-Pint242 merge; origin/main already merged, `git merge origin/main` said "Already up to date").

Base check: on base `git grep -n systemMessage -- internal/hook` held only test files (overlimit_level_test.go:49, worker_job_tooluse_test.go:86,99), `ls skills/` had no first-run, and `grid init --detect --json` and `--answers -` exist (internal/cli/init.go, contract/commands.json:5358-5408, cp_init_test.go). HOLDS.

| Row | Result |
| --- | --- |
| 1 (V1) No config and no data: offer in systemMessage and one context line, exit 0 | HOLDS. `go test -count=1 -v -run CpOffer ./internal/hook/` exit 0, `--- PASS: TestCpOfferNoConfig`. Code: internal/hook/hook.go (`cpOfferWanted`, `cpOfferMessage`, `cpOfferContext`, and the `offer` use in sessionStart). |
| 2 (V1) Config at gridconfig.Path, only the old ~/.grid/config.json, or data under ~/.grid: no offer | HOLDS. Same run: `--- PASS: TestCpOfferQuietWithConfig` (both subtests) and `--- PASS: TestCpOfferQuietWithData` (VERSION, projects, knowledge under ~/.grid, and data under ~/.grid/<workspace>). |
| 3 Existing hook tests pass unchanged | HOLDS. `go test -count=1 ./internal/hook/` ok (7.8s). No existing test edited. |
| 4 first-run in catalog, golden, schema | HOLDS. `go test -count=1 . ./internal/hook/ ./internal/settings/ ./internal/cli/ ./internal/job/... ./internal/packaging/ ./internal/account/ ./internal/brain/ ./internal/doctor/ ./internal/guard/ ./internal/updates/` all ok. Those are the packages that directly import hook or settings (from `go list`: account, brain, cli, doctor, guard, job/jobcmd, packaging, updates) plus the brief's list. Edits: internal/settings/catalog.go:30, internal/cli/testdata/settings/skills.json, contract/schemas/cmd-skill-list.json (property and `required`), contract/fixtures/cmd-skill-list/valid/basic.json and populated.json, skills/first-run/SKILL.md. |
| 5 `make lint`, `make cross` | HOLDS. `make lint` exit 0 (go vet ./...); `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| New tests fail on base, pass on lane | HOLDS. With internal/hook/hook.go stashed: `--- FAIL: TestCpOfferNoConfig`, `cp_offer_test.go:62: systemMessage "" does not offer the first run`, `cp_offer_test.go:68: additionalContext names grid:first-run 0 times, want 1`, `cp_offer_test.go:75: a second SessionStart on the same machine dropped the offer`. Restored: pass. The Quiet tests pass on base too, as they pin the absence of an offer. |
| Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. `make webui-test`, TestFvArgvCollects, cmd/grid-server, `internal/slices` and `make realplugin` are app-repo checks: not run, not asked (plugin repo). |
| Fetch and merge origin/main before the last proof run | HOLDS. `git fetch origin` then `git merge origin/main`: Already up to date. |
| Windows and macOS builds | HOLDS. `make cross` above. |
| No attribution lines | HOLDS. `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` prints 0. |
| No guard, gate, cap or block on agents | HOLDS. The offer is output only; the hook still exits 0 and prints its usual context. Nothing removed. |
| Offer reads only the config paths and one folder list | HOLDS. `cpOfferWanted` does `os.Stat` on gridconfig.Path and gridconfig.OldPath, `os.Stat` for VERSION/projects/knowledge in ~/.grid and the Grid home, and one `os.ReadDir` of ~/.grid. No network call, no wait. It runs before recordSession and Resolve, so this session's own writes under the Grid home do not silence the offer (tested by the second run in TestCpOfferNoConfig). |

## What changed

- internal/hook/hook.go: sessionStart asks `cpOfferWanted` right after the early return and, when true, appends one line to additionalContext and sets `systemMessage`.
- internal/hook/cp_offer_test.go (new): TestCpOfferNoConfig, TestCpOfferQuietWithConfig, TestCpOfferQuietWithData.
- skills/first-run/SKILL.md (new): runs `grid init --detect --json`, asks (AskUserQuestion) only the questions with `asked: true`, pipes the answers to `grid init --answers -`. Every `grid ...` span matches contract/commands.json (TestShippedAgentsAndSkillsAreWellFormed passes).
- Catalog, skills golden, cmd-skill-list schema and its two valid fixtures name `first-run`.
- testdata/golden/hook-sessionstart-{git,personal,empty-input-uses-cwd}.golden: these goldens pin SessionStart on an empty machine (no config, no data), so the offer is now in them. Edited by hand (`UPDATE_GOLDEN=1` is refused in the sandbox) to the exact output the test printed. The test TestHookSessionStartGolden is unchanged. These goldens are outside the brief's Touches list; the changed code forces them (runbook reviewer rule 13).

## NOT VERIFIED on a live screen

- That Claude Code shows `systemMessage` of a SessionStart hook to the person, and that the model offers the skill from the context line. Only the hook's JSON output is tested.
- The skill's AskUserQuestion flow in a live session.
- `grid:first-run` as the skill's namespaced name (the plugin's other skills show as `grid:<name>`).
- Hook run with GRID_HOME set to an empty folder and no config: the offer shows (the brief names only ~/.grid data).

DONE
