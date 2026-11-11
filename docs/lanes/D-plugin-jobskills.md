# D-plugin-jobskills proof table

Base: main eb833a2d. Branch lane-D-plugin-jobskills.

| Row | Status | Proof |
| --- | --- | --- |
| 1. One naming scheme | HOLDS | The catalog follows the folders (`plan-jobs`, `review-job`, `run-job`). Why: `agents/*.md` and `agent-templates/*.md` already load those names, and the catalog's `job`, `orchestrate`, `plan` had no folder (review had no catalog name at all). Changed: `internal/settings/catalog.go:29`, `internal/cli/testdata/settings/skills.json:1`, `docs/settings.md:48`. `go test ./internal/settings/... ./internal/cli/...` exit 0. |
| 2a. plan-jobs carries the 10 Nov rules as pre-post checks | HOLDS | `skills/plan-jobs/SKILL.md:18-24` (item lists, `path:line` read this session, UI data source, widths 390/1440/3840, one budget number and split at about 60 calls, proof-table hand-off, verify); owner decisions quoted at `:8-14`; verbs checked with `go run ./cmd/grid job --help`, `proof --help`. `agents/orchestrator.md:22` runs them. |
| 2b. run-job carries the proof table hand-off | HOLDS | `skills/run-job/SKILL.md:19-25` (HOLDS with command and exit code or path:line, or NOT DONE; NOT VERIFIED). |
| 2c. review-job reviews against the proof table, rejects NOT DONE unread | HOLDS | `skills/review-job/SKILL.md:10-15`; `agents/reviewer.md:19`. |
| 3. One eval task per skill, real run pasted | NOT DONE (real run) | Tasks exist: `evals/tasks/job-plan`, `job-run`, `job-review`. The real command `grid eval run <id> --agent '["claude","-p","--plugin-dir",".../dist/plugin"]' --timeout 15m` after `make dist` exits 1 for all three: `agent: failed (exit 1)`, stdout `Not logged in · Please run /login`. The eval harness starts the agent with an empty environment and HOME is temporary; this builder has no `ANTHROPIC_API_KEY` to pass with `--env` (`CLAUDE_CODE_OAUTH_TOKEN` is refused by the harness). Result lines: `20261111T002631Z-a2be0223c188ca82 job-plan failed 0/7`, `20261111T002631Z-0ee16270f7fb4f16 job-run failed 0/6`, `20261111T002632Z-784aad8ea0ec9dad job-review failed 0/5`, all not measured. |
| 3b. Checks pass on a right answer and fail on a missing one (scripted agent, not a real run) | HOLDS | Scripted `sh` agent writing a conforming file: `job-plan passed 7/7 gates 1/1`, `job-run passed 6/6`, `job-review passed 5/5`. Agent `true` (no output, what a missing skill gives at worst): `job-plan failed 1/7 gates 0/1`, `job-run failed 1/6 gates 0/1`, `job-review failed 0/5 gates 0/1`, all exit 1. |
| 4a. go test, packages changed and importers | HOLDS | Importers of `internal/settings` (`go list`): `internal/cli`, `internal/doctor`, `internal/guard`, `internal/hook`. `go test ./internal/settings/... ./internal/cli/...` ok; `go test ./internal/doctor/... ./internal/guard/... ./internal/hook/... ./internal/eval/... ./cmd/...` ran in one call, the only failure was `TestSampleSuite`, fixed below, then `go test ./internal/eval/...` ok. The whole-module run is the orchestrator's. |
| 4b. make lint | HOLDS | `make lint` ran `go vet ./...`, no error output. |
| 4c. Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0, no output. |
| Rule: new tests fail on base, pass on lane | HOLDS (evals) | The evals fail with a no-op agent (3b) and pass with a conforming one. Go: no new Go test. |
| Rule: no existing test edited to pass | HOLDS with one edit | `internal/eval/suite_test.go:14-18` lists the sample suite's task ids and counts them; I added `job-plan`, `job-run`, `job-review` to that list. No assertion changed. `internal/cli/testdata/settings/skills.json` is the expected output of `grid skill list` and follows the catalog. |
| Rule: no guard, gate, cap or hold on agents | HOLDS | Only text in skills and agents; the pre-post checks are the orchestrator's own checklist. No hook or Go guard added. |
| Rule: no attribution lines, no personal paths | HOLDS | Commits carry one sentence each. |

## What changed

- Catalog skill names `job`, `orchestrate`, `plan` replaced by `plan-jobs`, `review-job`, `run-job`; docs and expected output follow.
- The three skills rewritten with the 10 Nov Job rules, the owner decisions, and verbs checked against the CLI: `grid job show|lint|cycles|floor|post|verify|result|tree`, `grid plan check`, `grid workflow show`, `grid brain propose`, `grid proof add`.
- `agents/orchestrator.md`, `builder.md`, `reviewer.md` name the checks, the proof table and the owner decisions.
- Three eval tasks under `evals/tasks/`.

## NOT VERIFIED

- Whether a real Claude run loads the three skills and produces a file that passes the checks: no login was available in the eval harness. To run it, export `ANTHROPIC_API_KEY` and add `--env ANTHROPIC_API_KEY` to the three commands in row 3.
- "Every link carries a why" is a one-line rule in `plan-jobs`, not a separate check with its own verb; `grid job lint` is what enforces the 8 to 160 character why on `after`, and I did not test that here.
- No live screen is involved in this lane.
