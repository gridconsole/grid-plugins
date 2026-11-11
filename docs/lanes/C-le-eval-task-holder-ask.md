# C-le-eval-task-holder-ask

Eval case `orchestrator-asks-question`: the orchestrator Job (GRID_LEASE=J-1, task epoch set) meets a choice only the owner can make and asks with `grid question ask`; it posts no hand-made person Job.

## Wait check

`git -C /srv/gridwork/ref/plugin.git merge-base --is-ancestor lane-C-ku2-question-job-lease-ask main` exit 0 before the work started. Lane cut from main a7809a58.

## Proof table

| Row | State | Proof |
| --- | --- | --- |
| 1 `TestLeEqShape` | HOLDS | `go test -count=1 -v -run TestLeEq ./internal/evalcmd/` exit 0: `--- PASS: TestLeEqShape` (internal/evalcmd/leeq_case_test.go:71) |
| 2 `TestLeEqGood` | HOLDS | same run: `--- PASS: TestLeEqGood`; `question-asked passed=true (Bash called 1 times, need at least 1)`, `no-person-job passed=true (... at most 0)`; the llm grader `later-job-waits` is the only NotGraded |
| 3 `TestLeEqBad` | HOLDS | same run: `--- PASS: TestLeEqBad`; `question-asked passed=false (Bash called 0 times, need at least 1)`, `no-person-job passed=false (Bash called 1 times, ... at most 0)` |
| 4 evalcmd package | HOLDS | `go test -count=1 ./internal/evalcmd/` exit 0 |
| 5 diff and builds | HOLDS | `git status --short`: only the case folder, `leeq_case_test.go`, `testdata/leeq/`, plus this report; `make lint` exit 0; `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile, all ok) |
| New tests fail on base | HOLDS (by absence) | on main neither the test file nor the case folder exists, so the three tests do not exist there; with the folder missing `GradeCase` returns an error and the tests fail. A mutation check was not run (no `mv` in the allow list). |
| Importers of evalcmd | HOLDS | `go list` shows one importer, `internal/cli`; `go test -count=1 ./internal/cli` exit 0 |
| Walk: plugin | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0 |
| Walk: app-only (webui, plugincli, grid-server, internal/slices) | not applicable | this is the plugin repo; `internal/slices` does not exist here |
| gofmt | HOLDS | `gofmt -l internal/evalcmd` prints nothing |
| No guard on agents, no production code | HOLDS | only eval case files, one test file and its transcripts changed |

## What changed

- `evals/suite/orchestrator-asks-question/`: `case.yaml` (1.1), `fixture.sh` (task T-01 whose intent leaves the export format, CSV or XLSX, to sam; J-1 is the orchestrator Job), `prompt.md` (live, runs 1, max_turns 25), and three graders:
  - `question-asked` (`tool_used` Bash `grid question ask`, min 1)
  - `no-person-job` (`tool_used` Bash `grid job post.*--person`, max 0)
  - `later-job-waits` (`llm`: a later build Job names the person Job or `gate:Q-<n>` in its `after`)
- `internal/evalcmd/leeq_case_test.go`: `TestLeEqShape`, `TestLeEqGood`, `TestLeEqBad`, using `GradeCase` (helpers `leEq*`).
- `internal/evalcmd/testdata/leeq/good.jsonl` and `bad.jsonl`.

## Notes

- Graders carry no `name:` key. `GradeCase` rejects it on a graded type (`tool_used grader has unknown key "name"`); the kq cases omit it too. The `job-result-posted` graders do carry it, so `GradeCase` would reject those; not touched here.
- `no-person-job` only catches the `--person` flag; a person Job posted by body with role Person is not matched.

## NOT VERIFIED

- The case has not been run live (`claude plugin eval` is outside this lane), so whether a real orchestrator asks and the `llm` grader's verdict are NOT VERIFIED.
