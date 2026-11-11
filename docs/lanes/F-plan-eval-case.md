# Lane F-plan-eval-case: plan-quality eval cases

Two cases in the evals format: `plan-quality-small` and `plan-quality-normal`. Each seeds a Grid home and a tiny `notes-app` repo with `setup.sh`. The agent writes `plan.md` in the task folder and `checks.json` reads that file. The test also runs each case end to end with a scripted agent that writes a good or a bad plan.

## Proof table

| Row / rule | Result | Proof |
| --- | --- | --- |
| 1. Fields quoted | HOLDS | `evals/README.md:87-97`: "`task.json` has these required fields" with `schema_version`, `id`, `title`, `tier`, `size`; `evals/README.md:120-129` and the type table at `:131-136`: "`checks.json` is a nonempty array" of `id`, `type`, `path`, `text`, `command`, `rubric`, `weight`, `gate`. Existing case: `evals/tasks/clean-csv/task.json:1` (`{"schema_version":1,"id":"clean-csv",...,"tier":"c","size":"small"}`), `evals/tasks/clean-csv/checks.json:2-4` (a `file_exists` with `gate`, a `command_passes` with `weight`, a `text_contains` with `path` and `text`). |
| 2. Two cases, setup builds fixture | HOLDS | `evals/tasks/plan-quality-small/` (one-file `src/title.js`, task T-02, no `research.md`; the plan must have no research step) and `evals/tasks/plan-quality-normal/` (four-file repo, task T-03 with `scout.md` and `research.md`; the plan must cite `research.md`, have `## Options` with a `Chosen:` line, `## Risks`, and a `needs:` line on every step). Both `setup.sh` build the repo under `work/notes-app` and the task folder under `$GRID_HOME`. `go test ./internal/eval -run 'TestSetupBuildsGridHomeAtGridHome/plan-quality' -count=1`: exit 0 (part of the package run below). |
| 3. Checks are file reads only | HOLDS | Every check is `command_passes` using only `test`, `grep`, `ls`, `tr` on workspace and `$GRID_HOME` files; no `llm_judge`, no `claude`, no network. `TestPlanQualityCasesAreReadChecksOnly` (`internal/eval/plan_quality_test.go`) refuses `llm_judge`, `claude`, `curl`, `wget`, `http://`, `https://`. Sections, `needs:` lines (`checks.json` id `needs-lines`), research cited (`research-cited`), Recall or Used line (`recall-line`). |
| 4. Test validates both cases; `go test ./internal/eval/...` passes | HOLDS | `go test ./internal/eval/... -count=1`: exit 0 (`ok .../internal/eval 22.285s`). Go test, as the existing cases use (`internal/eval/cases_test.go`). The existing `TestEveryShippedCaseIsSound` audits both new folders and `LoadSuite` loads them. New `internal/eval/plan_quality_test.go` runs setup, a scripted agent and the checks: 4 good/bad plans for small, 6 for normal, plus a missing-plan gate test. |
| New tests fail on base | HOLDS | With `evals/` stashed (the base): `go test ./internal/eval/ -run PlanQuality -count=1` exit 1. Lines: `plan_quality_test.go:105: case plan-quality-small is not in evals/tasks`; `--- FAIL: TestPlanQualityChecksSeparateGoodPlansFromBad/normal_good ... case plan-quality-normal is not in evals/tasks`; `--- FAIL: TestPlanQualityMissingPlanFailsTheGate`. |
| 5. `go test ./...`, `make lint` | HOLDS | `go test ./... -count=1`: exit 0, every package `ok`. `make lint`: exit 0 (`go vet ./...`, gofmt clean after `gofmt -w internal/eval/suite_test.go`). |
| Packages that import the changed one | HOLDS | Changed: `internal/eval` (test files only). Importers from `go list`: `internal/evalcmd`, `internal/land`, and `internal/cli` (through evalcmd). `go test ./internal/eval/... ./internal/evalcmd/... ./internal/land/... ./internal/cli/... -count=1`: exit 0 for evalcmd, land and cli; eval ok after the pin bump below. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...`: exit 0. `GOOS=darwin CGO_ENABLED=0 go build ./...`: exit 0. |
| No guard, gate, cap or hold on agents | HOLDS | Only eval cases and tests; no hook or Grid code touched. The `gate: true` on `plan-file` is the eval harness's own gate (a missing plan file), not an agent gate. |
| Existing tests not edited to pass | HOLDS, with two count pins | Two existing tests pin the number of cases and had to follow the new folders: `internal/eval/home_test.go:34` (`len(out) != 19` became `21`, "tier-b cases that are not fake-agent samples") and `internal/eval/suite_test.go:19-20` (the expected id list gained the two ids). No assertion was weakened. Nothing deleted. |
| No personal paths, hosts or emails | HOLDS | `grep -rIn -E "/home/|/srv/|@|grid2" evals/tasks/plan-quality-small evals/tasks/plan-quality-normal internal/eval/plan_quality_test.go`: no output (exit 1, no match). Fixture uses the sample name `alex` as the other cases do. |
| Bundle | see end | `/srv/gridwork/tmp/hand/F-plan-eval-case.bundle` and `-report.md`. |

## What changed

- `evals/tasks/plan-quality-small/` and `evals/tasks/plan-quality-normal/` (`task.json`, `prompt.md`, `setup.sh`, `checks.json`), both tier b.
- `evals/README.md`: a paragraph on the two cases, before "`make test` includes every sample".
- `internal/eval/plan_quality_test.go` (new): the schema and read-only checks, plus the good/bad plan runs.
- `internal/eval/home_test.go:34` and `internal/eval/suite_test.go:19-20`: the two count pins.

Notes for the orchestrator:

- The prompt says "use `grid plan write` when the verb is available, otherwise write the file", because the eval environment has no `grid` on PATH by default. The checks read `plan.md` either way.
- `## Options` and `Chosen:` are this case's own plan shape. `agent-templates/grid-planner.md` names the other sections (`## Where it stands`, `## How we approach it`, `## Risks`, `## Steps`, `## Verify`) but not an options section, so the plan-writing prompt in the case states it. If the planner template should require `## Options`, that is a separate brief.
- `needs-lines` accepts `none` or step numbers (`1`, `1, 2`); it does not check that the numbers name an earlier step.

## NOT VERIFIED on a live screen

- A real model run (`grid eval run plan-quality-normal --agent '["claude","-p"]' --env ANTHROPIC_API_KEY`). The checks are proven only against scripted plans. I ran neither `claude` nor the network.
- Whether a real planner follows the `Chosen:` line or the exact `needs:` indent. The orchestrator's live run shows it.
