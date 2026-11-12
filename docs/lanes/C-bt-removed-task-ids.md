# C-bt-removed-task-ids: a new task never takes a task id that a removed project holds

Base: plugin main cdbef50c. `git fetch origin && git merge origin/main` before the last run: "Already up to date".

Base check: `nextID` at internal/task/create.go:284 (base) was `dirs, err := gridhome.TasksDirs(home)` and read nothing under `projects/.removed`. `gridhome.TasksDirs` (gridhome.go:229-241) lists only `projects/<entry>/tasks`, so for `.removed` it gives `projects/.removed/tasks`, never `projects/.removed/<name>/tasks`. The stop condition did not apply.

## Proof table

| Row | Result |
|---|---|
| 1. TestBtRiRemovedProjectCounts gives T-08, T-12, T-01 | HOLDS. `go test -count=1 -v ./internal/task -run 'TestNextID\|TestBtRi'` exit 0, `--- PASS: TestBtRiRemovedProjectCounts` with three subtests. Red on base (create.go stashed): `bt_removed_task_ids_test.go:32: got "T-03", <nil>; want "T-08"` and `got "T-04", <nil>; want "T-12"`. |
| 2. End to end through `New` (TestBtRiNewAfterRemove) | HOLDS. alpha task is T-01, `os.Rename` of `projects/alpha` to `projects/.removed/alpha`, then Personal `New` gives T-02. Red on base: `first "T-01", second {Record:{ID:T-01 ... Project:personal ...}}; want T-02`. |
| 3. Invalid number in a removed folder errors; absent or empty `.removed` gives the same id | HOLDS. `TestBtRiInvalidRemovedNumberFails` PASS (on base: `got "T-01", nil error; want an error`). `TestBtRiNoRemovedFolderSameID` PASS: absent and empty `.removed` both give T-10 for the same live folders (green on base too, as it must be). |
| 4. Nothing else moves | HOLDS. `--- PASS: TestNextID` with its five subtests, task_test.go unedited. `go test -count=1 ./internal/task/ ./internal/gridhome/ ./internal/projects/` exit 0. |
| 5a. `make lint` | HOLDS, exit 0 (`go vet ./...`). |
| 5b. `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| 5c. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0. |
| 5d. `git diff main --stat` | HOLDS for code: only internal/task/create.go (+16) and internal/task/bt_removed_task_ids_test.go (+81). This report file is added as the runbook requires. |
| Importers of internal/task, tested | HOLDS, exit 0: task, gridhome, projects, braincmd, cli, guard, hook, job, job/frontmatter, job/jobcmd, job/jobtest (no tests), job/lint, land, signal, updates (no tests). The list is `go list -f '{{.ImportPath}} {{.Imports}}' ./...` filtered on internal/task. |
| Walk tests | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0. |
| No guard, gate, cap or hold on agents | HOLDS. The change only reads more folders; nothing is blocked. |
| No existing test edited or deleted | HOLDS. |
| No personal paths, hosts or emails; no attribution lines | HOLDS. |

## What changed

`nextID` (create.go) now also reads `projects/.removed/<each folder>/tasks` with the same `folders` and `number` helpers, so an invalid or overflowing number there fails like a live one. A missing `projects/.removed` is no error. Hidden staging folders stay skipped by `folderPattern`. The removed folders are read after the live ones, and the comment says why: Create holds `.task-alloc.lock` and Remove holds `.projects.lock`, so a remove can run during `nextID`. The rename only goes from `projects/<name>` to `projects/.removed/<name>`, so a folder that moves after the live read is found in the removed read.

`gridhome.TasksDirs` and every other reader are unchanged: removed tasks do not show in lists, Home or the worker.

## Notes

- A remove that runs between the `TasksDirs` read and the `.removed` read is covered. A remove that renames a folder while `ReadDir` of `projects` is in progress is also covered: the folder is either listed live or already in `.removed`. The `.removed` read is a single `ReadDir`, then the `tasks` folders one by one; a rename into `.removed` after that `ReadDir` can only be a folder the live read already saw.
- No concurrent-rename test was written; the concurrency argument is by read order only.

## NOT VERIFIED on a live screen

Nothing here is UI. No app rig sets up a removed project with a higher id, so `grid task new` against a real home with a removed project was not run end to end beyond `TestBtRiNewAfterRemove`.

Verdict: DONE
