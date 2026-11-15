# C-s6c-brain-search-misses-note

Brief: finish C-s6b. The fixture note got no hits once the project resolved (end review 0212Z S6).

## Fixture fault or scope fault: fixture

The test now first searches with the project named (`brain search --project example …`). On C-s6b's fixture, that also missed:

```
s6_brain_session_task_test.go:43: brain search --project example: exit 1 out "" err "brain: project \"example\" is not registered\n"
```
After a project record was added (`projects/example/project.md`), it still missed:
```
s6_brain_session_task_test.go:44: brain search --project example: exit 0 out "hits: []\nquery: install page handbook links commands\n" err ""
```
So the miss was the fixture, not scope. C-s6b's copy of the note had no `origin` block. `brain.scan` (internal/brain/topics.go:241) treats a note as invalid when `Note.Validate` fails, and Validate requires an origin id and kind (internal/brain/note.go:115-121). An invalid note is never searched. The real round-8 note carries `origin.kind: session` and `origin.id: d183b075b0c9c1dacc4e8567` (the r8 setup log in the eval workspace, lines 10-11). The fixture now has the same origin and a project record. Search scoring and the indexing code were not changed.

## The product change alone holds

With the fixture fixed and main's `internal/braincmd/command.go` and `internal/cli/brain.go` checked out, the `--project example` search passes. The no-flag search from the Job worktree still fails, so C-s6b's change to which project a read resolves to is needed:
```
--- FAIL: TestS6BrainSearchUsesSessionTaskProject/GRID_TASK (0.00s)
    s6_brain_session_task_test.go:52: brain search from the Job worktree with GRID_TASK: exit 1 out "" err "brain: this caller cannot use the Personal project\n"
--- FAIL: TestS6BrainSearchUsesSessionTaskProject/GRID_FEED_TASK (0.00s)
    s6_brain_session_task_test.go:52: brain search from the Job worktree with GRID_FEED_TASK: exit 1 out "" err "brain: this caller cannot use the Personal project\n"
FAIL	github.com/gridconsole/grid-plugins/internal/cli
```
After those files were restored, `git status --short` showed only the test file. The run on the lane after `git merge origin/main`:
```
--- PASS: TestS6BrainSearchUsesSessionTaskProject (0.01s)
    --- PASS: TestS6BrainSearchUsesSessionTaskProject/GRID_TASK (0.00s)
    --- PASS: TestS6BrainSearchUsesSessionTaskProject/GRID_FEED_TASK (0.00s)
ok  	github.com/gridconsole/grid-plugins/internal/cli
```

## Importers

The `go list` of packages that import internal/braincmd or internal/cli:
```
github.com/gridconsole/grid-plugins/cmd/grid
github.com/gridconsole/grid-plugins/internal/cli
github.com/gridconsole/grid-plugins/lib
```

## Proof table

| Row | State | Evidence |
|---|---|---|
| Step 2: test runs the search with `--project example`, cause written | HOLDS | internal/cli/s6_brain_session_task_test.go:42-46; the cause is the fixture (above) |
| Step 3: fix the cause the evidence shows | HOLDS | Fixture fixed (origin block and project.md). The product change is kept because it is still needed (base run above). |
| Acceptance: new test fails on main, passes on lane | HOLDS | Main's product files: exit 1 (pasted). Lane: `go test -count=1 -v -run TestS6BrainSearchUsesSessionTaskProject ./internal/cli/` exit 0 |
| Step 4: importers' tests | HOLDS | `go test -count=1 ./internal/braincmd/ ./internal/cli/ ./cmd/grid/ ./lib/` exit 0 (braincmd ok, cli ok, lib ok, cmd/grid has no test files) |
| Step 4: `make lint` | HOLDS | `make lint` (go vet ./...) exit 0 |
| Step 4: Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| Step 4: `git merge origin/main` | HOLDS | `git fetch origin && git merge origin/main` exit 0, before every proof run above |
| Step 4: plugin walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0 |
| Acceptance: lane doc updated | HOLDS | this file |
| Rule: no guard, gate, cap, hold or blocking hook added | HOLDS | The product change only widens which project a read resolves to; this lane changed only the test fixture. |
| Rule: no existing test edited to pass | HOLDS | Only C-s6b's new test file changed, and it is not on main |
| Rule: no attribution lines, no personal paths | HOLDS | `git log origin/main..HEAD` |
| App walk tests and guest walk tables | not run, not asked | Plugin repo |
| Whole-module test | not run, not asked | The orchestrator runs it at landing |
| RealPlugin tests | not run, not asked | None named |

## What changed

- internal/cli/s6_brain_session_task_test.go: the note fixture gets the real note's `origin` block, the fixture gets a `project.md` record, and the test first searches with `--project example` before the no-flag searches.
- Kept from C-s6b: internal/cli/brain.go (`SessionTask`) and internal/braincmd/command.go (the session task picks the project when no flag is given).

## Open items

None.

## NOT VERIFIED on a live screen

Nothing; no UI.

DONE
