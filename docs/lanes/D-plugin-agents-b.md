# D-plugin-agents-b: checker, operator, designer, curator, library and Explore templates

Head: see the report's first line. Base: main eb833a2d. Fix round 1 on top of the first hand-off. The builder is over the 60-call budget again (about 53 calls used before this file; the curator task and the real runs are NOT DONE, below).

## Proof table

| Row | State | Evidence |
| --- | --- | --- |
| 1a. checker read and fixed | HOLDS | `agent-templates/grid-checker.md:7` tools are Read, Grep, Glob and the two browser MCP sets: read-only by the tools list, no Edit, Write or Bash, no Grid guard. Proof kinds `screenshot\|video\|log\|check` as `grid proof add --help` lists; result filed with `grid job result`. |
| 1b. operator read and fixed | HOLDS | `agent-templates/grid-operator.md`: verbs checked with `go run ./cmd/grid job --help`, `proof --help`, `task --help` (exit 0). Three Grid-made caps removed (turn-30 stop, 15-minute poll cap, 40-line cap); Claude Code `maxTurns` stays. No-browser rule: `BLOCKED`, `No browser`. |
| 1c. designer read and fixed | HOLDS | `agent-templates/grid-designer.md:15`: numbered template per page type, `Data source:` and `If absent:` per block, widths 390, 1440, 3840, no plan mode. |
| 1d. curator read and fixed, brain-keeper named | HOLDS | `agent-templates/grid-curator.md:8-9` `skills: - brain-keeper` (written by D-plugin-skills-c, not here). Verbs checked with `go run ./cmd/grid brain --help` and `history --help`. Every decision carries `--note`; every link a why. |
| 1e. library | HOLDS | `agent-templates/grid-library.md`: for answering one question from the brain with note ids, read-only, haiku; verbs per `grid brain --help`. |
| 1f. Explore | HOLDS | `agent-templates/Explore.md`: read-only search agent that overrides Claude Code's built-in Explore so `grid agent set Explore` sets its model and effort (`syncName`, `internal/settings/agentsync.go`). |
| 2a. eval: checker | HOLDS (task) | `evals/tasks/agent-checker-read-only/`. |
| 2b. eval: designer | HOLDS (task) | `evals/tasks/agent-designer-spec/`. |
| 2c. eval: operator | HOLDS (task) | `evals/tasks/agent-operator-no-browser/` (task.json, prompt.md, setup.sh, checks.json): BLOCKED first line, `No browser`, site and workspace unchanged, template has a browser tool. |
| 2d. eval: library | HOLDS (task) | `evals/tasks/agent-library-answers/`: seeded home brain (two notes, written as the on-disk note files); the question says "labels", the note says "tags"; checks name `tag-storage`, the fact, the why, no unrelated note, brain and workspace unchanged, template tools have no Edit/Write. |
| 2e. eval: Explore | HOLDS (task) | `evals/tasks/agent-explore-finds/`: `UserProfile`, `user_profile`, `user-profile` in three files plus a `node_modules` decoy; checks need `path:line` for each of the three, no `node_modules`, nothing written. |
| 2f. eval: curator | NOT DONE | No task written. Seeding failed: `grid brain decide T-01-sample <id> reject --note ...` on a scratch Grid home exited 1 with `statat brain-proposals/<id>.md: no such file or directory` for a proposal at `projects/notes-app/brain-proposals/` (and for one at the home level), after the task needed `state:` and an RFC3339 `created:` in task.md and an `intent.md` (`internal/task/task.go:159`). `decide` resolves the proposal through the project store, which in an eval home needs a registered project; `grid project add` refused the folder (`folder belongs to project personal`). Needed next: a seed that registers a project, then checks on the proposal status and `--note` reason. |
| 2g. new tasks fail on base, pass on a correct run | HOLDS for operator, library, Explore | Empty agent (base): `grid eval run agent-library-answers --agent '["sh","-c","true"]'` printed `20261110T230544Z-5a16dd0cefccc6b2 agent-library-answers c small sh failed 1/7 0/1`, exit 1 (gate `answer` fails). A stand-in that writes a correct answer: library `20261110T230544Z-6f27c48e6c7712d1 ... passed 7/7 1/1 5.00/5`, Explore `20261110T230544Z-75c38d3ca2facf7a ... passed 7/7 1/1`, operator `20261110T230544Z-fc99e57f2e7466c3 ... passed 5/5 1/1`. These stand-ins prove the checks and fixtures, not the agents. |
| 2h. real run per agent (all six) | NOT DONE | Cause read: the agent step prints `Not logged in · Please run /login` and exits 1 after under 1 s (run `20261110T230547Z-c625a63d61b0b186`, library, run with `--env ANTHROPIC_API_KEY`; earlier runs `20261110T225749Z-a1330c79c2de9845` and `20261110T225752Z-24518f6727787c04` print the same, read with `grid eval show <id> --json`). The harness gives claude a temporary HOME, so only an environment login reaches it, and `--env ANTHROPIC_API_KEY` copies the variable only when it is set in the caller's environment. It is not set in this builder session, and I have no credential to give it. I did not copy the session's own login files into the temporary HOME: that would defeat the harness's isolation. To finish: run each command below from a shell where `ANTHROPIC_API_KEY` is set, after `make dist`. |
| 3a. go test, importers | HOLDS | `go test ./internal/eval ./internal/evalcmd ./internal/cli ./internal/settings ./internal/hook ./internal/packaging -count=1`: all ok, exit 0. Importers of the templates: `internal/settings`, `internal/hook`, `internal/packaging`, `internal/cli`. Whole-module test is the orchestrator's. |
| 3b. make lint | HOLDS (first hand-off), not rerun | `go vet ./...` exit 0 at the first hand-off; this round adds JSON, shell and one test-list line only. |
| 3c. Windows and macOS builds | HOLDS (first hand-off), not rerun | `make dist` built darwin/arm64, darwin/amd64, linux, windows/amd64 at the first hand-off; no Go code changed this round. |
| Rule: small commits, no attribution | HOLDS | `git log` on the lane. |
| Rule: no guard, gate, cap or hook added | HOLDS | Only eval files and one test-list line changed this round. |
| Rule: no existing test edited to pass | HOLDS with one edit | `internal/eval/suite_test.go:17-19`: added the new ids to the list of tasks in `evals/tasks`; no assertion weakened. |
| Rule: runbook check | HOLDS | Reviewer runbook read; rows carry commands and exit codes or the reason. |

