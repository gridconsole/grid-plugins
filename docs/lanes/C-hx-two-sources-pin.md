# C-hx-two-sources-pin

Test-only lane. One new file: `internal/workflow/hx_two_sources_test.go`. No production file changed.

## Proof table

| Row / rule | Result |
|---|---|
| 1. Disagreeing sources: `TaskSteps` returns `ship:run` (workflows.md), `GatePostRoles` returns person `[owner, member]` and ci `[]` (whole map compared), `ReadFor` returns `project-brain` with the note's "Quick" steps `Do the work:run,Approve it:approve`; one test, comment and failure message as the brief words them | HOLDS: `TestHxTwoSourcesDisagree` (hx_two_sources_test.go:63), `go test ./internal/workflow -run 'TestHxTwo|...' -count=1` exit 0 |
| 2. Note only: `GatePostRoles` returns the note's roles; `TaskSteps` returns err != nil and no steps; message pinned from `no workflow is named %q` (tasksteps.go:152) | HOLDS: `TestHxTwoNoteOnly`, same command, exit 0 |
| 3. File only: `GatePostRoles` equals `defaultGatePostRoles()`, `TaskSteps` returns `ship:run` | HOLDS: `TestHxTwoFileOnly`, same command, exit 0 |
| 4. `go test ./internal/workflow -run 'TestHxTwo\|TestTaskSteps\|TestGatePostRoles\|TestReadFor' -count=1` | HOLDS: exit 0, all PASS |
| 4. `make lint` | HOLDS: exit 0 (`go vet ./...`) |
| 4. `git diff main --stat` names only the new test file | HOLDS: plus this report under docs/lanes |
| Tests of importers: `go list` gives internal/cli, internal/feed, internal/task, internal/workflow | HOLDS: `go test ./internal/workflow ./internal/cli ./internal/feed ./internal/task -count=1` exit 0, all ok |
| Windows build | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS: `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| New tests fail on base | NOT APPLICABLE: the lane pins today's answers, so the tests pass on base by design. They turn red when the steps reader changes (the failure message names what to do). Not demonstrated by editing production code, which the brief forbids. |
| No guard/gate/cap on agents; no existing test edited or deleted; no RealPlugin test in this lane | HOLDS |
| Names: every package-level name starts with hxTwo, tests with TestHxTwo | HOLDS (hxTwoFileQuick, hxTwoSteps, hxTwoHome, hxTwoWantRoles) |

## What changed

New test file with three tests. Each builds a home like TestTaskSteps (workspace workflows.md = default section only, project.md, project workflows.md "Quick" with one step `ship`, project brain note "flow" written with `noteText`, task folder without workflow.md).

Deviation from the brief's wording: the note's member is `member`, not a free name. The gate-roles parser accepts only the roles cli, owner, member, lead, agent, worker (gates.go:86-89, :124), so a made-up name makes the whole note get rejected and `GatePostRoles` falls back to the default. That is how the first run failed.

The steps answer (workflows.md wins) is pinned as the known gap, not as correct.

## NOT VERIFIED on a live screen

Nothing here touches a screen.
