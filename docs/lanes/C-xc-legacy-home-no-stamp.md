# C-xc-legacy-home-no-stamp: the first write to an old unversioned Grid home no longer stamps VERSION 1

Base: lane branch cut from main at `103b9157` (the brief names main `44c7de5b`; the three stamp lines are where it says). The three stamps on base, none of which checks for old data:

- `internal/localdata/localdata.go:109`  `_, err = fmt.Fprintf(f, "%d\n", gridhome.FormatVersion)`
- `internal/settings/store.go:353`  `if err := atomicfile.Write(filepath.Join(home, "VERSION"), []byte(fmt.Sprintf("%d\n", gridhome.FormatVersion)), 0o644); err != nil {`
- `internal/task/create.go:119`  `if err := atomicWrite(filepath.Join(home, "VERSION"), []byte(fmt.Sprintf("%d\n", gridhome.FormatVersion))); err != nil {`

Verdict: NOT DONE. One existing test outside Touches asserts the old stamping (row N1). Everything else holds.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 TestXcFreshHomeStamped (task, settings, localdata) | HOLDS | Missing and empty home each end with VERSION `"1\n"`. `go test -count=1 -v ./internal/task ./internal/settings ./internal/localdata` shows `--- PASS: TestXcFreshHomeStamped` in all three (`xc_legacy_stamp_test.go` in each). The brief says `Store.Create`; the function is `task.Store.New`. |
| 2 TestXcLegacyHomeNotStamped (three files) | HOLDS | On a copy of `../migrate/testdata/owner-home` each site returns an error containing `Run: grid migrate`, VERSION stays absent, and the whole tree (path and sha256 of every file) is identical before and after. That includes no `.settings.lock`, `.task-alloc.lock`, `.xc.lock`, `machine` or `settings.json`. localdata is also tried through `Machine` and `Init`. `--- PASS: TestXcLegacyHomeNotStamped` shown for task, settings, localdata (`go test -count=1 -v`, see row 5). |
| 2 on base | HOLDS | The same test files on base `103b9157`, run before any production edit (`go test -count=1 -run TestXc ./internal/task ./internal/settings ./internal/localdata`, exit 1): see "Failing on base". |
| 3 TestXcAppFilesStillFresh (gridhome file plus task file) | HOLDS | A home with only `projects/p/project-roles.json`, `projects/p/docs/`, `projects/p/brain/`: `gridhome.Legacy` false (`internal/gridhome/xc_legacy_test.go`), and `Store.New` stamps `"1\n"` and makes `task.md` (`internal/task/xc_legacy_stamp_test.go`). Both `--- PASS` in row 5. |
| 4 TestXcVersionedHomesUnchanged (gridhome file) | HOLDS | `internal/gridhome/xc_legacy_test.go`: at VERSION 1 holding owner-home's tasks, `localdata.Lock`, a settings write and `task.New` all succeed and VERSION stays `"1\n"`; at VERSION 2 all three return text containing `Update grid.`. A third subtest makes `projects` a file so `Legacy` errors; `Lock` and the settings write still succeed, so neither asks `Legacy` when VERSION exists. `task.New` is not in that subtest because `nextID` reads the same folders and fails on it anyway. The test is an external `package gridhome_test` so it can import the writers. |
| 5a five packages | HOLDS | `go test -count=1 -v ./internal/gridhome/ ./internal/localdata/ ./internal/settings/ ./internal/task/ ./internal/migrate/` printed `ok` for all five, and `--- PASS: TestXcLegacyHomeNotStamped` for localdata, settings and task (gridhome has the rule test `TestXcLegacyRule` plus `TestXcAppFilesStillFresh` and `TestXcVersionedHomesUnchanged`). |
| 5b walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` -> `ok .../internal/cli 0.356s`. `go test -count=1 ./internal/job/lint/` -> `ok .../internal/job/lint 3.461s`. The app-repo walk tests do not apply to the plugin repo. |
| 6a make lint | HOLDS | `make lint` ran `go vet ./...`, no output, exit 0. |
| 6b make cross | CROSS_ROW |
| 6c diff stat | HOLDS | `git diff main --stat` names: the Touches files (`internal/gridhome/gridhome.go`, `internal/localdata/localdata.go`, `internal/settings/store.go`, `internal/task/create.go`, `internal/cli/cli.go` comment, the four new test files), one fixture helper (`setup` in `internal/guard/guard_test.go`, 3 lines), and this report `docs/lanes/C-xc-legacy-home-no-stamp.md`, which every lane keeps there (`ls docs/lanes`). |
| Importers' tests | HOLDS except N1 | List from `go list` (packages that import localdata, settings, task or gridhome): contract/formats, internal/account, agent, brain, braincmd, cli, doctor, gitx, golden, guard, hook, job, job/jobcmd, job/jobtest, land, localdata, mcp, migrate, orchestrator, ports, project, projectmd, projects, proof, routine, settings, signal, task, updates, usage, where, worker, workflow, lib, gridhome. `go test -count=1` on all: every package `ok` except `internal/cli` (N1). `internal/job/frontmatter` and `internal/job/lint` also `ok`. |
| New tests fail on base, pass on lane | HOLDS | See "Failing on base". |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change refuses a *write to a data folder of an old format* with the existing `FormatProblem(0)` text, as `FormatProblem` already does for VERSION 2. No agent, tool or hook is gated. |
| `grid migrate` still reaches an old home | HOLDS | The brief's grep (`localdata\.` in migrate and cli/migrate*) prints nothing on base and the branch does not touch it; `go test ./internal/migrate` is `ok`. |
| N1 `internal/cli` `TestTaskCommandsMigratedAndLegacyShape/legacy` | NOT DONE | See below. |

### N1: an existing test outside Touches asserts the old stamping

`internal/cli/task_shape_test.go:28-31` removes VERSION from a home that holds a legacy-shape task, then `:52` runs `grid task new` and expects it to succeed. That is the old stamping on an old home, which this brief removes: `task new` now returns `grid task new: This data is format 0; this grid reads 1. Run: grid migrate`.

```
--- FAIL: TestTaskCommandsMigratedAndLegacyShape (0.01s)
    --- FAIL: TestTaskCommandsMigratedAndLegacyShape/legacy (0.00s)
        task_shape_test.go:53: new: 1 grid task new: This data is format 0; this grid reads 1. Run: grid migrate
