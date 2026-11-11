# C-pcov2-where-projects-guest: tests for the resolver's per-role refusals and the registry's relink refusals

Repo: plugin. Branch lane-C-pcov2-where-projects-guest, from main 55c46cbd. Linux, go test with the repo's own go.
Tests only: two new files, no non-test file changed, no existing test edited.

- internal/where/role_refusals_test.go
- internal/projects/relink_refusals_test.go

COV folder: `mktemp` and `mkdir` were refused in this sandbox, so the profiles went to the clone root as
`cov-<pkg>-before.out` and `cov-<pkg>-after.out`. `.gitignore:5` (`*.out`) keeps them out of the commit.

## Proof table

| # | Row | Result | Command / place |
|---|-----|--------|-----------------|
| 1 | Coverage before and after, per package | HOLDS | where before `ok ... coverage: 91.9% of statements`, after `96.9%`; projects before `84.5%`, after `89.4%` (go test -count=1 -coverprofile=..., exit 0 each). `go tool cover -func` after: `total: (statements) 96.9%` (where), `total: (statements) 89.4%` (projects). |
| 1a | PersonalProjectAllowed and LookupError.Unwrap at 100.0% | HOLDS | `where/personal.go:65:	PersonalProjectAllowed	100.0%`, `where/errors.go:15:	Unwrap			100.0%` (also TasksDirs 100.0%, hasTaskID 100.0%). |
| 2 | where role rows: PersonalProjectAllowed per role, TasksDirs with projects a file | HOLDS | `TestPersonalProjectAllowedByRole` (owner, member, guest, empty_role) and `TestTasksDirsProjectsIsAFile` (owner, guest). go test exit 0. |
| 3 | where guest Projects | HOLDS | `TestGuestProjectsBrokenRecords`: projects a file -> error; invited record a directory -> exactly "cannot read project record"; invited record "not front matter" -> exactly "cannot parse project record"; the same two broken files in uninvited beta -> no error, records == [alpha]. |
| 4 | where Resolve refusals | HOLDS | `TestResolveNoFolder` ("no folder to resolve"); `TestResolveExplicitPersonalTaskByRole` (owner, member, guest, empty_role, each as path and as symlink outside the Grid home; skips when os.Symlink fails); `TestResolveTaskPathStatFails` (owner: error contains "does not exist", errors.Is ErrTaskNotFound false; skips on Windows with the reason). |
| 5 | where --task id with --project, tasks folder unreadable | HOLDS | `TestResolveTaskIDTasksUnreadable`: owner no tasks folder -> *LookupError "task T-09 not found in project alpha", Is ErrTaskNotFound (covers Unwrap); member -> Is ErrTaskNotFound; owner tasks a file -> same text, Is ErrTaskNotFound false. Guest not run (the brief's row names member). |
| 6 | where hasTaskID through a task ULID lookup | HOLDS | `TestResolveTaskULIDFrontMatter`: `ulid:` in front matter found; no opening `---`, id after the closing `---`, front matter never closing and no id: each returns `task <ULID> not found in project other` (where.go:300), Is ErrTaskNotFound. |
| 7 | projects records and List | HOLDS | `TestListUnsafeProjectsFolder` (symlink: Is localdata.ErrUnsafePath; regular file: error), `TestListPersonalByRole` (member -> [alpha]; "" -> [alpha personal]). |
| 8 | projects Add/Relink/Remove/Scan/Apply refusals | HOLDS | `TestRefusalsLeaveRegistryUnchanged` ("is not a directory" for Add, Relink, Scan on a file; `project "nope" is missing or ambiguous`; "machine file does not contain a ULID" for Add and Apply) and `TestUnsafeProjectsFolderRefusesRemoveAndScan` (Remove and Scan with projects a symlink: Is ErrUnsafePath). project.md bytes are compared before and after every refusal, and the projects folder is checked to hold only `a`. |
| 9 | projects relink refusals | HOLDS | `TestRelinkRefusals`: "Personal has no folder to relink"; "cannot relink to a Personal folder"; exactly "folder or repo already belongs to project b; relink that project or merge it before adding"; a's project.md unchanged after each. |
| 10a | go test -count=1 ./internal/where/ ./internal/projects/ : 0 fail | HOLDS | `ok .../internal/where 2.043s`, `ok .../internal/projects 0.678s`, exit 0 (run inside the importer run below). |
| 10b | Windows and macOS test builds, both packages | HOLDS (compiled) | `GOOS=windows go test -c -o /dev/null ./internal/where/` exit 0; same for `./internal/projects/`; `GOOS=darwin` the same, exit 0 for both. Compiled, needs Windows run: `go test -count=1 ./internal/where/ ./internal/projects/` on a Windows host. Compiled, needs macOS run: same command on macOS. |
| 10c | Symlink rows skip when os.Symlink fails; ENOTDIR row skips on Windows | HOLDS | `t.Skipf("no symlinks here")` in `TestResolveExplicitPersonalTaskByRole` and `refSymlink`; `t.Skip("Windows answers a path through a regular file as not-exist.")` in `TestResolveTaskPathStatFails`. The skips themselves are compiled only; needs a Windows run. |
| 10d | make lint: 0 fail | HOLDS | `make lint` printed `go vet ./...` and exited 0. `gofmt -l internal/where internal/projects` printed nothing. |
| R1 | Tests of the changed packages and every importer | HOLDS | `go test -count=1 ./internal/braincmd/ ./internal/cli/ ./internal/guard/ ./internal/hook/ ./internal/job/jobcmd/ ./internal/job/jobtest/ ./internal/land/ ./internal/projectmd/ ./internal/question/ ./internal/settings/ ./internal/signal/ ./internal/task/ ./internal/workflow/ ./internal/projects/ ./internal/where/` exit 0: all `ok` (jobtest has no test files). List from `go list -f ... ./...`: importers of where are braincmd, cli, guard, hook, job/jobcmd, job/jobtest, land, projectmd, projects, question, settings, signal, task, workflow; importers of projects are cli, job/jobcmd, signal, workflow. The whole-module test and the cross-repo set are the orchestrator's. |
| R2 | New tests fail where the behaviour is broken | HOLDS (mutation) | These tests pin what the code does today, so they pass on base code; the files do not exist on base. To show they bite, three temporary edits to non-test lines were made, the new tests run, and the edits reverted with `git checkout` (git status clean after). Failing lines below. |
| R3 | No edit to an existing test, no deleted test | HOLDS | `git diff origin/main --stat` lists only the two new test files and this report. |
| R4 | No guard, gate, cap or hold on agents | HOLDS | No non-test file touched. |
| R5 | No special files, no chmod triggers | HOLDS | Triggers are a regular file where a folder is expected, a directory where a file is expected, and symlinks. |
| R6 | Public repo: no personal paths, hosts or emails | HOLDS | Test files and this report name only temp-folder-relative paths. |
| R7 | Commits: one-sentence messages, no attribution lines | HOLDS | `git log -3 --format=%B` (see the hand-off). |

### R2: failing lines from the mutation check

`internal/where/personal.go:66` made `return true`:
```
--- FAIL: TestPersonalProjectAllowedByRole (0.00s)
    --- FAIL: TestPersonalProjectAllowedByRole/member (0.00s)
        role_refusals_test.go:47: PersonalProjectAllowed("personal") = true, want false
    --- FAIL: TestPersonalProjectAllowedByRole/guest (0.00s)
        role_refusals_test.go:47: PersonalProjectAllowed("Personal") = true, want false
```
`internal/where/where.go:213` condition made `false && ...`:
```
    --- FAIL: TestResolveExplicitPersonalTaskByRole/member/symlink (0.00s)
        role_refusals_test.go:170: Resolve = {...}, this caller cannot use the Personal project; want ErrTaskNotFound
```
`internal/projects/projects.go:447` condition made `... && false`:
```
    --- FAIL: TestRelinkRefusals/a_folder_in_the_Grid_home (0.00s)
        relink_refusals_test.go:224: Relink("a", ".../home/.grid/scratch") error = <nil>, want "cannot relink to a Personal folder"
    --- FAIL: TestRelinkRefusals/a_folder_another_project_holds (0.00s)
        relink_refusals_test.go:227: project a changed:
```

## What changed

Two new test files, 13 test functions (8 in where, 5 in projects), no change to a non-test file.

Uncovered blocks the brief named are all covered now (checked against `cov-*-after.out`):
where personal.go:65-67, :72-74; guest.go:100-102, :117-119, :121-123; errors.go:15; where.go:85-87, :213-215, :230-235, :274-283, :318-334.
projects.go:73-75, :77-79, :93-95, :98-100, :121-123, :214-216, :391-393, :418-424, :440-442, :447-449, :454-456, :515-517, :518-520, :559-561.
Blocks still at 0 are the ones the brief left out: where guest.go:49/79-84/105, where.go:128/258/342/483-487/506 (and :183, which was not picked); projects Lock failures, write's I/O branches, inspect's git branches, `sameFolder`, and a few Scan and `change` error branches outside the rows.

Notes on what the code does (the code wins over the brief where they differ):
- The `--task` symlink row reaches where.go:213-215 through the member row. With :213 disabled, only member/symlink failed: the guest still got ErrTaskNotFound from a later refusal (the guest task-path or project check), so for a guest that row holds on a different line than :213-215.
- Row 5 runs owner and member as the brief's row says; the guest branch of :276-277 is the same line as member's and was not given its own row.

## NOT VERIFIED on a live screen

No UI in this lane, so nothing needs a live screen. Needs a host that is not this one: running both packages on Windows (symlink and ENOTDIR skips, ReadDir on a regular file) and on macOS, `go test -count=1 ./internal/where/ ./internal/projects/`. Only the compile (`go test -c`) was done here.

Result: DONE
