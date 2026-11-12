# C-xn2-project-remove-moves: `grid project remove` moves the data folder to projects/.removed/<name>

Base: main 7a4bc0c2 (the brief names 2221cb86; on 7a4bc0c2 Remove at projects.go:397-398 is still `change(name, "")`, and change at :408-465 rewrites projects/<name>/project.md and moves nothing, so the base check passes).

Verdict: NOT DONE. Every row holds except row 4: `TestRelinkAndRemoveKeepData` (internal/projects/projects_test.go:125-138) pins the old behaviour, and that file is not in Touches. The follow-up is two lines; see row 4.

## Proof table

| # | Row | State | Proof |
|---|-----|-------|-------|
| 1 | Move | HOLDS | `go test -count=1 -v -run 'TestXnRm' ./internal/projects/` exit 0: `--- PASS: TestXnRmMovesAndUnlists`. After Remove("alpha"), List has beta and no alpha, projects/alpha is gone, and the tree under projects/.removed/alpha (project.md, tasks/T-1/task.md, knowledge/x/y.md) is byte-identical to the one before (xn_remove_move_test.go:86-111). The answer keeps alpha's name and id, with no folders or keys. |
| 2 | Collision | HOLDS | Same run: `--- PASS: TestXnRmCollision`. .removed/alpha already holds other bytes; the 2nd alpha lands at .removed/alpha-2 and the 3rd at alpha-3; after each move every earlier folder is byte-identical; .removed holds exactly 3 entries (xn_remove_move_test.go:113-139). |
| 3 | Nothing else moves | HOLDS | Same run: `--- PASS: TestXnRmOthersUnchanged`. Remove("nope") fails with `project "nope" is missing or ambiguous` and the whole projects/ tree is unchanged. With projects/.removed a regular file, Remove("alpha") fails, alpha is still listed and projects/alpha is unchanged. After that, Remove("alpha") leaves beta's bytes and its whole List entry unchanged (xn_remove_move_test.go:141-173). **Needed a change in records() (projects.go:81-82)**: see "What changed", point 2. |
| 4 | CLI and readers | **NOT DONE** | `go test -count=1 ./internal/projects/ ./internal/cli/` exit 1. `internal/cli` ok (69.2s). `internal/projects` fails one test: `--- FAIL: TestRelinkAndRemoveKeepData` with `projects_test.go:134: lost body:  open .../home/.grid/projects/example/project.md: no such file or directory`. This is the only failure in the package (`go test -count=1 -v ./internal/projects/` lists only this FAIL). The test reads project.md (:132) and notes/keep.md (:136) at projects/example/ after Remove, so it pins "the folder stays at projects/<name>", which "What to build" removes. projects_test.go is not in Touches, so I did not edit it. **Re-brief:** add `internal/projects/projects_test.go` to Touches. At :132 read `projects/.removed/example/project.md`, at :136 stat `projects/.removed/example/notes/keep.md`. The test still checks that no data is lost across Remove. The two gk_keys_survive_test.go cases are changed as Touches asks and pass (`--- PASS: TestKeysSurviveWriters/remove`, `--- PASS: TestKeysSurviveRelinkAfterKeySet`). The golden is unchanged: `git diff main -- internal/cli/testdata` prints nothing (exit 0). |
| 5 | lint, cross | HOLDS | `make lint` exit 0 (go vet ./... clean, no gofmt output). `make cross` exit 0 (darwin/arm64 and windows/amd64: vet and test compile, every package ok). |
| 6 | Files | HOLDS | `git diff main --stat`: internal/projects/gk_keys_survive_test.go, internal/projects/projects.go, internal/projects/xn_remove_move_test.go, plus this report docs/lanes/C-xn2-project-remove-moves.md. |
| R | New tests fail on base | HOLDS | I stashed projects.go only (`git stash push -- internal/projects/projects.go`), ran `go test -count=1 -v -run 'TestXnRm\|TestKeysSurvive' ./internal/projects/` and got exit 1: `gk_keys_survive_test.go:185: open .../projects/.removed/alpha/project.md: no such file or directory` / `--- FAIL: TestKeysSurviveWriters/remove`; `xn_remove_move_test.go:104: List after Remove still has alpha: map[alpha:{...Folders:[] ...} beta:{...}]` / `--- FAIL: TestXnRmMovesAndUnlists`; `xn_remove_move_test.go:127: .../projects/alpha: Lstat error <nil>, want it gone` / `--- FAIL: TestXnRmCollision`; `xn_remove_move_test.go:158: lstat .../projects/.removed/project.md: not a directory` / `--- FAIL: TestXnRmOthersUnchanged`. Then `git stash pop`. |
| R | No existing test edited to pass | HOLDS, with the brief's exceptions | Only the two gk_keys_survive_test.go cases the brief names were changed. TestRelinkAndRemoveKeepData was left as it is (row 4). |
| R | Deleted test step names its cover | HOLDS | The remove step of TestKeysSurviveRelinkAfterKeySet was dropped. Its comment (gk_keys_survive_test.go:102-105) names TestKeysSurviveWriters/remove as the cover. No test function was deleted. |
| R | No guard, gate, cap, hold or hook | HOLDS | Remove checks no tasks or agents. The only failures are the rename and filesystem errors it returns (projects.go:445-476). |
| R | Importers tested | HOLDS for the importers | Importers of internal/projects (`go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep 'grid-plugins/internal/projects\b'`): internal/cli, internal/job/jobcmd, internal/projectmd, internal/projects, internal/signal, internal/workflow. `go test -count=1` on all six: cli ok, jobcmd ok, projectmd ok, signal ok, workflow ok, projects FAIL (row 4 only). |
| R | Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (ok 0.179s). `go test -count=1 ./internal/job/lint/` exit 0 (ok 0.373s). |
| R | RealPlugin | n/a | No row names a *RealPlugin* test. |
| R | No attribution, no personal paths | HOLDS | Commit messages are one sentence with no trailers. The test paths are t.TempDir. |

