# H-brain-propose-task: `grid brain propose --task` writes the proposal where `grid brain decide` reads it

Base: main with G-brain-check-gate-remove merged (`git merge-base --is-ancestor 5061998f HEAD`, exit 0). Code commit 498de989 on lane-H-brain-propose-task.

## Proof table

| # | Row | Result | Proof |
| --- | --- | --- | --- |
| 1 | `grid brain propose --task <task> ...` writes `<task folder>/brain-proposals/<name>.md` and appends one `brain proposed · <name>` line to that task's history.md | HOLDS | `go test ./internal/brain/... ./internal/braincmd/... -run Propose -count=1`, exit 0. `TestProposeTaskWritesTheTaskFolderAndHistory` (internal/braincmd/propose_task_test.go:40) reads the file from the task folder, checks it is absent under `projects/sample/brain-proposals`, and counts the history line (exactly 1, still 1 after a refused duplicate). Store level: `TestProposeTaskWritesTheTaskFolder` (internal/brain/propose_task_test.go:26). Code: internal/brain/store.go `ProposeTask`, internal/braincmd/command.go `propose` case |
| 2 | Round trip: propose with `--task`, then `grid brain decide <task> <name> accept`; the note lands in knowledge/ with the proposal's status set | HOLDS | `TestProposeTaskThenDecideRoundTrip` (internal/braincmd/propose_task_test.go:72): propose exit 0, decide exit 0, `projects/sample/knowledge/conventions/fresh.md` parses as `accepted` and holds the body, the task-folder proposal parses as `accepted`. Store level: `TestProposeTaskThenDecideTaskAccepts`. Same command as row 1, exit 0 |
| 3 | Without `--task`, propose is as today (existing propose tests pass unchanged); a `--task` that does not resolve exits non-zero and writes nothing | HOLDS | `git show --stat HEAD` lists no edit to an existing test file (two new test files only); the existing propose tests (`TestTwoProcessesProposeAtOnce`, internal/cli brain tests) pass in the row-5 run, exit 0. `TestProposeWithoutTaskStaysAtTheProjectLevel` shows a project-level file, no task file and no task history. `TestProposeTaskThatDoesNotResolveWritesNothing` (`--task T-99`): exit non-zero, error on stderr, no `fresh.md` anywhere under the Grid home, no history.md |
| 4 | `grid brain keep --task <task>` lists the task-folder proposal | HOLDS | `TestProposeTaskIsListedByKeep`: after propose, `keep --task T-01` exit 0 prints `task:brain-proposals/fresh.md -> conventions: ...`. No change to waiting.go; it already reads the task folder |
| 5 | `go test ./... -count=1` for internal/brain, internal/braincmd, internal/cli and contract, and `go vet ./...`: 0 fail | HOLDS | See "Packages tested" below, exit 0. `go vet ./...` exit 0 |
| R1 | New tests fail on base and pass on the lane | HOLDS | Base source (command.go and store.go stashed), same command, exit 1: see "Failing on base" below. On the lane: exit 0 |
| R2 | Packages that import the changed packages are tested | HOLDS | go list output (direct importers of internal/brain): braincmd, cli, hook, job, job/jobcmd, job/jobtest, people, routine, task, workflow; of internal/braincmd: cli. All tested, see below |
| R3 | Lint | HOLDS | `go vet ./...` exit 0; `gofmt -l .` prints nothing, exit 0 |
| R4 | Windows and macOS builds | HOLDS | `make cross` exit 0: vet and test compile for darwin/arm64 and windows/amd64 |
| R5 | Help golden and command inventory regenerated | HOLDS | `make generate` exit 0; only contract/commands.json changed (`--task` added to `propose`), no other generated drift. testdata/golden/brain-help.golden edited by hand to the new usage (internal/cli `TestRemainingTaskOperandHelp/brain` failed before, passes now) |
| R6 | No guard, gate, cap, hold or blocking hook added | HOLDS | `ProposeTask` refuses only what `Propose` already refuses: an invalid note, an unknown topic, a duplicate name. No new check, limit or hook |
| R7 | No existing test edited or deleted | HOLDS | `git show --stat HEAD`: 2 test files created, none modified or deleted |
| R8 | Commit hygiene | HOLDS | One commit, one-sentence message, no attribution line. No personal path, host or email in the diff |

