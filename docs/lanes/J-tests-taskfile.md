# J-tests-taskfile: unit tests for task.md frontmatter scalars and the workflow step reader

Repo plugin, branch lane-J-tests-taskfile. One new file: `internal/taskfile/taskfile_units_test.go`. No change to the code under test.

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. `ScalarFrontmatter`: quoted, unquoted, empty, missing keys | HOLDS | `TestScalarFrontmatterValues` (double and single quoted, bare, `# comment` stripped, `"issue #12"` kept, `empty:` and whitespace-only give `""`, inline list stays a string, list items ignored), `TestScalarFrontmatterMissingKey`, `TestScalarFrontmatterCRLFAndBody`, `TestScalarFrontmatterDuplicateKeyLastWins`. `go test -count=1 ./internal/taskfile/` exit 0. Code: frontmatter.go:57. |
| 1. `ScalarFrontmatter`: no frontmatter, closing `---` missing | HOLDS | `TestScalarFrontmatterNoBlock`: empty text, plain body, opening only (LF and CRLF), opening line alone, fence not at the start, indented fence all return nil. Exit 0. |
| 2. `WorkflowSteps` | HOLDS | `TestWorkflowStepsReadsInlineAndBlockLists`, `TestWorkflowStepsRefusesUnreadable` (13 cases: no block, no key, empty list, named string, non-map item, missing kind or step, bad step name, duplicate, one bad item drops all, odd key), `TestWorkflowStepsKeepsUnknownKind`. workflow.go:15. |
| 2. `PlanApprovalIndex`: no approve step gives -1; several approvals give the first plan approval | HOLDS | `TestPlanApprovalIndex` (15 cases, incl. nil, no approve, ship-only after run/work, approve first, named plan, ship then named plan gives the plan index, two named plans gives the first), `TestPlanApprovalIndexFromWorkflowText`. workflow.go:45. |
| 2. `PlanApprovalRequired` agrees | HOLDS | `TestPlanApprovalRequiredAgreesWithIndex` checks index and required together and that index >= 0 always gives required. `TestPlanApprovalRequiredUnreadableWorkflow` (7 cases) pins the documented default: missing, empty, named, malformed or unknown-kind workflow requires plan approval. workflow.go:65. |
| 3. `HistoryLine` with and without via | HOLDS | `TestHistoryLineVia` (8 cases): no via argument (with and without detail), via given, empty via left out, only the first via used, an empty first via hides a later one, via not trimmed or escaped. Matches the doc comment and line at frontmatter.go:152. |
| 4. `go test ./internal/taskfile/` 0 fail | HOLDS | `go test -count=1 ./internal/taskfile/` exit 0 (`ok`). |
| Importers' tests | HOLDS | `go list` of direct importers of taskfile: guard, hook, plan, task, workflow. `go test -count=1 ./internal/taskfile/ ./internal/guard/ ./internal/hook/ ./internal/plan/ ./internal/task/ ./internal/workflow/` exit 0, all `ok`. |
| Lint | HOLDS | `make lint` (`go vet ./...`) exit 0. `gofmt -l internal/taskfile` prints nothing. |
| Windows build | HOLDS | `GOOS=windows go vet ./internal/taskfile/` exit 0. |
| macOS build | HOLDS | `GOOS=darwin go vet ./internal/taskfile/` exit 0. |
| New tests fail on base | NOT APPLICABLE | The code is unchanged and the brief forbids changing it, so these tests pass on base too; they add coverage and do not fix a bug. One of my own first-run failures was an index miscount in a test case (wrong expected value), fixed in the test; the code was right. |
| No existing test edited or deleted | HOLDS | Only the new test file and this report were added. |
| No guard, gate, cap, hold or blocking hook added | HOLDS | Test file and report only. |
| Public repo hygiene | HOLDS | Inline strings only; no paths, hosts or emails. Commit messages carry no attribution lines. |

## Findings in the code under test

No bug. Observed and not changed, not pinned by a test: a closing `---` with trailing text (`---x`) still closes the block, because the `block` regexp (frontmatter.go:15) does not require a line end after the fence. An empty block (`---\n---\n`) returns nil for the same reason (the closing fence needs a preceding newline).

## What changed

- `internal/taskfile/taskfile_units_test.go`: ScalarFrontmatter table tests, WorkflowSteps reader tests, PlanApprovalIndex and PlanApprovalRequired tests, HistoryLine via tests.
- `docs/lanes/J-tests-taskfile.md`: this report.

## NOT VERIFIED on a live screen

Nothing here touches a UI. Windows and macOS were checked by `go vet` only; the tests were not run on those systems.

DONE