## What changed

1. `Store.Remove` (projects.go:397-404 doc, move at :445-476 inside change, under the `.projects.lock` that change takes at :409). It finds the record and drops this machine's folders and keys as before, so the Entry JSON is unchanged. It writes nothing. It makes projects/.removed (`os.MkdirAll`), then picks the first free `projects/.removed/<name>`, `<name>-2`, `<name>-3`... by `os.Lstat`, like migrate plan.go:188-189. Both paths go through `localdata.Safe`. Then one `os.Rename`. When the rename fails, the error comes back and nothing has changed except an empty projects/.removed that MkdirAll may have made. The source is `rec.Name`, not the operand, because find also matches on the id. Relink still writes through `s.write` and is unchanged.
2. `records()` (projects.go:81-82) now skips regular files in projects/. Before this, `localdata.Safe(home, "projects", <file>, "project.md")` got ENOTDIR from Lstat for any regular file there, and that is not ErrNotExist. So List, Add, Relink, Remove and Scan all failed. This was already true on main for any file in projects/ (for example a Finder .DS_Store), and it was the reason row 3 failed. project.Load already skips such files (project.go:123). Links still go to Safe and are still refused (TestUnsafeProjectsFolderRefusesRemoveAndScan passes). The brief's Touches note for projects.go names only Remove. This line is in the same file, so row 6 holds, but I am pointing it out here so it is not found only in the diff.
3. gk_keys_survive_test.go. The remove case reads projects/.removed/alpha/project.md and expects the start file unchanged, order `id lands owner folders note keys tags`. The remove step is dropped from TestKeysSurviveRelinkAfterKeySet. The header comment no longer lists Remove among the writers of Store.write.
4. New file xn_remove_move_test.go: TestXnRm* tests and xnRm* helpers.

## Readers (as the brief asks)

- gridhome.TasksDirs (gridhome.go:229-241) only reads `projects/` and lists `projects/<entry>/tasks`, including `projects/.removed/tasks`. It creates nothing.
- Its callers create nothing either: environment.go:39-42, worker.go:251-254, task_operand.go:33 (via the resolver, which only filters), task/create.go:236-246 and :284-290 (via `folders`, task.go:365-368). All of them ReadDir and move on when the folder does not exist.
- project.Load (project.go:122-130) skips projects/.removed, which is a directory with no project.md. It never reads .removed/<name>/project.md.

## Notes (not changed, for the backlog)

- task/create.go:284 works out the next task id over TasksDirs. That covers `projects/.removed/tasks` but not `projects/.removed/<name>/tasks`, so the T-NN numbers of a removed project are no longer counted and a new task can reuse one.
- Remove treats Personal like any other project. `grid project remove Personal` (if the resolver lists it) now moves projects/Personal to .removed/Personal. Relink refuses Personal (projects.go:479); Remove has no such check, and I added none.
- On Linux, `rename(2)` replaces an empty directory. The Lstat check runs under `.projects.lock`, so only a writer that does not take that lock could race it.

## NOT VERIFIED

- Windows rename with a file held open inside the folder. The brief expects the error to come back with nothing changed; that needs a Windows host. `make cross` only proves the code compiles there.
- The app side (remove_project.go against the new plugin), which is WAIT-C-xn3.

Head: fafa3b18 (plus the commit adding this report).

NOT DONE
