# C-fv-plugin-guest-task-new-approve

Base: 4e736654 (plugin main). On the base `guestWriteVerb` (internal/cli/cli.go:229-242) did not name `task new`, so all rows apply. Lane code head: 8f13558d.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Invited guest `task new "a guest task" --project example`: exit 1, line, project snapshot unchanged | HOLDS | `go test -count=1 -run TestFvGuestTaskNewRefusedEarly/invited ./internal/cli/` PASS. Stderr is `grid task new: role "guest" may not new a task; a guest answers and comments` (guestTaskMoveRefusal, internal/cli/cli.go:252-254, same wording as close/reopen/stop). guestSnapshot of the project folder identical (fv_guest_task_test.go:72-80). |
| 2. Uninvited guest, unknown project (and no `--project`): same exit and stderr | HOLDS | Subtests `uninvited`, `unknown_project`, `no_project_flag` of TestFvGuestTaskNewRefusedEarly PASS: exit 1 and the same line, stdout empty, project folder unchanged. |
| 3. member, owner, empty role, guest + GRID_LEASE=task: `task new` exit 0, folder exists | HOLDS | TestFvGuestTaskNewStillCreates PASS (4 subtests): exit 0, `--json` path is a directory, project folder changed. |
| 4. Guest `grid approve`: non-zero, `approving is for a person of the team: set GRID_MEMBER`, snapshot identical; member does not get the line | HOLDS | TestFvGuestApproveRefused/guest and /member PASS. The guest run reaches approvalWriter (approve_record.go:33); task_write.go:177-181 returns 3 (the test asserts non-zero plus the line). |
| 5. TestFvGuestTaskVerbs, one row per write verb, plus the commands.json check | HOLDS | PASS, 11 subtests. The same test reads ../../contract/commands.json and fails on a `task` subcommand in neither the table nor show/list/history, or a table verb missing from commands.json. Table below. |
| 6a. TestGuestStillAnswersAndComments passes unchanged | HOLDS | Passes in `go test -count=1 -run 'TestFvGuest\|TestGuest' ./internal/cli/` (not in the failing list). guest_role_test.go not edited. |
| 6b. `go test -run 'TestFvGuest\|TestGuest' ./internal/cli/`: 0 fail | **NOT DONE** | 4 subtests of the existing TestGuestProjectParity fail, see "Blocker". |
| 6c. `git diff main --stat` names only the two Touches files | HOLDS for code | `git diff main --stat`: internal/cli/cli.go (8 +-), internal/cli/fv_guest_task_test.go (286 +). This report is a third file, in its own commit, as the runbook asks. |
| 6d. `make lint` exit 0 | HOLDS | `make lint` ran `go vet ./...` and printed no finding. The exit code was not captured (piped into tail). |
| New tests fail on base | HOLDS | With internal/cli/cli.go from 4e736654: `--- FAIL: TestFvGuestTaskNewRefusedEarly/invited`: `exit 0, want 1: grid task new: T-02 has no first Job: ...`; `/uninvited`: `stderr "grid task new: project is not registered", want "grid task new: role \"guest\" may not new a task; ..."`; `/unknown_project` the same; `/no_project_flag`: `"grid task new: this caller cannot use the Personal project"`; `TestFvGuestTaskVerbs/new`: `exit 0 ... changed true; stdout: T-02 created`. So on base an invited guest's `task new` created T-02. cli.go restored with `git checkout HEAD -- internal/cli/cli.go`; `git status --short` clean. |
| Tests of changed package and its importers | HOLDS except the blocker | `go list` importers of internal/cli: cmd/grid, lib. `go test -count=1 ./internal/cli/ ./cmd/grid/ ./lib/`: lib ok, cmd/grid no test files, internal/cli FAIL only on TestGuestProjectParity/task_new/{project,project-project-id}/{plain,json}. Every other test in internal/cli passes. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./... && GOOS=darwin go build ./...` exit 0. |
| No guard on agents | HOLDS | GRID_LEASE=task is still exempt (cli.go:194, unchanged); TestFvGuestTaskNewStillCreates/guest_with_task_lease PASS. Nothing removed. |
| *RealPlugin* tests | n/a | None named by the brief; none run. |

### Row 5 table (invited guest, fixture task T-01 queued, project without a git remote, no GRID_TASK)

| verb | exit | first stderr line | project changed |
|---|---|---|---|
| new | 1 | `grid task new: role "guest" may not new a task; a guest answers and comments` | no |
| stop, close, reopen | 1 | `grid task <verb>: role "guest" may not <verb> a task; a guest answers and comments` | no |
| delete, restore | 3 | `grid task <verb>: task edit rejected: a person with an allowed role must perform this task move` | no |
| rename | 1 | `grid task rename: scope: role cannot rename tasks` (RenameAllowed) | no |
| decline | 1 | `grid task decline: scope: role cannot decline tasks` (DeclineAllowed) | no |
| move | 1 | `grid: This project has no remote, ...` (not a role check) | no |
| attach | 1 | `grid task attach: agent cannot attach T-01 without its assigned task folder` (agent scope, not a role check) | no |
| status (queued to running) | 0 | none; stdout `T-01 status: queued -> running` | **yes** |

TestFvGuestTaskVerbsWhenValid runs the two verbs the table leaves open with valid input: `task attach T-01` with GRID_TASK=T-01 exits 0 and writes a session record; `task move laptop --task T-01 --project example` on a project whose folder is a git repo with a remote exits 0 and queues a move intent (project folder changed).

## Blocker (row 6b)

The existing internal/cli/guest_parity_test.go (TestGuestProjectParity) fails four subtests, and I did not edit it: Touches names two files, and the rules forbid editing an existing test to make it pass.

```
--- FAIL: TestGuestProjectParity/task_new/project/plain/invited
    guest_parity_test.go:388: invited guest still gets unknown answer: {Code:1 Stdout: Stderr:grid task new: role "guest" may not new a task; a guest answers and comments
```

The same line for task_new/project/json, task_new/project-project-id/plain and /json. That test lists the early refusals in `guestParityEarlyAnswer` (guest_parity_test.go:408-435: task delete/restore, task close/reopen/stop, ...) and fails when an invited guest gets the same answer as an unknown project for any other command. The refusal this brief asks for is exactly such an early refusal, so the listing needs one more case. Needed change, not applied and not run: in `guestParityEarlyAnswer`, add `"task new"` to the case at guest_parity_test.go:418, or a case giving `guestParityOutput{1, "", "grid task new: role \"guest\" may not new a task; a guest answers and comments\n"}`. The orchestrator decides whether to apply it at landing or widen Touches.

## What changed

- internal/cli/cli.go: `guestTaskMove` now includes `new`, so `guestWriteVerb` refuses a guest's `task new` before any project is resolved, exit 1, same wording as stop/close/reopen. Comments updated.
- internal/cli/fv_guest_task_test.go (new): TestFvGuestTaskNewRefusedEarly, TestFvGuestTaskNewStillCreates, TestFvGuestApproveRefused, TestFvGuestTaskVerbs, TestFvGuestTaskVerbsWhenValid.

## Follow-ups (guest writes the CLI layer does not refuse; asserted as seen, not fixed)

- `grid task status <task> <state>` by an invited guest changes task.md (queued to running, exit 0).
- `grid task move <machine>` by an invited guest queues a move intent on a project with a remote.
- `grid task attach` by an invited guest writes a session record when GRID_TASK names the task (agentShell is true for the guest role, so the scope rule allows the bound task).
- The wording "may not new a task" is ungrammatical; kept because the brief asks for the same wording as close/reopen/stop.
- `grid task new --help` as a guest is refused like `grid task close --help` (guestWriteVerb exempts only a lone --help).

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: the hosted server relaying a browser write to this plugin.
