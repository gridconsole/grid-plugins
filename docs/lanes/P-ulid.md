# P-ulid: plugin task commands read and write the migrated task shape

Changed (internal/task, tests in internal/task and internal/cli):
- task.Fold reads the migrated shape, `id: <ULID>` with `display: T-NN`. The display must match the folder's T-NN, and a `ulid:` that differs from the id is refused. It still reads the legacy shape: `id: T-NN` or no id, plus an optional `ulid:`. In memory `Record.ID` stays the T-NN display id, so the output of show, list and the receipts is the same.
- grid task new writes `id: <ULID>` then `display: T-NN`, with no `ulid:` line. New always writes VERSION 1 and refuses other versions, so it never needs the legacy shape. The legacy shape stays read-only (contract/README.md:221, Task IDs are ULIDs).
- Golden testdata/golden/task-record.golden was updated by hand to the new header, because the env-prefixed UPDATE_GOLDEN run is refused here. TestSessionGolden, TestTaskOperandHelp and TestContractCommandsParserInventory pass unchanged, so they were not regenerated.
- Existing helper `legacyTask` (internal/task/attach_test.go) is edited, not an assertion. It still builds a pre-ULID task: it now turns `display: T-NN` back into `id: T-NN` and drops the ULID id line. Its two tests keep their meaning.

New tests (fail on base, pass on lane):
- TestFoldTaskShapes (internal/task/fold_shape_test.go). Base: `fold_shape_test.go:40: Fold: {...}, task.md needs a matching id, title, project and state` (migrated). `fold_shape_test.go:35: Fold error <nil>, want "display"` (legacy other display).
- TestTaskCommandsMigratedAndLegacyShape (internal/cli/task_shape_test.go) runs task show, rename (folding verb), new and list on a synthetic migrated home and a legacy one. Base: `task_shape_test.go:37: show: 1 grid task show: ...T-01-fix-the-label: task.md needs a matching id, title, project and state` and `task_shape_test.go:65: new task is not in the migrated shape`.

Packages that import internal/task (go list): braincmd cli guard hook job job/jobcmd job/jobtest land orchestrator signal task.

Checks:
- go test ./... : exit 0 (covers every importer above)
- make lint (go vet ./...) : exit 0
- gofmt -l internal/task internal/cli : empty
- GOOS=windows go build ./... : exit 0
- GOOS=darwin go build ./... : exit 0

NOT VERIFIED: never run against a real migrated home. Only synthetic fixtures were used. No UI changes.
