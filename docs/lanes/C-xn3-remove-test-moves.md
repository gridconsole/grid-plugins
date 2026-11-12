# C-xn3-remove-test-moves: TestRelinkAndRemoveKeepData reads the removed data under projects/.removed

Base: lane-C-xn2-project-remove-moves at 9b70f109, merged with origin/main 792b8d6e (local main is the same sha) in merge commit 1a547fae.

Verdict: DONE. Every row holds. No product code changed.

## Proof table

| # | Row | State | Proof |
|---|-----|-------|-------|
| D1 | Merge origin/main | HOLDS | `git merge --no-edit origin/main` made 1a547fae with no conflicts, so I resolved no files. |
| D2 | Change only the after-Remove assertions | HOLDS | fc08584a changes internal/projects/projects_test.go:132-144 only (8 lines added, 2 removed). The test reads `projects/.removed/example/project.md` and checks "Project prose." (:133-136). It stats `projects/.removed/example/notes/keep.md` (:137-139). It checks that projects/example/project.md, projects/example/notes/keep.md and projects/example itself are gone, using `os.Lstat` + `os.IsNotExist` (:140-144). Every relink assertion (:108-124) and the Remove answer check (:125-131) are unchanged. |
| D3 | Product code only if data is lost | HOLDS, no product change | The test passes against the move, so no data is lost. `git diff --stat 1a547fae..HEAD` names internal/projects/projects_test.go plus the two lane reports, and no .go product file. |
| A1 | `go test ./internal/projects/ -run TestRelinkAndRemoveKeepData -count=1 -v` | HOLDS | exit 0: `--- PASS: TestRelinkAndRemoveKeepData (0.24s)` / `PASS` / `ok  github.com/gridconsole/grid-plugins/internal/projects 0.437s` |
| A2.1-3 | C-xn2 rows 1-3: Move, Collision, Nothing else moves | HOLDS | `go test -count=1 -v -run 'TestXnRm\|TestKeysSurvive' ./internal/projects/` exit 0: `--- PASS: TestKeysSurviveRelinkAfterKeySet`, `--- PASS: TestKeysSurviveWriters`, `--- PASS: TestXnRmMovesAndUnlists`, `--- PASS: TestXnRmCollision`, `--- PASS: TestXnRmOthersUnchanged`, `ok ... internal/projects 0.770s` |
| A2.4 | C-xn2 row 4: CLI and readers | HOLDS | `go test ./internal/projects/ ./internal/cli/ -count=1` exit 0: `ok ... internal/projects 6.134s`, `ok ... internal/cli 153.006s`. The gk_keys_survive_test.go cases pass (row above). `git diff --stat main -- internal/cli/testdata` prints nothing, so golden project-remove-json is unchanged. |
| A2.5 | C-xn2 row 5: lint, cross | HOLDS | `make lint` exit 0 (`go vet ./...`, no gofmt output). `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package `ok` or `[no test files]`. |
| A2.6 | C-xn2 row 6: files | HOLDS | `git diff --stat main..HEAD`: docs/lanes/C-xn2-project-remove-moves.md, internal/projects/gk_keys_survive_test.go, internal/projects/projects.go, internal/projects/projects_test.go, internal/projects/xn_remove_move_test.go, plus this report. projects_test.go is the file this brief adds to C-xn2's Touches. This report is the one the builder runbook requires. |
| A3 | `go test ./internal/projects/ ./internal/cli/ -count=1` | HOLDS | exit 0, the same run as A2.4. |
| A4 | `make lint` and `make cross` | HOLDS | Both exit 0, the same runs as A2.5. |
| R | Changed test fails on base | HOLDS | With main's projects.go (`git checkout origin/main -- internal/projects/projects.go`, then `git checkout HEAD --` it back), `go test ./internal/projects/ -run TestRelinkAndRemoveKeepData -count=1 -v` exit 1: `projects_test.go:135: lost body:  open .../home/.grid/projects/.removed/example/project.md: no such file or directory` / `--- FAIL: TestRelinkAndRemoveKeepData`. Before the edit, on the merged lane head: `projects_test.go:134: lost body:  open .../home/.grid/projects/example/project.md: no such file or directory` / `--- FAIL: TestRelinkAndRemoveKeepData`. `git status --short` was clean after the restore. |
| R | Existing test edited | HOLDS, by direction | This brief's Do item 2 changes the after-Remove assertions of TestRelinkAndRemoveKeepData. The owner decision is that removed project data moves to `<home>/projects/.removed/<name>` and nothing is deleted. No other test was edited. |
| R | Deleted test | n/a | No test or assertion was deleted. The new assertions check the same data (body and note) in its new place. |
| R | No guard, gate, cap, hold or hook | HOLDS | Only a test changed. |
| R | Importers tested | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E 'grid-plugins/internal/projects( \|$)'` lists internal/cli, internal/job/jobcmd, internal/projectmd, internal/projects, internal/signal and internal/workflow. `go test -count=1` on all six exits 0: projects and cli are in A3, and `jobcmd ok 7.971s`, `projectmd ok 1.089s`, `signal ok 1.048s`, `workflow ok 16.353s`. |
| R | Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok 0.291s`). `go test -count=1 ./internal/job/lint/` exit 0 (`ok 0.378s`). |
| R | RealPlugin | n/a | No row names a *RealPlugin* test. |
| R | No attribution, no personal paths | HOLDS | Commit messages are one sentence with no trailers. The test paths are t.TempDir. |

## What changed

1. internal/projects/projects_test.go:132-144: after Remove, TestRelinkAndRemoveKeepData reads project.md and notes/keep.md under `projects/.removed/example/`. It checks that both, and the folder itself, are gone from `projects/example/`.
2. docs/lanes/C-xn2-project-remove-moves.md: the Verdict line, row 4 and the last line now say that this lane closed row 4. The original NOT DONE text of row 4 is kept as the record.
3. This report.

The C-xn2 rows come from the C-xn2 brief as it was handed to its builder. The `briefs/C-xn2-project-remove-moves.md` path this brief names is not in the repo.

## Open items (carried from C-xn2, not changed)

- task/create.go:284 picks the next task id over TasksDirs. That list does not cover `projects/.removed/<name>/tasks`, so a new task can reuse a removed project's T-NN.
- Remove has no Personal check. Relink refuses Personal (projects.go:479).

## NOT VERIFIED

- Windows rename with a file held open inside the folder. `make cross` proves only that the code compiles for Windows.
- The app side (remove_project.go against the new plugin), which is WAIT-C-xn3 in the app repo.

Head: the commit adding this report, on top of fc08584a.

DONE
