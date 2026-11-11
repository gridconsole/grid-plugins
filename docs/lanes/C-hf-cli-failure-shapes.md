# C-hf-cli-failure-shapes

Base: lane-C-hf-cli-failure-shapes at 7e95373b (includes C-gy-job-result-job-waiting, which kept its README line as shape 4). README:7-53 on that base and `internal/cli/error_codes.go`, `error_codes_test.go:16-75` were read before the change.

## Proof table

| Row / rule | Result |
| --- | --- |
| 1. TestHfFsCodeTable: README table equals the codes collected by go/parser both ways | HOLDS. `go test ./internal/cli -run 'TestHfFs\|TestStableErrorCodes' -count=1` exit 0. Failing on base README (`git checkout 7e95373b -- contract/README.md`): `README lists code "linked_file" that no Go file prints`; `code "job_waiting" (task_write.go:literal) is missing from the README code table`; same for `changed`, `nothing_to_undo`, `invalid_settings`, `write_failed`, `unconfirmed_key`. Literals found by struct-literal `Code` field in role_policy.go, person_check.go, task_write.go (test fails if a file has none). |
| 2. TestHfFsShapes: `task show T-99` (exit 1), `signal act <missing> --project example` (exit 3), `signal move <id>` (exit 2, invalid), key sets equal README examples | HOLDS, same run. Added two shapes the brief lists but did not name a command for: `person check --json` with GRID_ROLE=builder (exit 3) and the job_waiting object via `writeJobWaiting` with an `orchestrator.WaitingError` (no live approve; the plan fixture is outside this lane). Failing on base: `README has no example for shape "envelope"`. |
| 3. TestHfFsSilent | HOLDS. Command `task show --json` outside a task folder, from `TestTaskUsageAndFailures` (task_test.go:258): exit 1, empty stdout, stderr set. Also reads the README sentence "prints nothing on stdout". Failing on base: `README does not say that an unclassified failure prints nothing on stdout`. |
| 4. `linked_file` in README | HOLDS. `grep -n linked_file contract/README.md` returns nothing; no Go file prints it (it appears only in docs/lanes/FP1.md and S6b.md). The row and its hard-link prose were removed, since the code is not printed on this base. |
| 5a. `go test ./internal/cli -run 'TestHfFs\|TestStableErrorCodes' -count=1` | HOLDS, 0 fail (exit 0). |
| 5b. `make lint` | HOLDS: runs `go vet ./...`, exit 0, no output. `gofmt -l internal/cli` empty. |
| 5c. diff names only the two Touches files | HOLDS apart from this report: `git diff 7e95373b --stat` = contract/README.md, internal/cli/hf_failure_shapes_test.go (plus docs/lanes/C-hf-cli-failure-shapes.md, which the runbook requires). `git diff main` would also list the C-gy lane, already merged here. |
| Tests of changed packages and importers | `go list` importers of internal/cli: cmd/grid, lib. `go test ./internal/cli ./cmd/grid ./lib ./contract/... ./internal/contract -count=1`: all ok (cmd/grid, contract: no test files). The first run of internal/contract failed `TestGoReadmeSubverbsInCommandsJSON` on my README text `grid agent budget undo` (budget is not in commands.json); I reworded the doc, then `go test ./internal/contract -count=1` exit 0. |
| Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| New tests fail on base, pass on lane | HOLDS, lines above. TestHfFsKnownOpen does not read the README, so it passes on base by design. |
| No existing test edited or deleted; no production file touched; no guard/gate | HOLDS. |
| Tests use a temp home, no clock, no sleep | HOLDS (harness `newHarness`/`actFixture`, t.TempDir). |
| *RealPlugin* tests | Not applicable: no row names one. |

## What changed

- contract/README.md: "CLI JSON failures" is now four numbered shapes (envelope; signal object with categories `refused` and `invalid` and a code present only when classified; person gate object; job_waiting object), each with a tagged example (`<!-- shape:NAME -->` before the json fence) that the test reads; one paragraph on when nothing is printed; a code table of exactly the 12 printable codes: the five `Code*` constants, `person_required`, `job_waiting` and the five settings codes. `linked_file` is removed.
- internal/cli/hf_failure_shapes_test.go: TestHfFsCodeTable, TestHfFsShapes, TestHfFsSilent, TestHfFsKnownOpen.

## Known open (code differs from what a reader would want; no behaviour changed)

- A refused signal object prints no `code` when the refusal is unclassified: signal.go:238 passes `errorCode(err)`, which is "" at error_codes.go:44-45 (e.g. `signal act --act wake` on a new note: "this note is not snoozed"; `signal move` to the same project: "the note is already there"). The README states the code is optional there. TestHfFsKnownOpen lists both probes and fails if a listed one gains a code or an unlisted refusal lacks one.
- Some commands print their own older JSON failure (`job show` prints `{"ok":false,...}`, `brain show` prints `{"error":"..."}` on stderr); the README says such objects carry no code. Not probed by the test.
- The five settings codes are reached through `commandError` for `grid agent budget` and `grid claude settings`; the table rows are matched to the Go set, but no test runs those commands.
- On the Windows run no test was executed, only the build.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: a live `grid approve --json` / `grid job result --json` waiting job (the job_waiting shape is checked through `writeJobWaiting` only).