## Commands for the real runs

Run from the clone root with `ANTHROPIC_API_KEY` set, after `make dist`. `<t>` is the template file and `<id>` the task:

```sh
go run ./cmd/grid eval run <id> --env ANTHROPIC_API_KEY --timeout 15m --agent '["sh","-c","cp <clone>/agent-templates/<t> .claude/agents/ && PATH=<clone>/dist/plugin/bin:$PATH exec claude -p --permission-mode acceptEdits --plugin-dir <clone>/dist/plugin"]'
```

Pairs: agent-checker-read-only with grid-checker.md; agent-operator-no-browser with grid-operator.md; agent-designer-spec with grid-designer.md; agent-library-answers with grid-library.md; agent-explore-finds with Explore.md. Curator: no task yet.

## What changed

- `agent-templates/grid-checker.md`, `grid-operator.md`, `grid-designer.md`, `grid-curator.md` (first hand-off).
- `evals/tasks/`: `agent-checker-read-only`, `agent-designer-spec` (first hand-off); `agent-operator-no-browser`, `agent-library-answers`, `agent-explore-finds` (this round).
- `internal/eval/suite_test.go`: the new ids added to the expected list.

## NOT VERIFIED

- Any real agent run: all six are blocked by the missing login in the harness environment (row 2h).
- The curator's frontmatter `skills: brain-keeper` loading: the skill lands with D-plugin-skills-c.
- Browser behaviour: no browser on the box; the checker and operator `No browser` replies are unproven until a real run.
- Whether `grid` on PATH inside the agent step is what an orchestrator run uses: the commands above put `dist/plugin/bin` first.
