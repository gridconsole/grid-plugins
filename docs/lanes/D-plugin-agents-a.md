# D-plugin-agents-a: planner, scout, researcher and runner templates

Repo: plugin, branch lane-D-plugin-agents-a, base eb833a2d.

## Proof table

| Row or rule | Result |
|---|---|
| 1. Four templates read against the old agent and the owner decisions, fixed, verbs exist | HOLDS. `agent-templates/grid-{planner,scout,researcher,runner}.md` rewritten. Verbs checked with `go run ./cmd/grid <verb> --help` (exit 0): `job post/result/show/lint`, `plan write/check/items/hash`, `task new/show/status`, `question add/withdraw/list`, `history add`, `brain search/show/index --read-only/keep/propose`, `proof add`, `workflow show`, `person approvers`. The planner gained a `tools:` line (it had none) and no longer has the old reply caps, `maxTurns`, review-round limit, "15 minutes" or hard-coded reviewer gate. |
| 2. One eval task per agent, ported from old where they apply | HOLDS for the files: `evals/tasks/agent-planner-delegates`, `agent-scout-maps`, `agent-researcher-sources`, `agent-runner-runs-once` (the first two ported from `old-plugin/evals`, rewritten to Jobs). `go run ./cmd/grid eval list --suite evals/tasks --tier b` lists all four, exit 0. |
| 2. Real run pasted per agent | NOT DONE. `go run ./cmd/grid eval run agent-runner-runs-once --agent '["claude","-p","--plugin-dir",".../dist/plugin"]' --env ANTHROPIC_API_KEY --timeout 9m` after `make dist` (exit 0): run `20261111T002622Z-3936eb87add8f47d`, `failed 0/5`, agent exit 1 after 1.6 s, stdout `Not logged in · Please run /login`. The VM has no login and no API key in the environment, and the harness uses a temporary HOME. I did not run the other three: they would fail the same way. Run them where `claude` is logged in or `ANTHROPIC_API_KEY` is set. |
| 3. Test of valid front matter for every template, and sync into a scratch HOME | HOLDS. `internal/hook/agenttemplates_test.go`: `TestShippedTemplatesFrontMatter` (name equals file name, description, model haiku/sonnet/opus, tools) and `TestSessionStartSyncsEveryShippedTemplate` (all files written into a scratch `.claude/agents`). Fails on base: `agenttemplates_test.go:62: grid-planner: no tools line` (I stashed `agent-templates` and ran it). Passes on the lane: `go test -count=1 ./internal/hook/` exit 0. Older tests cited: `TestShippedTemplatesRender` (`internal/settings/agentsync_test.go:44`), `TestSessionStartSyncIdempotent` (`internal/hook/agentsync_test.go:42`). |
| 4. go test and make lint | Packages I changed and their importers: `go test -count=1 ./internal/hook/... ./internal/settings/... ./internal/cli/... ./internal/packaging/... ./internal/doctor/... ./internal/guard/... ./internal/golden/... ./internal/eval/... ./internal/evalcmd/...` ran ok for all after I fixed the two eval tests below; `make lint` exit 0; `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. The whole-module test is the orchestrator's. |
| Docs: which scheme a session uses | HOLDS. `docs/agents.md`. Not removed: see open item 1. |
| Rule: no guard, gate, cap or hold added | HOLDS. The templates lost caps; the test and evals only read. |
| Rule: new tests fail on base | HOLDS for row 3 (above). The eval tasks fail on a fresh fixture by design: `TestChecksRunOnFreshFixture` in `internal/eval/home_test.go` runs every tier-b check on it and passes. |
| Rule: small commits, no attribution lines | HOLDS. |

## Existing tests I changed

The harness counts tier-b cases and lists every task, so adding four tasks changed two numbers, not an assertion's meaning: `internal/eval/home_test.go:34` `16` to `20`; `internal/eval/suite_test.go:14` four ids added to the wanted list. Two of my tasks are size `small` because `internal/evalcmd/command_test.go:47` expects exactly one tiny tier-b task.

## What is NOT VERIFIED

- Any real model behaviour: no eval run reached the model (not logged in). Whether the four agents load from `~/.claude/agents` after SessionStart in `claude -p --plugin-dir` is not seen.
- The eval checks `job-posted` (planner) assume `grid job post` writes a file naming `store.js` under `<task>/feed/`; not run. A hand-made task fixture may lack the lease `job post` needs, so that check could fail for the fixture and not the agent.
- The scout and runner checks read a file the main session writes from the subagent's reply, so they cannot tell the subagent from a main session that did the work itself.
- The 10 Nov brief and Job rules (lists for moves, path:line, UI widths, one budget number, split at about 60 calls, proof table) belong in the `plan-jobs` and `run-job` skills, which this lane did not touch.

## Open items

1. `agents/` (builder, orchestrator, reviewer) and `agent-templates/grid-builder`, `grid-reviewer`, `grid-planner` overlap. The session uses the templates. The owner picks which set goes.
2. The other eight templates still carry `maxTurns` and reply-length caps.
3. `grid-planner` has `Agent` and `Skill` in `tools:`; whether a subagent can start subagents is Claude Code's rule, not seen here.
