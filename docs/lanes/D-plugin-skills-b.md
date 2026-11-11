# D-plugin-skills-b: the deliver, questions and research skills

Base: plugin main eb833a2d. Lane branch: lane-D-plugin-skills-b.

## Proof table

| Row | Status | Evidence |
| --- | --- | --- |
| 1. skills/deliver, skills/questions, skills/research SKILL.md exist, verbs exist | HOLDS | `skills/deliver/SKILL.md`, `skills/questions/SKILL.md`, `skills/research/SKILL.md`. `go test ./internal/packaging` exit 0 (`TestSkillCommandsImplemented` reads every `grid <verb>` named in a skill; it failed first on the word "grid ask" in the questions skill and passed after I removed it). Subverbs checked with `--help`: deliver, proof add/index, plan items, brain propose/keep/decide/search/show, question add/list/withdraw, answer, person approvers/show, feed post, feed types (has `question`), job show/result, task history. |
| 2. deliver refuses to call a task done without proof per plan item, and proposes brain notes | HOLDS for the text, NOT VERIFIED live | `skills/deliver/SKILL.md` steps 2-7: proof per plan item with `grid proof add`, a `--dry-run` first, the refusal read and fixed, an unproven item reported as NOT DONE, the brain proposal step (`grid brain propose` with a link and a why, then the grid-curator subagent, never the person). The refusal itself is the CLI's (`docs/delivery.md`, "Delivery refuses ... missing or invalid proof"), not a rule the skill invents. |
| 3. One eval task per skill, real run pasted | PARTLY: tasks HOLD, real run NOT DONE | `evals/tasks/skill-deliver`, `skill-questions`, `skill-research`. Real run blocked: see below. |
| 4. go test ./... and make lint 0 fail | HOLDS | `go test ./...` printed no FAIL line (exit 0); `make lint && echo LINT-OK` printed LINT-OK. |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| `grid ask` has no code | NOT DONE (a missing verb) | `go run ./cmd/grid ask --help` prints `ask: not built yet`, exit 2. The questions skill uses `grid question add` (and `grid feed post --type question`) and names the gap. |
| Old evals deliver-* ported | NOT DONE | The old fixtures (`deliver-skill`, `deliver-refuses-closed`, `deliver-refuses-rejected`) are already in `evals/tasks` as ported cases that use the old `status:` and `subagents/` layout. I wrote a new-layout `skill-deliver` (plan items, proof, brain proposal) and did not port them again for lack of calls. |
| No guard, gate, cap or hold added | HOLDS | The skills only name refusals that `grid deliver` and the questions verbs already return. No hook, no Go code. |
| New tests fail on base, pass on lane | HOLDS for the harness, see below | |
| Existing test edited | One line | `internal/eval/suite_test.go` lists every task id in `evals/tasks`; I added the three new ids to that list (a longer list, no check weakened). `TestSampleSuite` failed with "unexpected tasks" before and passes after (`go test ./internal/eval` ok). |

## Eval real run: NOT DONE

`go run ./cmd/grid eval run skill-research --agent '["claude","-p","--plugin-dir","<clone>/dist/plugin"]' --env ANTHROPIC_API_KEY --timeout 15m` after `make dist`:

```
20261111T002707Z-28f4c1bc08b66b99  skill-research  b  small  claude  failed  0/7  0/1  not measured  1257ms
agent stdout: "Not logged in · Please run /login"
```

The eval gives the agent a temporary HOME, so the Claude login file is not found, and no credential variable (`ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, `CLAUDE_CONFIG_DIR`) is set in this builder's environment. `CLAUDE_CODE_*` variables are refused by `--env`. The same happened for skill-questions and skill-deliver (agent exit 1 within 2 seconds, checks skipped). Someone with a key must run the three commands; the checks were not tuned against a real transcript.

What I could prove: with a do-nothing agent (`--agent '["true"]'`) each task's checks fail on the untouched fixture, so the checks fail when the skill or its work is missing:

```
skill-research   c small true failed 2/7 0/1 1.11/5
skill-questions  c small true failed 3/7 0/1 1.88/5
skill-deliver    c small true failed 3/6 0/0 1.50/5
```

(The passing checks of each are the "left alone" ones: intent untouched, not closed, code unchanged.)

## What changed

- `skills/deliver/SKILL.md`: proof per plan item, dry run, brain proposal step folded in from the old memory skill, deliver, with the owner decisions quoted.
- `skills/questions/SKILL.md`: person points only, `grid question add` with options, recommendation and why, `grid answer`, withdraw, Feed `question` post.
- `skills/research/SKILL.md`: one question per Job, `path:line` for every code fact, `research.md`, unverified marked.
- `evals/tasks/skill-deliver`, `skill-questions`, `skill-research` (tier c, because `internal/eval/home_test.go` counts exactly 16 tier-b ported cases and builds a good outcome for each).
- `internal/eval/suite_test.go`: three ids added to the task list.

## Open items and NOT VERIFIED

- Real model runs of the three evals (credentials). Check ids and regexes in `skill-questions/checks.json` assume `grid question add` writes a file under the task folder naming "sam", the options, "recommend" and "why"; I did not run `grid question add` to see the file format, so those checks may need adjusting after the first real run.
- `skill-deliver` assumes `grid deliver` accepts the hand-built fixture (a legacy-style `task.md`, plan.md, a "plan accepted" history line) and that `grid brain propose` writes a path containing "proposal"; not run.
- The deliver skill names a `grid-curator` subagent; `agents/` in this repo has only builder, orchestrator and reviewer, so the curator file belongs to the agents lane.
- The skills lean on `grid plan items --json`, `grid brain propose --link` (JSON with `to`, `relation`, `why`): flags read from `--help`, not run end to end.
- Not covered: UI rows (none in this brief).
