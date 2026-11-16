# P-ctx-haiku-helpers: Haiku helpers ship as real plugin agents, and builders hand them heavy reads

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. scout, runner, library ship in agents/ on haiku with the templates' tools | HOLDS | `agents/scout.md:4`, `agents/runner.md:4`, `agents/library.md:4` (`model: haiku`); tools, effort and description equal the templates' (`go test -count=1 -run TestHh .` exit 0, TestHhHelpersShipAsPluginAgents) |
| 1. agent-templates/ still works | HOLDS | templates unchanged; `make test` walks over agent-templates pass (internal/hook, internal/contract, internal/job, internal/settings ok) |
| 2. run-job Helpers rules (scout, runner, line range, library), one line each with its reason | HOLDS | `skills/run-job/SKILL.md:59-66`; TestHhSkillsHandHeavyReadsToHelpers exit 0 |
| 2. review-job, where it reads code | HOLDS | `skills/review-job/SKILL.md:18-22` |
| 3. builder, reviewer, checker point at the rules in one line, no pasted text | HOLDS | `agents/builder.md:28`, `agents/reviewer.md:29`, `agents/checker.md:30`; TestHhAgentsPointAtHelperRules exit 0 |
| 4. new tests fail on base, pass on the lane | HOLDS | base failing lines below; lane `go test -count=1 -run 'TestHh\|TestEr' .` exit 0 |
| 4. er_descriptions tests pass with the new entries | HOLDS | TestErDescWalkEveryFile walks agents/*.md, so it reads the three new files: `go test -count=1 -run 'TestHh\|TestEr' .` exit 0 |
| 5. make test | HOLDS, one unrelated flake | `make test` exit 2: only `internal/env` TestUpHealthTimeoutHonorsStopGrace (`env_test.go:260: signals.log = ""`). The lane does not touch internal/env (`git diff --stat origin/main HEAD`) and nothing in it imports the changed packages. Rerun on the lane, `go test -count=3 -run TestUpHealthTimeoutHonorsStopGrace ./internal/env/`, exit 0; the same test on origin/main in a temporary worktree, exit 0 |
| 5. make lint | HOLDS | `make lint` exit 0 |
| 5. make dist | HOLDS | `make dist` exit 0 (darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64) |
| 5. lane doc, last line DONE or NOT DONE | HOLDS | this file |
| Rule: tests of changed packages and their importers | HOLDS | changed: root `gridplugins`, `internal/cli`. Importers (go list): root is imported by internal/cli, internal/job, internal/job/jobcmd and by the tests of internal/brain, braincmd, contract, routine and settings; internal/cli is imported by cmd/grid and lib and by the tests of contract/formats, account, braincmd, golden, jobgraph, land, mcp, packaging, question and settings. All of them ran in `make test` and passed. |
| Rule: Windows and macOS builds | HOLDS | `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| Rule: walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0 |
| Rule: fetch and merge origin/main before the last proof run | HOLDS | `git fetch origin` and `git merge origin/main` exit 0, nothing new to merge |
| Rule: no guard, gate, cap, hold or blocking hook | HOLDS | none added. The worker-spawn change below keeps the spawn exactly as it was on base. |
| Rule: no attribution lines | HOLDS | `git log origin/main..HEAD` |
| Cross-repo set, whole-module test at landing | not run, not asked | the orchestrator runs these at landing |
| Real plugin, guest walk tables | not run, not asked | app-repo checks; this lane is plugin only |

Failing lines on base (`go test -count=1 -run 'TestHh' .`, exit 1):

```
--- FAIL: TestHhHelpersShipAsPluginAgents (0.00s)
    haiku_helpers_test.go:44: agents/scout.md does not ship
    haiku_helpers_test.go:44: agents/runner.md does not ship
    haiku_helpers_test.go:44: agents/library.md does not ship
--- FAIL: TestHhSkillsHandHeavyReadsToHelpers (0.00s)
    haiku_helpers_test.go:81: skills/run-job/SKILL.md lacks "## Helpers"
    haiku_helpers_test.go:81: skills/run-job/SKILL.md lacks "`grid:scout`"
    haiku_helpers_test.go:81: skills/run-job/SKILL.md lacks "`grid:runner`"
    haiku_helpers_test.go:81: skills/run-job/SKILL.md lacks "line range"
    haiku_helpers_test.go:81: skills/review-job/SKILL.md lacks "`grid:scout`"
    haiku_helpers_test.go:81: skills/review-job/SKILL.md lacks "line range"
--- FAIL: TestHhAgentsPointAtHelperRules (0.00s)
    haiku_helpers_test.go:100: agents/builder.md: 0 lines point at the Helpers rules, want 1
    haiku_helpers_test.go:100: agents/reviewer.md: 0 lines point at the Helpers rules, want 1
    haiku_helpers_test.go:100: agents/checker.md: 0 lines point at the Helpers rules, want 1
    haiku_helpers_test.go:110: agents/builder.md: tools "Read, Grep, Glob, Bash, Edit, Write, Skill" lack Agent
    haiku_helpers_test.go:110: agents/reviewer.md: tools "Read, Grep, Glob, Bash, Skill" lack Agent
```

## What changed

- `agents/scout.md`, `agents/runner.md`, `agents/library.md`: new plugin agents, which Claude Code names `grid:scout`, `grid:runner` and `grid:library`. Each keeps its template's description, `model: haiku`, `effort: low` and tools, plus the template's whole body, because a Job of that role now renders this file (`embed.go` AgentFor prefers agents/). Each adds one line: with no Job, answer the agent that sent you and post nothing. The runner's tools are `Bash, Read, Grep, Edit`, as its template sets them; Edit is the template's choice, so this set is not strictly read-only.
- Forced by existing walk tests that read every `agents/*.md`. The three files preload `skills: [run-job]`, because `internal/contract/im_job_bash_lines_test.go:128` needs every agent to preload a skill that carries the Bash lines. The runner and library also carry the one-command Bash line, because `internal/job/r3oc_one_command_test.go:65` needs it in every agent that has Bash.
- `embed.go`: adds `IsHelperAgent`. `internal/cli/worker.go` workerSpawnTools does not pass a helper agent's `tools:` to a worker spawn. Why: without this, a scout Job would spawn with `Read, Grep, Glob` and could not run `grid job result`. The existing test that covers this is `internal/cli/p_r23_spawn_tools_test.go:9` TestTemplateOnlyRoleSpawnsWithoutTools (scout, runner and library spawn with no tools list); it passes unchanged on the lane. I did not paste a run of it failing without the worker.go change.
- `skills/run-job/SKILL.md`: a `## Helpers` section with four rules, one line each with its reason: an unknown code area goes to `grid:scout`, a long test, build or lint run to `grid:runner`, a large file or log is read by line range, and a brain question goes to `grid:library`. `skills/review-job/SKILL.md` holds the three rules a reviewer reading code needs.
- `agents/builder.md`, `agents/reviewer.md`, `agents/checker.md`: one line each pointing at the Helpers section of the run-job skill. Builder and reviewer gain `Agent` in `tools:`. That line is what a worker spawn passes on, so without Agent a spawned builder could not send a helper. The builder template already lists Agent.
- `haiku_helpers_test.go`: the new tests.

## What reads agent-templates/, and whether the copies are still needed

- `internal/settings/agentsync.go` (`grid agent sync`, run by the hook) copies `agent-templates/grid-*.md` into the user's `~/.claude/agents` as `grid-scout` and so on.
- `embed.go` embeds them as the AgentFor fallback for roles with no agents/ file (researcher, designer, operator, planner).
- `internal/packaging/dist.go:66` ships the folder.
- The settings catalog (`internal/settings/catalog.go`) names roles `grid-<role>` for model and effort defaults.

The copies are still needed: the other roles have no agents/ file, and agent sync and the catalog read them by name. For scout, runner and library, the `grid-*` copies in `~/.claude/agents` now duplicate the plugin agents. Retiring those three templates is a follow-up for the owner, because agentsync, the catalog and several walk tests read them.

## Open items

- Checker: `tools:` has no Agent. `internal/cli/be_worker_spawn_tools_test.go:10` pins the checker's tools exactly, and an existing test may not be edited to pass. So a spawned checker follows only the line-range rule and cannot send a helper.
- NOT VERIFIED on a live session: that Claude Code lists the agents as `grid:scout`, `grid:runner` and `grid:library` once the plugin is installed, and that builders actually send them. The orchestrator can measure this from the next session logs (subagent calls per session).
- `internal/env` TestUpHealthTimeoutHonorsStopGrace failed twice on the lane and then passed 3 of 3, on files this lane does not touch. It is timing-flaky.

DONE