### Failing on base

Command: `go test ./internal/brain/... ./internal/braincmd/... -run Propose -count=1` with internal/braincmd/command.go and internal/brain/store.go at base. Exit 1.

```
internal/brain/propose_task_test.go:29:18: s.ProposeTask undefined (type Store has no field or method ProposeTask)
FAIL	github.com/gridconsole/grid-plugins/internal/brain [build failed]
--- FAIL: TestProposeTaskWritesTheTaskFolderAndHistory (0.00s)
    propose_task_test.go:44: exit 2: brain: unknown flag --task
--- FAIL: TestProposeTaskThenDecideRoundTrip (0.00s)
    propose_task_test.go:75: propose exit 2: brain: unknown flag --task
--- FAIL: TestProposeTaskIsListedByKeep (0.00s)
    propose_task_test.go:101: propose exit 2: brain: unknown flag --task
FAIL	github.com/gridconsole/grid-plugins/internal/braincmd
```

### Packages tested

`go test -count=1 ./internal/brain/... ./internal/braincmd/... ./internal/cli/... ./internal/hook/... ./internal/job/... ./internal/people/... ./internal/routine/... ./internal/task/... ./internal/workflow/... ./contract/... ./internal/golden/... ./internal/packaging/...` exit 0: brain, braincmd, cli (26s), hook, job, job/frontmatter, job/jobcmd, job/lint, people, routine, task, workflow, contract/formats, golden, packaging, packaging/cmd all `ok`.

## What changed

- `internal/brain/store.go`: `Propose` keeps its behaviour; its stamping and validation moved to `proposalNote`. New `Store.ProposeTask(task, note, origin, history)` validates the same way, then writes `<task folder>/brain-proposals/<name>.md` under the task's `.brain-decide.lock` (the lock `DecideTask` takes), then calls `history("brain proposed · <name>")` after the lock is released. If the history line cannot be written the proposal is removed again, so a failure leaves nothing. A secret-shaped note is stored redacted and refused, with no history line, as `Propose` does. The entry path is `task:brain-proposals/<name>.md`, the form `keep` already uses for task proposals.
- `internal/braincmd/command.go`: `propose` takes `--task <task>`. The task is resolved before anything is written, by the same resolver and refusal as `show --task`; if it does not resolve the command exits through `TaskOperandError` and writes nothing. Without `--task` the code path is the old one, even from inside a task folder. The history line is written by the task writer from the task's own folder, so it does not depend on the caller's directory. Usage line and help text updated.
- `contract/commands.json` and `testdata/golden/brain-help.golden`: the new flag and usage text.
- `internal/brain/propose_task_test.go` (5 tests) and `internal/braincmd/propose_task_test.go` (5 tests).

## Open items

- contract/formats.md "Brain origin stamp" still lists only the two brain-level proposal locations; the brief did not list that file, so a task-folder proposal is not described there. Add it where the formats owner next edits that section.
- `grid brain show --proposal` has no `--task` read for a task-folder proposal; the brief did not ask for it. `keep --task` and `decide` read it.
- The history line's author is `grid-brain` (role agent), as `decide` uses `grid-curator`. The origin stamp in the note stays a random session id.
- `decide.go` writes its history line from the caller's directory with the task id, so `decide T-01` from outside the project can fail to find the task. Not changed (not in scope); `propose --task` avoids it by writing from the task folder.

## NOT VERIFIED on a live screen

- No live agent run: a curator that proposes with `--task` and then decides was not run (the brief says no `claude`). The round trip is proven by the in-process CLI test above.
- The dev UI's brain or task view was not opened. The task view shows the proposal through the existing task-folder reader (`keep`/ship gate); only the command output and files were checked.
