# C-small-where-task-file-not-folder

Base sha: 5925b841098c704e445a7bd6da9d342a05983f68 (lane branch start).

## Proof table

| Row | Result |
|---|---|
| 1. File path resolves on base, errors after | HOLDS. On base, `go test ./internal/where/ -run TaskFileNotFolder` failed: `task_file_not_folder_test.go:39: file path: err = <nil>, want ErrTaskNotFound` for owner, member, guest and "". After the change: pass (internal/where/where.go:230-241 `info.IsDir()` check). Text contains "does not exist" (asserted at the test file :41). |
| 2. Same file as an id gives ErrTaskNotFound | HOLDS. TestTaskFileNotFolderByID passes on base and on the lane (the :286 path is unchanged). It needs `Cwd` in the query; without one Resolve answers "no folder to resolve". |
| 3. Role rows owner, member, guest (sam, project-roles.json), "" | HOLDS. TestTaskFileNotFolderByPath has four subtests, each asserting errors.Is ErrTaskNotFound, "does not exist", no path outside the Grid home, and that `T-02-real` by path still resolves. |
| 4. Symlinked task folder resolves | HOLDS. TestTaskSymlinkToFolderResolves passes (Stat is kept, not Lstat). It skips on Windows when os.Symlink fails. |
| 5. `tasks` itself a regular file | HOLDS. TestTasksFolderIsAFileKeepsItsAnswer passes on base and on the lane: "does not exist", not ErrTaskNotFound. It skips on Windows with a reason. |
| 6a. `go test ./internal/where/ -count=1` | HOLDS, exit 0 (`ok ... 5.314s`). |
| 6b. `GOOS=windows go vet ./internal/where/` | HOLDS, exit 0. |
| 6c. `make lint` | HOLDS, exit 0 (`go vet ./...`). |
| Importers' tests | HOLDS, exit 0. Importers of internal/where from go list: braincmd, cli, guard, hook, job/jobcmd, job/jobtest, land, projectmd, projects, question, settings, signal, task, workflow. All pass (jobtest has no tests). |
| Builds | HOLDS. `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` both exit 0. gofmt -l is empty. |
| New tests fail on base | HOLDS. The four role subtests fail on base (lines above). The other three tests are contrast rows and pass on both. |
| No existing test edited or deleted; no guard, gate or cap | HOLDS. The diff adds one new test file and changes only the Stat block in where.go. The change is a lookup error result, with no refusal on agents. |
| Commit message, no attribution | HOLDS. One sentence, no trailer. |

## What changed

- internal/where/where.go (Stat block in explicitTask): when Stat succeeds on a non-directory, return `&LookupError{Kind: ErrTaskNotFound, Message: "task folder <dir> does not exist"}`. Other Stat errors (such as ENOTDIR) keep their plain error. The symlink case is unchanged because Stat still follows links.
- internal/where/task_file_not_folder_test.go (new).

## NOT VERIFIED on a live screen

None. This is a lookup change with no UI. The Windows tests were vetted and built only, not run.
The cross-repo set and the whole-module test are left to the orchestrator at landing. After C-pcov2-where-projects-guest merges, its where tests need a rerun.

DONE
