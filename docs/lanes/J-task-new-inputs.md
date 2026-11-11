# J-task-new-inputs: repeatable `--input` for `grid task new`

Proof table (head is the commit that adds this file; code is at 63524def).

| Row | Result | Proof |
|---|---|---|
| 1. `--input a.png --input notes.txt` copies byte-equal files into `<task>/inputs/`, inside the stage, base name only | HOLDS | `go test ./internal/cli/ -run 'TaskNew\|Inputs'` exit 0 (`TestTaskNewInputsCopiesFiles`); `go test ./internal/task/` exit 0 (`TestInputsCopiedByteEqualUnderBaseName`). Copy runs at internal/task/create.go (`copyInputs(stage, ...)`, after intent.md, before the rename); name is `filepath.Base` at internal/task/inputs.go `copyInput`. `TestInputsCopyFailureLeavesNoHalfTask` shows a copy failure leaves no task folder. |
| 2. Missing or folder `--input` fails before anything is written, non-zero, message names the path | HOLDS | `checkInputs` runs first in `Store.New` (internal/task/create.go, before the lock and the Grid home); `TestTaskNewInputsMissingOrFolderWritesNothing` (exit non-zero, stderr holds the path, Grid home snapshot unchanged) and `TestInputsBadPathFailsBeforeAnythingIsWritten` (Grid home not even created) pass. |
| 3. Same base name gets a numbered name | HOLDS | `a.png`, `a-2.png`, `a-3.png` (`TestInputsSameBaseNameGetsNumberedName`, `TestTaskNewInputsSameNameNumbered`), O_EXCL so no overwrite. |
| 4. No `--input` makes the same folder as today | HOLDS | `TestNoInputsMakesNoInputsFolder` (folder holds exactly intent.md, task.md); `TestTaskNewWithoutInputsMakesNoInputsFolder`. The whole existing `go test ./internal/cli/` and `./internal/task/` pass. |
| 5. `go test ./internal/task/` 0 fail; `go test ./internal/cli/ -run 'TaskNew\|Inputs'` 0 fail; `GOOS=windows go vet ./internal/task/` exit 0 | HOLDS | all three exit 0 (windows vet run on `./internal/task/ ./internal/cli/`). |
| New tests fail on base, pass on lane | HOLDS | With the source changes stashed, `go test ./internal/cli/ -run 'TaskNew\|Inputs'` failed: `TestTaskNewInputsCopiesFiles ... exit = 2, stderr "flag provided but not defined: -input"` (same for RelativePath, MissingOrFolder, SameNameNumbered). The `internal/task` tests do not compile on base (`NewOptions.Inputs`, `InputsDir` undefined). |
| Packages that import task: tests | HOLDS | importers from `go list`: braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, land, signal, cmd/grid. `go test` of those plus task, contract: all ok, exit 0 (full `./internal/cli/` ok, 52s). |
| Lint, Windows and macOS builds | HOLDS | `make lint` exit 0; `GOOS=windows go build ./cmd/grid` exit 0; `GOOS=darwin go build ./cmd/grid` exit 0; `GOOS=windows|darwin go vet ./internal/task/ ./internal/cli/` exit 0. |
| No guard, gate, cap or limit added | HOLDS | No count or size limit on inputs; nothing blocks an agent. |
| No existing test edited or deleted | HOLDS | `git diff origin/main --stat` shows only new test files. Two generated data files changed, see below. |
| Small commits, one sentence, no attribution, no personal data | HOLDS | 3 commits; no trailer lines. |

## What changed

- `internal/task/inputs.go` (new): `checkInputs`, `copyInputs`, `copyInput`, `InputsDir`.
- `internal/task/create.go`: `checkInputs` after `Validate`; `copyInputs(stage, o.Inputs)` after intent.md in the stage.
- `internal/task/task.go`: the `Inputs []string` field on `NewOptions`. The brief named create.go for the field, but `NewOptions` is declared in task.go.
- `internal/cli/task.go`: repeatable `--input` flag (`inputList`), relative paths joined to the working folder, usage line.
- New tests: `internal/task/inputs_test.go`, `internal/cli/task_new_inputs_test.go`.
- Generated files outside the brief's list, changed because a new flag changes them: `contract/commands.json` (the `--input` flag entry; `TestContractCommandsParserInventory` fails without it) and `testdata/golden/task-help.golden` (usage line; `TestTaskRenameDeclineHelp`). I edited them by hand because the `UPDATE_CONTRACT_COMMANDS=1` regenerate form is refused in my shell; the inventory test passes against the hand edit.

## Notes

- `go build ./cmd/grid` with GOOS set left untracked `grid` and `grid.exe` in the clone root; my shell refuses `rm`. They are not committed and not in the bundle. Delete them if the clone is reused.
- A symlink given to `--input` is followed and its target copied.
- Merge with I-realrun-first-job: its hunk follows `store.New`; mine is before it (the `o.Inputs` loop) and in the flag block.

## NOT VERIFIED on a live screen

- The app's new-task form does not pass `--input` yet; nothing in the UI was run. The app images path (`not_supported`) is unchanged by this lane.
- Cross-repo set and whole-module test are left to the orchestrator.

DONE