```

The file is not in Touches and the test body is not a fixture helper, so I left it. Suggested fix for the orchestrator to authorize: in the `legacy` case write VERSION `"1\n"` just before the `task new` call at `:52` (the shape check of the new task still holds), or assert the refusal there. `show` and `rename` in the same test still run on the home with no VERSION and pass.

## Failing on base

Test files only, production code untouched, base `103b9157` (exit 1):

```
--- FAIL: TestXcLegacyHomeNotStamped (0.00s)
    xc_legacy_stamp_test.go:137: New on an old home: error <nil>, want one containing "Run: grid migrate"
    xc_legacy_stamp_test.go:140: VERSION written: "1\n"
    xc_legacy_stamp_test.go:143: the old home changed:
        added .task-alloc.lock
        added VERSION
        added machine
        added personal/tasks/T-347-second-task/task.md ...
--- FAIL: TestXcLegacyHomeNotStamped (0.00s)
    xc_legacy_stamp_test.go:110: SetSkill on an old home: error <nil>, want one containing "Run: grid migrate"
    xc_legacy_stamp_test.go:113: VERSION written: "1\n"
        added .settings.lock / VERSION / settings.json
--- FAIL: TestXcLegacyHomeNotStamped/Lock, /Machine, /Init (localdata)
    VERSION written: "1\n"   (Lock adds .xc.lock + VERSION; Machine adds .machine.lock, VERSION, machine; Init adds VERSION)
```

`TestXcFreshHomeStamped` and `TestXcAppFilesStillFresh` pass on base, as they should (they pin what must stay).

## What changed

- `internal/gridhome/gridhome.go`: `Legacy(home) (bool, error)`. True when any `<tasks dir>/<task>/task.md` exists in `TasksDirs(home)`. It does not read VERSION; the callers ask it only after their own read said VERSION is missing. Hidden folders (`.new-*` staging) are skipped, as the shell glob `<dir>/*/task.md` does and as `nextID` does. An old home with projects but no task is still stamped (known edge, in the comment).
- `internal/localdata/localdata.go` `Init`: when VERSION is missing and `Legacy`, return `errors.New(gridhome.FormatProblem(0))` before the `O_EXCL` create. `Machine` and `Lock` reach it.
- `internal/settings/store.go` `locked`: same check, placed before `.settings.lock` is opened so a refused home gains no file (the existing setup test already insists on that for invalid setups).
- `internal/task/create.go` `New`: same check, placed before `.task-alloc.lock`. Only `New` stamps; `checkFormat` is shared with `write.go:499` and `trash.go:104` (writers of an existing task, which never stamped) and is unchanged.
- `internal/cli/cli.go:313` comment corrected.
- Fixture helper named: `setup` in `internal/guard/guard_test.go` now writes `VERSION` `"1\n"` (its home holds a hand-made task and the tests make settings writes). No assertion changed.

## Notes for the backlog

- A first-write race: `Machine` on a fresh home that reads "VERSION missing" just before a concurrent `grid task new` stamps VERSION and renames its task in would see a task folder and refuse once with "Run: grid migrate"; a retry works. Narrow and only on a brand-new home.
- Reads and `grid task` writers on an existing old-home task (`approve`, `trash`, ...) still run on a home with no VERSION and do not stamp it, as before.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: a real `grid` binary against a real old home (the tests use the migrate fixture copy; `claude`, `curl` and servers are out of scope for this lane).
