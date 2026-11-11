# O-task-history-create: a hook line on a new task creates history.md instead of being dropped

Repo: plugin, lane branch lane-O-task-history-create, from main 330256ce. Code head 0bdd2e4f (fix 124e80cf, tests 0bdd2e4f); this report is committed after it.

Cause (eval-programme.md finding 6, prerequisite P4): `Create` writes only intent.md and task.md, and `AppendHistory` returned nil without writing when history.md was missing, so every hook record (`brain read`, `subagent finished`, tool lines) before the first journal write was dropped silently. The fix is in `AppendHistory` only, as the brief says: `Create` is unchanged, so the refused attach and delete tests that assert no history.md still hold.

## Proof table

All commands run from the repo root on this lane's head. Exit 0 unless noted.

| # | Row or rule | Result | Proof |
|---|---|---|---|
| 1 | A test that fails on main: Create a task, `AppendHistory(t.Path, "brain read", "brain:project/deploy", "grid-builder", "")`, read history.md | HOLDS | Main is 330256ce. With main's `history.go` checked out under the lane's test file (`git checkout 330256ce -- internal/task/history.go`, restored after with `git checkout HEAD -- internal/task/history.go`, `git status --short` empty), `go test -count=1 -run 'TestAppendHistory\|TestJournalWriteKeepsLines' -v ./internal/task/` exit 1: `history_create_test.go:39: open .../personal/tasks/T-01-example-task/history.md: no such file or directory` / `--- FAIL: TestAppendHistoryCreatesHistoryInAFreshTask`; `history_create_test.go:192: open .../history.md: no such file or directory` / `--- FAIL: TestAppendHistoryCreateIsSafeUnderConcurrency`; `history_create_test.go:243: want 4 lines, got 3:` / `--- FAIL: TestJournalWriteKeepsLinesAppendHistoryCreated` (on main the hook line before the note is simply absent). `TestAppendHistoryNeedsARegularTaskFile` and `TestAppendHistoryStillRefusesALinkedHistory` pass on main: they are controls for row 3, which must not change. |
| 2 | After the change history.md has exactly the one line (`taskfile.HistoryLine` shape), mode 0600; a different event appends; a repeat is skipped | HOLDS | `go test -count=1 -race -run TestAppendHistoryCreatesHistoryInAFreshTask -v ./internal/task/` exit 0. The test reads one line, compares it with `taskfile.HistoryLine(<its own timestamp>, "brain read", "brain:project/deploy", "grid-builder")`, checks 0600 (skipped on Windows), appends `subagent finished` and sees two lines, repeats it and sees the bytes unchanged (dedup at history.go, the `old == next` check), and checks 0600 again. |
| 3 | No task.md: no history.md, nil. history.md a symlink or directory: refused. task.md a symlink: not a task folder | HOLDS | `TestAppendHistoryNeedsARegularTaskFile` (plain temp dir, Grid home, project folder, task.md a directory, task.md a symlink): each returns nil, creates no history.md and no .task.lock, and the file behind the task.md symlink is unchanged. `TestAppendHistoryStillRefusesALinkedHistory`: history.md a symlink to an existing file, a symlink to a missing file, and a directory each return "history must be a regular file", the link or directory is not replaced, nothing is written through the dangling link. Both pass on main and on the lane, `-race`, exit 0. |
| 4 | 20 goroutines on a fresh task: 20 distinct lines, none lost, no error, -race | HOLDS | `TestAppendHistoryCreateIsSafeUnderConcurrency`: 20 goroutines released together, history.md has 20 lines, each `hook concurrent NN` exactly once, 20 distinct lines, no `.write-*` temp file left. `go test -count=1 -race` on it exit 0. The first writer creates the file under .task.lock; the file is re-read under the lock, so a writer that lost the race appends (history.go, the `switch` after the lock). |
| 5 | A later `Store.Note` or status move keeps the AppendHistory lines and appends after them | HOLDS | `TestJournalWriteKeepsLinesAppendHistoryCreated`: hook line, `Store.Note`, hook line, `Store.Change` close give four lines in that order (checked per line) and mode 0600. |
| 6 | attach_test.go:383 and trash_test.go:162, :393 pass unchanged | HOLDS | Those lines are in `TestAttachAgentBoundary`, `TestTrashSameMinuteAndOccupiedID` and `TestTrashMoveFailureLeavesNoReceipt`. `git diff --stat origin/main..HEAD` shows only history.go and the new test file; `go test -count=1 -race ./internal/task/` exit 0 (ok 8.6s). None of the three calls AppendHistory, so they are unaffected by design. |
| 7 | `go test -count=1 -race ./internal/task/ ./internal/hook/ ./internal/land/`; `go vet ./internal/task/`; Windows vet | HOLDS | Test run exit 0: `ok internal/task 8.6s`, `ok internal/hook 4.5s`, `ok internal/land 4.5s`. `go vet ./internal/task/` exit 0. `GOOS=windows CGO_ENABLED=0 go vet ./internal/task/` exit 0. |
| 8 | Packages that import task, with their tests | HOLDS | `go list -f '{{.ImportPath}}: {{join .Deps " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E 'grid-plugins/internal/task( \|$)'` lists: cmd/grid, internal/braincmd, cli, env, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, orchestrator, packaging, packaging/cmd, ports, signal, workflow. `go test -count=1` on those plus `./internal/job/...`: all ok, exit 0 (cmd/grid and job/jobtest have no tests). hook and land also ran with `-race` in row 7. |
| 9 | Lint | HOLDS | `make lint` exit 0 (go vet ./... and gofmt -l . clean). |
| 10 | Windows and macOS builds | HOLDS | `make cross` exit 0: vet and test compile of every package for darwin/arm64 and windows/amd64. |
| 11 | New tests fail on base and pass on the lane | HOLDS | Row 1 (three fail on main) and rows 2 to 5 (pass on the lane). The two controls pass on both, by design. |
| 12 | Edit only the named files; no existing test edited or deleted | HOLDS | `git diff --stat origin/main..HEAD`: `internal/task/history.go` (`AppendHistory` and its doc comment only) and the new `internal/task/history_create_test.go`. No change to create.go, write.go or the hook package. No test deleted. |
| 13 | No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change writes a line that was being dropped. It adds no refusal: an agent's hook line that was accepted before is accepted now, and a missing history.md in a task folder no longer loses it. |
| 14 | Small commits, one-sentence messages, no attribution lines, public repo clean | HOLDS | Two commits (fix, tests), one sentence each, no Co-Authored-By or session line. Tests use `t.TempDir()`; no personal path, host or email in the lane's files. |
| 15 | New tests in a new file named after the lane's subject, not rows in a shared table | HOLDS | `internal/task/history_create_test.go`, five test functions. |

