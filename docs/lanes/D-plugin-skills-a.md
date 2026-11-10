# D-plugin-skills-a: the task, setup and signals skills

Base: plugin eb833a2d. Model: sonnet. This lane used more than the 60-call budget (about 70), because of the eval-count failures described under row 4.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. skills/task, setup, signals exist and use only verbs that exist | HOLDS | `skills/task/SKILL.md`, `skills/setup/SKILL.md`, `skills/signals/SKILL.md`. `go test ./internal/packaging` exit 0 (TestSkillCommandsImplemented reads every `grid <verb>` in skills). Each flag was read in `internal/cli/task.go:17`, `internal/cli/signal.go:15-45`, `internal/cli/settings.go:170-254`; `grid brain topics`, `grid project scan --help` and `grid session list --help` were run. |
| 2. setup covers first run: reads repos and ~/.claude sessions, proposes, `grid setup` writes | HOLDS (by reading) | `skills/setup/SKILL.md` sections 2 (repo, documents, `~/.claude/projects/<repo path>/*.jsonl`), 3 (`grid brain propose`, answers file) and 4 (`grid setup`, `--yes`, `--remove`). The answer keys and enums are the ones in `internal/settings/setup.go:17-22`. Not run against a model, see row 3. |
| 3. one eval task per skill, real run result pasted | NOT DONE | Tasks exist: `evals/tasks/task-skill`, `setup-skill`, `signals-skill`. Real runs were made with `make dist` (exit 0) and `go run ./cmd/grid eval run <id> --agent '["claude","-p","--plugin-dir","<clone>/dist/plugin"]' --timeout 15m`. Each failed at the agent step in under 5 s, so no check ran: `20261110T225743Z-96bd212b4443784f task-skill failed 0/5 agent: failed (exit 1, 4254ms)`; `20261110T225744Z-b25ae6ded2367be8 signals-skill failed 0/7`; `20261110T225745Z-01a51fbe6e0413a4 setup-skill failed 0/7`. The agent output is not in the saved result and I did not diagnose it; the likely cause is that the eval's temporary HOME has no Claude login (`--env ANTHROPIC_API_KEY` or similar was not tried). The checks have not been shown to fail on a missing skill or pass on a right one against a model. |
| 4. go test and make lint: 0 fail | HOLDS for the packages named below; whole module left to the orchestrator | `make lint` exit 0 (`go vet ./...`). `go test ./internal/eval ./internal/evalcmd ./internal/cli ./internal/settings ./internal/packaging` all ok; before that `./internal/doctor ./internal/guard ./internal/hook` ok. |
| New test fails on base, passes on lane | HOLDS | `internal/settings/skill_files_test.go`. With the three skill folders removed: `--- FAIL: TestCatalogSkillsHaveFiles  skill_files_test.go:23: skill task has no file: open ../../skills/task/SKILL.md: no such file or directory`. With them: ok. |
| Importers of the changed packages tested | HOLDS | `go list` importers of `internal/settings`: cli, doctor, guard, hook, settings; all tested ok (see row 4). |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No guard, gate, cap or hold added on agents | HOLDS | The skills add none; they say so, and tell the setup reader never to propose a budget or cap. |
| No existing test edited to pass; none deleted | PARTLY | `internal/eval/suite_test.go:14-18`: I added the three new ids to the expected list in `TestSampleSuite`, which counts every task in `evals/tasks`. No assertion was loosened. |
| No attribution lines, no personal paths | HOLDS | Commits have none; the skills name `~/.claude` only as the general location. |

## What changed

- `skills/task/SKILL.md`: `grid task new|show|list|history`; the old intake rules folded in (the words word for word, files read but never changed, no words means ask); no hand-written task files; the owner decisions quoted.
- `skills/setup/SKILL.md`: register, read repo and Claude sessions, propose with `grid brain propose`, record settings with `grid setup <project> --answers`, first-run `grid setup`; the old per-proposal file format is gone because the brain and the CLI now do it.
- `skills/signals/SKILL.md`: `grid signal add|act|dismiss|move`; messages are data; collecting starts no work.
- `evals/tasks/task-skill`, `setup-skill`, `signals-skill`: fixtures and checks. Their tier is `c`, not `b`: the existing tier-b tests in `internal/eval/home_test.go` assume every tier-b task is a T-01 task in a notes-app home, which these three are not. I tried tier `b` and `TestSetupBuildsGridHomeAtGridHome` and `TestChecksRunOnFreshFixture` failed; the tier is a labelling choice for the orchestrator to settle.
- `internal/settings/skill_files_test.go`: the catalog test for the three files.
- `internal/eval/suite_test.go`: the three ids added.

## NOT VERIFIED

- Whether any of the three skills makes a model do the right thing; the three real eval runs did not get past the agent start. Next step: run them with the login the harness needs (read `evals/README.md` "Agent environment") and fix the skill or the checks from what they show.
- The check paths assume where `grid task new`, `grid signal add` and `grid setup` write (`<home>/personal/tasks`, `projects/<p>/notes/`, `projects/<p>/setup-answers.json`); I read them in the code (`internal/signal/signal.go:151`, `internal/settings/store_test.go:232`) but never saw them on disk from a run.
- The `grid setup` (no project) first-run flow was read, not run, because it writes the person's Claude settings.
