# C-xc-legacy-home-no-stamp: the first write to an old unversioned Grid home no longer stamps VERSION 1

Base: the lane branch is cut from main at `103b9157` (the brief names main `44c7de5b`; the three stamp lines are where it says). The three stamps on base; none checks for old data:

- `internal/localdata/localdata.go:109`  `_, err = fmt.Fprintf(f, "%d\n", gridhome.FormatVersion)`
- `internal/settings/store.go:353`  `if err := atomicfile.Write(filepath.Join(home, "VERSION"), []byte(fmt.Sprintf("%d\n", gridhome.FormatVersion)), 0o644); err != nil {`
- `internal/task/create.go:119`  `if err := atomicWrite(filepath.Join(home, "VERSION"), []byte(fmt.Sprintf("%d\n", gridhome.FormatVersion))); err != nil {`

Verdict: DONE. Row N1 was NOT DONE here and is CLOSED BY C-xc2-legacy-test-version (report: docs/lanes/C-xc2-legacy-test-version.md). Every other row holds.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 TestXcFreshHomeStamped (task, settings, localdata) | HOLDS | A missing home and an empty home each end with VERSION `"1\n"`. `go test -count=1 -v` on the three packages shows `--- PASS: TestXcFreshHomeStamped` in each (`xc_legacy_stamp_test.go` in each folder). The brief says `Store.Create`; the function is `task.Store.New`. |
| 2 TestXcLegacyHomeNotStamped (three files) | HOLDS | On a copy of `../migrate/testdata/owner-home` each site returns an error containing `Run: grid migrate`, VERSION stays absent, and the whole tree (path and sha256 of every file) is the same before and after: no `.settings.lock`, `.task-alloc.lock`, `.xc.lock`, `machine`, `settings.json` or task folder. localdata is tried through `Lock`, `Machine` and `Init`. `--- PASS: TestXcLegacyHomeNotStamped` shown for task, settings and localdata (row 5a). |
| 2 on base | HOLDS | Same three test files on base `103b9157`, run before any production edit, exit 1: see "Failing on base". |
| 3 TestXcAppFilesStillFresh (gridhome file plus task file) | HOLDS | A home with only `projects/p/project-roles.json`, `projects/p/docs/`, `projects/p/brain/`: `gridhome.Legacy` is false (`internal/gridhome/xc_legacy_test.go`), and `Store.New` stamps `"1\n"` and makes the task (`internal/task/xc_legacy_stamp_test.go`). Both `--- PASS` in row 5a. |
| 4 TestXcVersionedHomesUnchanged (gridhome file) | HOLDS | `internal/gridhome/xc_legacy_test.go`. At VERSION 1 with owner-home's tasks, `localdata.Lock`, a settings write and `task.New` all succeed and VERSION stays `"1\n"`. That tree is one on which `Legacy` is true (TestXcLegacyRule asserts it), so a writer that asked `Legacy` there would have refused: this covers `task.New` too. At VERSION 2 all three return text containing `Update grid.`. A third subtest makes `projects` a file so `Legacy` itself errors; `Lock` and the settings write still succeed, which shows they do not ask it when VERSION exists. The test is an external `package gridhome_test` so it can import the writers. |
| 5a five packages | HOLDS | `go test -count=1 -v ./internal/gridhome/ ./internal/localdata/ ./internal/settings/ ./internal/task/ ./internal/migrate/`, every package `ok`, with `--- PASS: TestXcLegacyHomeNotStamped` for localdata, settings and task; gridhome shows `TestXcLegacyRule`, `TestXcAppFilesStillFresh`, `TestXcVersionedHomesUnchanged` PASS; task also shows `TestXcFreshHomeStamped` and `TestXcAppFilesStillFresh` PASS. |
| 5b walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `ok  .../internal/cli  0.356s`. `go test -count=1 ./internal/job/lint/`: `ok  .../internal/job/lint  3.461s`. Re-run after this report was written: see the last line of "Walk tests after the report". The app-repo walk tests do not apply to the plugin repo. |
| 6a make lint | HOLDS | `make lint` ran `go vet ./...`, no output, exit 0. |
| 6b make cross | HOLDS | `make cross` exit 0; it printed `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`, every package `ok`. |
| 6c diff stat | HOLDS | `git diff main --stat` names the Touches files (`internal/gridhome/gridhome.go`, `internal/localdata/localdata.go`, `internal/settings/store.go`, `internal/task/create.go`, `internal/cli/cli.go` comment only, the four new `xc_legacy*_test.go` files), one fixture helper (`setup` in `internal/guard/guard_test.go`, 3 lines), and this report in `docs/lanes/`, where every lane keeps its report. |
| Tests of importers | HOLDS (N1 CLOSED BY C-xc2-legacy-test-version) | `go list` importers of localdata, settings, task or gridhome: contract/formats, internal/account, agent, brain, braincmd, cli, doctor, gitx, golden, guard, hook, job, job/jobcmd, job/jobtest, land, localdata, mcp, migrate, orchestrator, ports, project, projectmd, projects, proof, routine, settings, signal, task, updates, usage, where, worker, workflow, lib, plus gridhome itself. `go test -count=1` on all: every package `ok` except `internal/cli`, red only on N1. Before the guard fixture fix `internal/guard` was red; before the dot-folder skip `internal/task` `TestNewSweepsOnlyStaleStagingFolders` was red; both are green now. |
| `grid migrate` still reaches an old home | HOLDS (not by the brief's grep) | I did not run the brief's `git grep` on the ref repo. What I saw: `go test ./internal/migrate` is `ok`, the cli suite is green except N1 (no migrate command test is red), and in `internal/cli` the lane changes only a comment in `cli.go`. |
| New tests fail on base, pass on lane | HOLDS | "Failing on base" below, and the `--- PASS` lines in row 5a. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change refuses a write to a data folder of an old format with the existing `FormatProblem(0)` text, as `FormatProblem` already does for VERSION 2. No agent, tool or hook is gated. |
| Commits | HOLDS | One sentence each, no attribution lines, no personal paths or hosts in code, tests or this report. |
| RealPlugin tests, UI rows, mismatch proof | N/A | No row names them. |
| Budget | NOT MET | About 70 calls used against 55. The importer run found 2 reds that needed fixing and the cross build outran its first timeout. |
| Shas | | The checks ran on `52699469`; the later commits add only this report. |
| N1 `internal/cli` `TestTaskCommandsMigratedAndLegacyShape/legacy` | CLOSED BY C-xc2-legacy-test-version | The legacy case now writes VERSION `"1\n"` before `task new`; `go test -count=1 ./internal/cli -run TestTaskCommandsMigratedAndLegacyShape` and `go test -count=1 ./internal/cli/ ./internal/gridhome/` pass (exit 0). See below. |

### N1: an existing test outside Touches asserts the old stamping (CLOSED BY C-xc2-legacy-test-version)

`internal/cli/task_shape_test.go:28-31` removes VERSION from a home that holds a legacy-shape task, then `:52` runs `grid task new` and expects it to succeed. That is the old stamping on an old home, which this brief removes: `task new` now returns `grid task new: This data is format 0; this grid reads 1. Run: grid migrate`.

```
--- FAIL: TestTaskCommandsMigratedAndLegacyShape (0.01s)
    --- FAIL: TestTaskCommandsMigratedAndLegacyShape/legacy (0.00s)
        task_shape_test.go:53: new: 1 grid task new: This data is format 0; this grid reads 1. Run: grid migrate
FAIL
FAIL	github.com/gridconsole/grid-plugins/internal/cli	176.519s
```

The file is not in Touches and the test body is not a fixture helper, so I left it. Suggested fix for the orchestrator to authorize: in the `legacy` case write VERSION `"1\n"` just before the `task new` call at `:52` (the check that the new task has the migrated shape still holds), or assert the refusal there. `show` and `rename` in the same test run on the home with no VERSION and pass.

## Failing on base

Test files only, production code untouched, base `103b9157`; `go test -count=1 -run 'TestXc' ./internal/task/ ./internal/settings/ ./internal/localdata/` exit 1:

```
--- FAIL: TestXcLegacyHomeNotStamped (0.00s)
    xc_legacy_stamp_test.go:137: New on an old home: error <nil>, want one containing "Run: grid migrate"
    xc_legacy_stamp_test.go:140: VERSION written: "1\n"
    xc_legacy_stamp_test.go:143: the old home changed:
        added .task-alloc.lock
        added VERSION
        added machine
        added personal/
        added personal/tasks/
        added personal/tasks/T-347-second-task/
        added personal/tasks/T-347-second-task/intent.md
        added personal/tasks/T-347-second-task/task.md
FAIL
FAIL	github.com/gridconsole/grid-plugins/internal/task	0.509s
--- FAIL: TestXcLegacyHomeNotStamped (0.00s)
    xc_legacy_stamp_test.go:110: SetSkill on an old home: error <nil>, want one containing "Run: grid migrate"
    xc_legacy_stamp_test.go:113: VERSION written: "1\n"
    xc_legacy_stamp_test.go:116: the old home changed:
        added .settings.lock
        added VERSION
        added settings.json
FAIL
FAIL	github.com/gridconsole/grid-plugins/internal/settings	0.061s
--- FAIL: TestXcLegacyHomeNotStamped (0.01s)
    --- FAIL: TestXcLegacyHomeNotStamped/Lock (0.00s)
        xc_legacy_stamp_test.go:131: Lock on an old home: error <nil>, want one containing "Run: grid migrate"
        xc_legacy_stamp_test.go:134: VERSION written: "1\n"
        xc_legacy_stamp_test.go:137: the old home changed:
            added .xc.lock
            added VERSION
    --- FAIL: TestXcLegacyHomeNotStamped/Machine (0.00s)
        xc_legacy_stamp_test.go:131: Machine on an old home: error <nil>, want one containing "Run: grid migrate"
        xc_legacy_stamp_test.go:134: VERSION written: "1\n"
        xc_legacy_stamp_test.go:137: the old home changed:
            added .machine.lock
            added VERSION
            added machine
    --- FAIL: TestXcLegacyHomeNotStamped/Init (0.00s)
        xc_legacy_stamp_test.go:131: Init on an old home: error <nil>, want one containing "Run: grid migrate"
        xc_legacy_stamp_test.go:134: VERSION written: "1\n"
        xc_legacy_stamp_test.go:137: the old home changed:
            added VERSION
FAIL
FAIL	github.com/gridconsole/grid-plugins/internal/localdata	0.182s
FAIL
```

`TestXcFreshHomeStamped` and `TestXcAppFilesStillFresh` pass on base, as they should: they pin what must stay. `TestXcLegacyRule` and `TestXcVersionedHomesUnchanged` (gridhome) call `gridhome.Legacy`, which does not exist on base, so they do not compile there.

## What changed

- `internal/gridhome/gridhome.go`: `Legacy(home) (bool, error)`, true when any `<tasks dir>/<task>/task.md` exists in `TasksDirs(home)`. It does not read VERSION; callers ask it only after their own read said VERSION is missing. An old home with projects but no task is still stamped (the known edge, said in the comment).
- Departure from the brief's literal rule: `Legacy` skips dot-prefixed folders (the `.new-*` staging folders). The brief's `<dir>/*/task.md` read as a shell glob skips them too, `nextID` already treats them as no task, and the existing test `TestNewSweepsOnlyStaleStagingFolders` (`internal/task/task_test.go:381`) puts half-written `.new-*/task.md` folders in a home with no VERSION and expects `New` to succeed. `TestXcLegacyRule` has a case for it.
- `internal/localdata/localdata.go` `Init`: when VERSION is missing and `Legacy`, return `errors.New(gridhome.FormatProblem(0))` before the `O_EXCL` create. `Machine` and `Lock` reach it.
- `internal/settings/store.go` `locked`: the same check, placed before `.settings.lock` is opened so a refused home gains no file (the existing setup test already insists on that for invalid setups).
- `internal/task/create.go` `New`: the same check, placed before `.task-alloc.lock`. Only `New` stamps. `checkFormat` is shared with `write.go:499` and `trash.go:104` (writers of an existing task, which never stamped) and is unchanged.
- `internal/cli/cli.go:313` comment corrected.
- Fixture helper named: `setup` in `internal/guard/guard_test.go` now writes `VERSION` `"1\n"` (its home holds a hand-made task and its tests make settings writes). No assertion changed.

## Notes for the backlog

- A first-write race: `Machine` on a brand-new home that read "VERSION missing" just before a concurrent `grid task new` stamps VERSION and renames its task in would see a task folder and refuse once with "Run: grid migrate"; a retry works.
- Reads and the writers of an existing old-home task (`approve`, `trash`, ...) still run on a home with no VERSION and do not stamp it, as before.

## Walk tests after the report

Run once more after this file was on disk: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList' && go test -count=1 ./internal/job/lint/`, result in the final message.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: a real `grid` binary against a real old home; the tests use a copy of the migrate fixture, and `claude`, `curl` and servers are outside this lane.

DONE (N1 CLOSED BY C-xc2-legacy-test-version)