## What changed

`internal/task/history.go`, `AppendHistory` only:

- A folder is a task folder when it holds a regular task.md (`os.Lstat`, so a symlinked or directory task.md does not count). A local `isTask` closure keeps the change inside the one function.
- Before the lock: history.md missing and not a task folder returns nil, as on main (no lock file is made in a plain folder). History.md that is not a regular file is still refused with "history must be a regular file".
- Under .task.lock: history.md is opened with `OpenNoFollow`. Missing in a task folder starts from empty content; missing elsewhere returns nil. The file is read after the lock, so concurrent first writers do not lose a line.
- The write is the existing `atomicWrite`, which gives a new file CreateTemp's mode 0600, the same as the journal writes (mode_test.go:112-141). The dedup, the `plan accepted` quoting and the line shape are untouched.

## Notes for the backlog (not bugs in this lane)

- A crash can leave `.task-write.json`. `recoverWrite` replays its full history image, which would drop a line AppendHistory wrote after the crash and before the next journal write. This is the same on main for an existing history.md; `AppendHistory` does not run `recoverWrite`.
- The task folder test is "holds a regular task.md", as the brief says, not the folder-name pattern `FolderAt` uses; the hook callers already pass folders that `FolderAt` resolved.

## NOT VERIFIED on a live screen

Nothing here touches a UI or a served read. The scorecard row "Notes read (hook fired)" (`T/history.md`, `brain read`) needs a live task and a real hook: the orchestrator's live check on the dev UI covers it.
