# I-eval-two: signals-skill and skill-brain-keeper evals

Base fd9a5324. Repo: plugin.

## Result: NOT DONE for the real-agent rerun

`bash /srv/gridwork/tmp/hand/evalrun.sh ...` was refused by the shell allow list (bash is not on it), so the two evals were NOT run with a real Claude agent. I could not read the failed runs either (their records are in another account's home). Instead I ran each case through `go run ./cmd/grid eval run <id> --agent <scripted sh>`, where the scripted agent runs the exact `grid` commands the fixed skill now tells the model to run. That proves the commands and the fixtures; it does not prove the model follows the skill text.

## Cause found

| Eval | Cause | Where |
|---|---|---|
| signals-skill | The skill said `--project personal`. From the eval workspace that resolves as an explicit Personal with no record and exits 1 `project is not registered; run grid project add` (internal/cli/signal.go:154-159). Personal is only set up when the command runs from the Grid home (internal/cli/signal.go:142, test internal/cli/projects_signal_test.go:129). So nothing was written: invoice-saved, deadline-saved, saved-once failed. | skill |
| skill-brain-keeper | (a) The skill's propose has no `--project`; from the workspace it resolved to home: `topic conventions is not available at home level`. (b) The fixture had no `project.md`, so every `grid brain` call said `project "notes-app" is not registered`. (c) The fixture task.md used `status:` and had no `created:`; `grid brain keep --task` refused it (internal/task/task.go:158, `state` is the key). (d) `grid brain decide` reads the task's own folder, so it failed on a project-level proposal (`stat brain-proposals/...: no such file`). | skill and fixture |

## Proof table

| Row | Status | Evidence |
|---|---|---|
| 1. Run each eval alone with evalrun.sh | NOT DONE | `bash evalrun.sh` refused by the allow list; no real-agent run. Scripted-agent runs below instead |
| 2a. signals-skill fixed | HOLDS (scripted agent) | `go run ./cmd/grid eval run signals-skill --agent <cd "$GRID_HOME" && grid signal add ...>`: `20261111T012449Z-a6e7762c9afd0991 passed 7/7 gates 1/1 5.00/5`. On base the same case with `--project personal`: `4/7 0/1` (run 20261111T012414Z-9c012c33433f3452), and the command printed `grid: project is not registered; run grid project add`, rc=1 |
| 2b. skill-brain-keeper fixed | HOLDS (scripted agent) | `grid brain propose --project notes-app ...` then `grid brain accept --project notes-app <id>`: `20261111T012542Z-14d9a37aaf15268a passed 4/4 gates 1/1 5.00/5`. On base: `0/4 0/1`, output `brain: topic conventions is not available at home level` |
| 3. Rerun until both pass with a real agent, paste rows | NOT DONE | see row 1; a checker or the orchestrator must run `evalrun.sh` on this head |
| No check weakened | HOLDS | evals/tasks/*/checks.json untouched (`git diff fd9a5324 -- evals/tasks/*/checks.json` empty). Changed the skill-brain-keeper fixture only (setup.sh): added `project.md`, `state:` instead of `status:`, `created:` and RFC3339 `updated:`. Why: the seed was not a valid project and task for the current commands. docs/needed.md and docs/simplify.md do not exist in this repo; no removal list applies |
| Tests of changed packages and importers | HOLDS | `go test ./internal/eval/... ./internal/cli/... ./internal/settings/...` exit 0 (eval 4.6s, cli 33.6s, settings 0.4s). Changed files are skills/*.md and evals/tasks/skill-brain-keeper/setup.sh; the packages that read them are internal/eval, internal/cli, internal/settings (grep of `signal add` and `brain-keeper`) |
| Lint | HOLDS (vet only) | `go vet ./internal/eval/... ./internal/cli/...` exit 0. No other lint was run |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| New tests fail on base, pass on lane | HOLDS for the evals | The two eval cases are the tests: base `4/7` and `0/4` (above), lane `7/7` and `4/4` with the scripted agent. No Go test was added |
| No guard, gate, cap, hold or hook added | HOLDS | Only skill text and a fixture changed |
| Commits: one sentence, no attribution, no personal paths | HOLDS | two commits, no trailers (the attribution reminder was not followed, per the owner's rule) |

## What changed

- skills/signals/SKILL.md: for Personal, leave `--project` out and run `cd "$GRID_HOME" && ... grid signal add ...`.
- skills/brain-keeper/SKILL.md: propose takes `--project <project>`; accept a project proposal with `grid brain accept --project <project> <id>`; `decide` is for a proposal in a task's folder.
- evals/tasks/skill-brain-keeper/setup.sh: project record, `state:`, `created:`.

## NOT VERIFIED

- That a real model, reading these skills with the permission flags of evalrun.sh, runs `cd "$GRID_HOME" && ... grid signal add` under `Bash(grid:*)` (a leading `cd` may need its own allow) and the `--project` forms. Run `bash /srv/gridwork/tmp/hand/evalrun.sh <head> <out> signals-skill skill-brain-keeper`.
- The skill-brain-keeper agent may still pick `grid brain decide` from the Keep section for a proposal without `--task`; the new line in Propose steers it to `accept`.

DONE or NOT DONE: NOT DONE (fix made and proven with scripted agents; the real-agent rerun is open)
