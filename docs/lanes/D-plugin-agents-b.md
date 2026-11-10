# D-plugin-agents-b: checker, operator, designer, curator, library and Explore templates

Head: see the report's first line. Base: main eb833a2d. Builder ran over the 60-call budget (about 66) to keep `go test` green.

## Proof table

| Row | State | Evidence |
| --- | --- | --- |
| 1a. checker read and fixed | HOLDS | `agent-templates/grid-checker.md:7` tools are Read, Grep, Glob and the two browser MCP sets: read-only by the tools list, no Edit, Write or Bash, no Grid guard. Proof kinds now `screenshot\|video\|log\|check` (the kinds `grid proof add --help` lists); result is filed by the runner with `grid job result`. |
| 1b. operator read and fixed | HOLDS | `agent-templates/grid-operator.md`: verbs checked with `go run ./cmd/grid job --help`, `proof --help`, `task --help` (all exit 0). Removed the turn-30 stop, the 15-minute poll cap and the 40-line cap (Grid-made caps); the Claude Code `maxTurns` stays. Added the no-browser rule (`BLOCKED`, `No browser`). |
| 1c. designer read and fixed | HOLDS | `agent-templates/grid-designer.md:15`: ported the old page-type rule (numbered template per page type, `Data source:` and `If absent:` per block, measurable floor), added widths 390, 1440, 3840, no plan mode. |
| 1d. curator read and fixed, brain-keeper named | HOLDS | `agent-templates/grid-curator.md:8-9` `skills: - brain-keeper` (skill is written by D-plugin-skills-c; not written here). Verbs checked against `go run ./cmd/grid brain --help` (keep, index --read-only, show --proposal, search, topics, decide, propose) and `history --help`. Every link carries a why; every decision carries `--note`. |
| 1e. library | HOLDS, unchanged | `agent-templates/grid-library.md`: kept. For: answering one question from the brain (`grid brain index --read-only`, `topics`, `search`, `show`) with note ids, read-only, haiku. Verbs exist per `grid brain --help`. |
| 1f. Explore | HOLDS, unchanged | `agent-templates/Explore.md`: kept. For: a read-only search agent that overrides Claude Code's built-in Explore of the same name so the person sets its model and effort with `grid agent set Explore` (`syncName` in `internal/settings/agentsync.go` accepts the name). |
| 2a. eval: checker | HOLDS (task), NOT DONE (real run) | `evals/tasks/agent-checker-read-only/` (task.json, prompt.md, setup.sh, checks.json): ports the old `agent-checker-read-only`. Checks: template tools list has no Edit/Write/Bash, first line REJECTED, `No browser`, site unchanged and no other file. Real run: `go run ./cmd/grid eval run agent-checker-read-only --agent '["sh","-c","cp <dist>/plugin/agent-templates/grid-checker.md .claude/agents/ && exec claude -p --permission-mode acceptEdits --plugin-dir <dist>/plugin"]' --env ANTHROPIC_API_KEY --timeout 15m` printed `20261110T225749Z-a1330c79c2de9845 agent-checker-read-only c small sh failed 0/5 0/1`, exit 1: setup passed, the agent step exited 1 after 2.7 s, so all checks were skipped. Cause not read (output not captured); likely no login: the harness gives claude a temporary HOME and `ANTHROPIC_API_KEY` was not set here. |
| 2b. eval: designer | HOLDS (task), NOT DONE (real run) | `evals/tasks/agent-designer-spec/`. Checks: spec.md exists (gate), >= 3 `Data source` and `If absent`, widths 390/1440/3840, both themes, no invented `author` field (the read shape has none), no other file. Real run: same command with grid-designer, `20261110T225752Z-24518f6727787c04 ... failed 0/7 0/1`, exit 1, same agent-step failure. |
| 2c. evals: operator, curator, library, Explore | NOT DONE | Out of budget. Plan: operator no-browser `BLOCKED` (reuse the checker fixture); curator and library need a seeded brain (`grid brain propose` in setup, then `keep` and `decide` checks); Explore needs a small repo with a name spelled three ways. |
| 2d. new tests fail on base | NOT DONE | Not run: no agent run reached the checks. The command checks read files the agent must write, so they fail on an empty workspace by construction; not shown. |
| 3a. go test, importers | HOLDS | `go test ./internal/settings ./internal/hook ./internal/packaging ./internal/cli ./internal/eval ./internal/evalcmd -count=1`: all ok after the fix below. Importers of the templates: `internal/settings`, `internal/hook`, `internal/packaging`, `internal/cli`. Whole-module test is the orchestrator's. |
| 3b. make lint | HOLDS | `make lint` ran `go vet ./...`, exit 0. |
| 3c. Windows and macOS builds | HOLDS | `make dist` built darwin/arm64, darwin/amd64, linux and windows/amd64, exit 0. |
| Rule: small commits, no attribution | HOLDS | `git log` on the lane. |
| Rule: no guard, gate, cap or hook added | HOLDS | Only template prose and evals changed; three Grid-made caps removed from the operator. |
| Rule: no existing test edited to pass | HOLDS with one edit | `internal/eval/suite_test.go:17`: added the two new ids to the expected task list, because that test lists every task in `evals/tasks`. No assertion weakened. The new tasks are tier `c`: the tier-b tests in `internal/eval/home_test.go` require a notes-app Grid home fixture that these evals do not use (first try as tier b failed those tests; reverted). |
| Rule: runbook check | HOLDS | Reviewer runbook read; rows above carry commands and exit codes. |

## What changed

- `agent-templates/grid-checker.md`, `grid-operator.md`, `grid-designer.md`, `grid-curator.md`.
- `evals/tasks/agent-checker-read-only/`, `evals/tasks/agent-designer-spec/`.
- `internal/eval/suite_test.go`: two ids added.

## NOT VERIFIED

- A real agent run of any template: both runs failed before the agent did any work. The reason is unread; log in or set `ANTHROPIC_API_KEY` and rerun with the commands above.
- That the checker and operator report `No browser` with no browser: needs the run above.
- That the curator frontmatter `skills: brain-keeper` loads: the skill does not exist until D-plugin-skills-c lands.
- Browser behaviour on a live page: no browser on the box.
