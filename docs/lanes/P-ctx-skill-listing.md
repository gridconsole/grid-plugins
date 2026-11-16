# P-ctx-skill-listing: Grid-launched agents list only the skills their Job needs

Repo: plugin. Branch lane-P-ctx-skill-listing, base main `6cc26809` (`git merge origin/main`: already up to date).

## Proof table

| Row | State | Proof |
| --- | --- | --- |
| 1. Decide and write down | HOLDS | `docs/decisions/D-ctx-skill-listing.md`. Agents Grid starts get a per-run `claude --settings <inline JSON>` layer with `disableBundledSkills: true` and `skillOverrides` set to `name-only` for each of the person's own skills: `<home>/.claude/skills/*/SKILL.md` (folder name and front matter `name`), `<home>/.claude/commands/*.md`, and the skills in `.claude/skills` of the folder the agent starts in. The skills a Job or step names are passed as `keep` and stay fully listed. No settings file is written, so the person's own sessions are unchanged (pinned by `TestCtxRoutineSkillLayerInArgv`, which stats the four settings files). Plugin skills are not reachable by `skillOverrides` (docs), so row 2 cuts them. Workflow routines run `grid task new`, not claude, and their Jobs' agents are not started by the plugin today; the decision says those launchers pass the same layer with `keep` = the Job's `Skills:`. |
| 2. Plugin skills, person-facing descriptions | HOLDS | `skills/setup/SKILL.md:3` (311 bytes on base), `skills/task/SKILL.md:3` (249), `skills/questions/SKILL.md:3` (223), `skills/signals/SKILL.md:3` (302): each now at most 200 bytes (1,085 bytes together on base). `TestCtxPersonSkillDescriptionsShort` (`ctx_descriptions_test.go`) holds each at ≤200 bytes. On base, with the skills stashed, `go test -count=1 -run TestCtxPersonSkillDescriptionsShort .` exits 1: `setup: description is 311 bytes, want at most 200`, `task: description is 249 bytes`, `questions: description is 223 bytes`, `signals: description is 302 bytes`. On the lane it passes, with `TestErDescTwoSentences` and `TestErDescWalkEveryFile` (`go test -count=1 -run "TestCtx\|TestErDesc" -v .`, exit 0). No `disable-model-invocation` and no `paths:`, as the decision says: setup and task are called in plain words, not only `/name`; agents use questions (`skills/research/SKILL.md:22`); a routine has Claude load signals; and none is tied to a file pattern. |
| 3. Launch side for internal/routine, test fails on base | HOLDS | `internal/routine/skill_layer.go` `SkillLayer`; `internal/routine/execute.go:201-210` appends `--settings <layer>` to a skill routine's claude argv when the executor has the person's Home (`grid routine run` always passes it, `internal/cli/routine.go:185`). Tests are in `internal/routine/ctx_skill_layer_test.go`. Base without the layer code, `go test -count=1 -run TestCtx ./internal/routine/` exits 1: `internal/routine/ctx_skill_layer_test.go:85:14: undefined: SkillLayer`. With `SkillLayer` added but the argv not yet wired, it exits 1: `--- FAIL: TestCtxRoutineSkillLayerInArgv ... argv [... "--permission-mode" "dontAsk"] want [... "dontAsk" "--settings" "{\"disableBundledSkills\":true,\"skillOverrides\":{\"alpha\":\"name-only\",\"beta\":\"name-only\",\"gamma\":\"name-only\",\"proj\":\"name-only\"}}"]` and `--- FAIL: TestCtxRoutineSkillLayerWithoutOwnSkills ... want --settings {"disableBundledSkills":true}`. On the lane, `go test -count=1 ./internal/routine/` exits 0. |
| 4a. make test | HOLDS | `make -C . test` exit 0: 72 `ok` lines, no FAIL (including `internal/routine`, `internal/cli` 336s, `internal/orchestrator` 524s). An earlier run that overlapped a second `make test` and a separate importer run failed on timing in packages that import nothing this lane changed: `internal/env` `TestUpHealthTimeoutHonorsStopGrace`, `internal/eval` `TestFailuresAreSaved/agent` and `TestTraceComesFromTheFullStdout` (`context deadline exceeded` at a 3s agent timeout), and `internal/orchestrator` (`panic: test timed out after 10m0s`). Run alone on the same head, `go test -count=1 ./internal/env ./internal/eval` exit 0 and `go test -count=1 -timeout 20m ./internal/orchestrator` exit 0 (445s), and the clean `make test` above passed them all. |
| 4b. make lint | HOLDS | `make -C . lint` exit 0 (`go vet ./...`, gofmt clean). |
| 4c. make dist | HOLDS | `make -C . dist` exit 0: grid 0.2.3569 built for darwin/arm64, darwin/amd64, linux/amd64, linux/arm64 and windows/amd64. Afterwards `git status --short` shows no tracked change. |
| Tests of changed packages and importers | HOLDS | Changed: `.` (root, embeds skills), `internal/routine`. Importers from `go list -f '{{.ImportPath}} {{.Imports}} {{.TestImports}} {{.XTestImports}}' ./...`: `cmd/grid`, `internal/cli` (routine and root); `internal/brain`, `internal/braincmd`, `internal/contract`, `internal/job`, `internal/job/jobcmd`, `internal/routine`, `internal/settings` (root). `go test -count=1 . ./cmd/grid ./internal/cli ./internal/routine ./internal/brain ./internal/braincmd ./internal/contract ./internal/job ./internal/job/jobcmd ./internal/settings` exit 0, every package `ok` (`cmd/grid` has no test files). |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0. |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` → `ok internal/cli`, exit 0; `go test -count=1 ./internal/job/lint/` → `ok internal/job/lint`, exit 0. Both run after the merge of origin/main, before the last proof run. |
| fetch + merge origin/main before last proof run | HOLDS | `git fetch origin` then `git merge origin/main`: `Already up to date.` |
| No existing test edited | HOLDS | `git diff origin/main --stat` lists only new test files (`ctx_descriptions_test.go`, `internal/routine/ctx_skill_layer_test.go`). `TestProcessRoutineLimits` and `TestR23RoutineWithoutToolsKeepsItsArgv` still pin the argv without a Home unchanged and pass. That follows the `tools` precedent of lane P-r23-signals-routine: a routine without the new input keeps its argv. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The layer only uses Claude Code built-ins (`disableBundledSkills`, `skillOverrides` `name-only`). Nothing is turned off: a name-only skill stays callable by name. The 200-byte bound is a test on description text, not a limit on agents. |
| No attribution, no personal paths in files | HOLDS | Commits carry one-sentence messages and no trailers. The new files hold no host, user path or email. |
| Cross-repo set, whole-module run at landing | not run, not asked | Run by the orchestrator at landing. |
| RealPlugin tests | not run, not asked | The brief names none. |

## What changed

- `internal/routine/skill_layer.go` (new): `SkillLayer(home, dir, keep)` builds the inline `--settings` JSON.
- `internal/routine/execute.go`: a skill routine's claude argv ends in `--settings <layer>` when the executor has a Home.
- `skills/{setup,task,questions,signals}/SKILL.md`: shorter descriptions. The text below the front matter is unchanged.
- Tests: `internal/routine/ctx_skill_layer_test.go`, `ctx_descriptions_test.go`.
- Docs: `docs/decisions/D-ctx-skill-listing.md`, this file.

## NOT VERIFIED

- The listing size of a real `claude -p` routine run with the layer, and that Claude Code accepts the inline layer as written. No `claude` was run on the VM.
- On Windows with a `claude.cmd` shim, the quoting of the inline JSON (see the decision's last item).

## Open items

- The launch side for a Job's agent (the app, or a plugin worker if one is added) should call `SkillLayer` with `keep` set to the Job's `Skills:`. Out of scope here, since the plugin does not start Job agents today.

DONE
