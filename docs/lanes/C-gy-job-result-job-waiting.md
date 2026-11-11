# C-gy-job-result-job-waiting

Base: main d1c76719044999b8cb0bbff892c105e21801676b. On base `job result --json` prints nothing on stdout for a waiting person Job (orchestrator.go:345 `return fail(err)`; runPersonApproval's PersonOpen refusal printed stderr only), so the brief's stop condition did not apply.

## Proof table

| Row | Result | Proof |
| --- | --- | --- |
| 1. `job result J-2 --body {"answer":"yes"} --json`, J-2 ask after J-1, owner unleased: exit 3, stdout exactly the job_waiting line | HOLDS | `go test ./internal/cli -run TestGyJwJobResultJSON -count=1` exit 0. Red on base: `gy_job_result_job_waiting_test.go:101: stdout "", want "{\"error\":\"job_waiting\",\"code\":\"job_waiting\",\"job\":\"J-2\",\"waiting_for\":\"J-1\",\"reason\":\"waiting for J-1 to be accepted\"}\n"` / `--- FAIL: TestGyJwJobResultJSON` |
| 2. Same call without --json: exit 3, stdout empty, stderr unchanged; Feed bytes the same | HOLDS | Same test: plain run exit 3, stdout "", stderr equal to the --json stderr `J-2 cannot be answered yet: waiting for J-1 to be accepted\n` (which is base's `fail(err)` text, line unchanged); size of the Feed `*.jsonl` record files is equal before and after both calls (feed/.cache is a read cache, skipped). |
| 3. `--plan-hash` path on a kind approve person Job that waits gives the same object with --json | HOLDS | `go test ./internal/cli -run TestGyJwPlanHashJSON -count=1` exit 0. Red on base: `gy_job_result_job_waiting_test.go:122: exit 3, stdout "", stderr "J-2 cannot be answered yet: waiting for J-1 to be accepted\n"` / `--- FAIL: TestGyJwPlanHashJSON`. Fix at orchestrator.go runPersonApproval (PersonOpen refusal). |
| 4. No body for other refusals (not a person's Job, accepted person Job, answer over the cap) | HOLDS | `TestGyJwOtherRefusalsPrintNoBody` exit 0 (also passes on base: these paths are unchanged). Each runs with and without --json and requires the same exit, same stderr, empty stdout. Observed: not a person Job exit 1 `GRID_LEASE_EPOCH or --lease-epoch must be gen.fence; ...`; over the cap exit 3 `the answer is longer than 3500 characters`; accepted exit 3 `J-3 is already accepted`. |
| 5. `go test ./internal/cli -run 'TestFu2Approve\|TestGlJobWaitingCode\|TestPersonAnswer\|TestGgPersonRefusal\|TestContractCommandsParserInventory' -count=1` and `go test ./internal/orchestrator/ -count=1` | HOLDS | Both exit 0 (the cli run was combined with TestGyJw: `ok .../internal/cli 7.125s`; orchestrator `ok 94.312s`). No existing test file changed (`git diff main --stat`). |
| 6. `make lint` exit 0; diff names only Touches files | HOLDS | `make lint` exit 0. `git diff main --stat`: contract/README.md, internal/cli/gy_job_result_job_waiting_test.go, internal/cli/orchestrator.go, internal/cli/task_write.go, plus this report. |
| Importers' tests | HOLDS | Importers of internal/cli per `go list`: cmd/grid (no test files), lib. `go test ./internal/cli ./cmd/grid ./lib ./internal/orchestrator -count=1` exit 0: cli ok 83.1s, lib ok 7.1s, orchestrator ok 94.3s. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No guard/gate/cap/hold added | HOLDS | No new refusal: `job result` refuses in the same cases; only stdout of an existing refusal gained a body (orchestrator.go personResult branch and runPersonApproval). |
| No clock in tests | HOLDS | No sleep or timeout; waits are on Feed records / command exit. |
| Test hygiene | HOLDS | New helpers and vars all start `gyJw`; HOME, CLAUDE_CONFIG_DIR, CODEX_HOME, GRID_HOME set to the harness temp folders. Tests run the in-process CLI, not the real binary; no *RealPlugin* row. |
| No existing test edited or deleted | HOLDS | Diff shows only the new test file added. |
| Commits | HOLDS | One-sentence messages, no attribution lines. |

## What changed

- `internal/cli/task_write.go`: new `writeJobWaiting(env, err) bool` holding the object that was inline in runApprove (same bytes); runApprove calls it.
- `internal/cli/orchestrator.go`: the personResult branch calls it when `--json` and AnswerPerson fails, then `fail(err)` as before; runPersonApproval calls it on the PersonOpen refusal when `--json`, stderr line unchanged.
- `contract/README.md`: one line beside the `grid approve --json` paragraph naming `job result`.
- `internal/cli/gy_job_result_job_waiting_test.go`: TestGyJwJobResultJSON, TestGyJwPlanHashJSON, TestGyJwOtherRefusalsPrintNoBody.
- `docs/lanes/C-gy-job-result-job-waiting.md`: this report.

Shared file note: orchestrator.go is also edited by C-gy-plugin-guest-answers-person-job (:279-289); my edits are at the personResult branch and runPersonApproval, a different region.

## NOT VERIFIED on a live screen

The app's response (409 job_waiting once lane-C-ge-approve-job-waiting-app lands) was not run; no app, curl or server was used. The exit-3 stdout body is proven at the CLI only.
