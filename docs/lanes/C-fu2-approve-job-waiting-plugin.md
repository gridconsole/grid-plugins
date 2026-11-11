# C-fu2-approve-job-waiting-plugin

Base: 4e736654 (origin/main, nothing newer after `git fetch`; no rebase needed).
Base check: task_write.go:186-197 and person.go:19-33 on that base print only the stderr line; no JSON body before this lane.

## Proof table

| Row | Result |
| --- | --- |
| 1. TestFu2ApproveJobWaitingJSON: exit 3, stdout exactly the object, stderr names J-1, J-1/J-2 posted, no approval.json | HOLDS: `go test -count=1 -v -run Fu2 ./internal/cli/` exit 0, `--- PASS: TestFu2ApproveJobWaitingJSON` (internal/cli/fu2_approve_job_waiting_test.go:68). On base it fails: `fu2_approve_job_waiting_test.go:77: stdout "": unexpected end of JSON input` |
| 2. Same without --json: stdout empty, exit 3, stderr as today | HOLDS: `--- PASS: TestFu2ApproveJobWaitingWithoutJSON` (:98); asserts stderr equals `grid approve: J-2 cannot be answered yet: waiting for J-1 to be accepted\n` |
| 3. TestApproveWaitsForTheAfterJob and TestApproveLoadFailureExits3WithPrefix pass unchanged; load failure with --json prints no job_waiting | HOLDS: approve_job_test.go not edited (not in `git diff origin/main --stat`); both pass in `go test -count=1 ./internal/cli/` exit 0; `--- PASS: TestFu2ApproveLoadFailureWithJSONHasNoJobWaiting` (:113) asserts empty stdout and stderr `grid approve: feed unreadable\n` |
| 4. owner and member give exit 3 plus the object | HOLDS: `--- PASS: TestFu2ApproveJobWaitingRoles/owner`, `/member`. Base fails: `stdout ""` at :141 |
| 4. GRID_AGENT=1 refused ahead of the new body | **DIFFERS, premise not true on this base.** runApprove has no agent refusal before the wait. `workerMarked` (task_write.go:209) is read only by person check (person_check.go:22) and agentShell (task_write.go:222, whose comment says no command refuses a caller because of it). approvalWriter (approve_record.go:20-38) looks at GRID_ROLE and GRID_MEMBER only. With GRID_AGENT=1 and the same setup, the run exits 3 on base (stdout empty, stderr the wait line) and on this lane exits 3 with the same stderr plus the job_waiting object. Subtest `TestFu2ApproveJobWaitingRoles/agent` pins that. I added no guard (rule: none on agents). The object is the wait reason only and opens nothing. If the app lane needs an agent to get no body, that is a decision for the owner, not a change I made. |
| 5. `go test -count=1 ./internal/cli/ ./internal/orchestrator/` 0 fail | HOLDS: exit 0, `ok internal/cli 44.588s`, `ok internal/orchestrator 51.256s` |
| 5. `make lint` exit 0 | HOLDS: exit 0 (`go vet ./...`); `gofmt -l internal/cli internal/orchestrator` empty |
| 5. `git diff origin/main --stat` names only Touches | HOLDS: contract/README.md, internal/cli/fu2_approve_job_waiting_test.go, internal/cli/task_write.go, internal/orchestrator/person.go |
| TestContractCommandsParserInventory | HOLDS: passes inside the cli package run; commands.json and contract_inventory_test.go untouched |
| Tests of changed packages and importers | HOLDS: `go list` importers of internal/orchestrator or internal/cli: cmd/grid, internal/cli, internal/migrate, lib. `go test -count=1 ./cmd/grid/ ./internal/migrate/ ./lib/` exit 0 (cmd/grid has no test files; migrate ok 10.5s, lib ok 4.6s) |
| Windows and macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| New tests fail on base, pass on lane | HOLDS: base failures pasted in rows 1 and 4; lane run all PASS |
| *RealPlugin* tests | Not needed: no row names one |
| No guard, no attribution lines, no personal paths | HOLDS: no refusal added (same refusal cases as base: the Ready wait only); commit messages carry no attribution |

## What changed

- internal/orchestrator/person.go: new exported `WaitingError{Job, Target, Reason}`; `Error()` gives `<id> cannot be answered yet: <reason>` as before. `PersonReady` returns it inside `feed.InvalidRecordError`, so every errors.As/Is on that type still matches (PersonOpen, AnswerPerson and personAnswers get the same error value text).
- internal/cli/task_write.go (approve branch): after the unchanged stderr line, with --json and an errors.As match on `*orchestrator.WaitingError`, writes `{"error":"job_waiting","job":...,"waiting_for":...,"reason":...}` to stdout, then returns 3. The snapshot-load failure is not a WaitingError and keeps today's output.
- contract/README.md: one paragraph for the job_waiting body next to the person_required text.
- internal/cli/fu2_approve_job_waiting_test.go: four tests (seven with subtests).

## NOT VERIFIED on a live screen

The app lane's reading of the body (app plugincli client) and the 409 mapping: not run here. Only the plugin's stdout/exit code are proved.
