# C-pcov3-jobcmd-sources-project-record

Tests for broken project records and repo roots in `grid job render` (sources, projectOf, roots). One new file: `internal/job/jobcmd/project_record_test.go`. No non-test file changed.

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1 | Coverage before/after (`go test -count=1 -coverprofile=/tmp/C-pcov3-jobcmd-sources-project-record-jobcmd.out ./internal/job/jobcmd/ && go tool cover -func=... \| tail -1`, exit 0) | HOLDS. Before (untouched clone, linux): `total: (statements) 84.4%`. After: `total: (statements) 87.3%`. After is above before. `sources` 100.0%, `roots` 100.0%, `projectOf` 88.9% (the `filepath.Rel` error return at :605-606 is not reachable with two absolute paths and is not in the brief). |
| 2 | task.md a directory -> :544-545 error; task.md that does not parse -> `task.md: ` (:550); nothing written under jobs | HOLDS. `TestSourcesTaskMdUnreadable`, `TestSourcesTaskMdDoesNotParse` (project_record_test.go). `render` does not refuse earlier: `loadJob` (:110) does not read task.md, so the test goes through `grid job render ... --json` (exit 1, `error` equals the sources error), `LintOptions` and `sources` directly. The unreadable case asserts the bare read error (it names the task.md path, no `task.md: ` prefix). The parse case asserts `task.md: line 1: missing frontmatter block`. Both compare the listing of `<task>/jobs` before and after render: unchanged. |
| 3 | projectOf, 4 cases + "" cases give no error and no repos from sources | HOLDS. `TestProjectOfPlaces`: gridHome "" -> "", task outside the Grid home -> "", `projects/demo/notes/a` -> "", `projects/demo/tasks` itself -> "" (extra), `projects/demo/tasks/T-01` -> "demo". `TestSourcesSkipsRootsWithoutProject`: with a project.md, folder and repo in place, a task outside the Grid home and an Env with GridHome "" both give no error, project "", no folders, no repos (:560). |
| 4 | roots: no project.md; project.md a directory; project.md not parsing; no folder on this machine | HOLDS. `TestRootsProjectRecord`: no project.md -> no error, no folders, no repos (:618-619); directory -> bare read error naming the path, not prefixed `project ` (:621-622); `folders: nothing` -> `project demo: no front matter between --- lines` (:626); `---\nfolders: [\n---\n` -> prefix `project demo: front matter: `; folder only for machine `other`, and a record with no folders -> no error, no repos (:630-631). |
| 5 | repo path `../x` and an absolute outside path refused; `sub/repo` accepted | HOLDS. `TestRootsRepoPaths`: exact message `project demo: repo path "<p>" must stay inside the project folder` for both, and for a bad path after a good one. `sub/repo` -> `Repo` and `Repos["sub/repo"]` are `<project folder>/sub/repo`. Extra: first repo `.` makes the primary repo the folder and names no repo (:643). |
| 6 | prototype relative joined, absolute kept | HOLDS. `TestRootsPrototype` asserts the returned roots (`world.files.Prototype`; roots is what the code exposes): relative `design/proto.html` -> `<project folder>/design/proto.html`; absolute -> unchanged (:647-651). Extras: none named -> "", and a record with no folder on this machine leaves it unset. |
| 7 | Windows pin | compiled, needs Windows run. Subtest `TestRootsRepoPaths/windows_forms_are_refused` gives `..\x` and `D:\other` and expects the :636-637 refusal; it skips on other OSes (`--- SKIP ... compiled, needs Windows run`). Command: `GOOS=windows go test -c -o prrecord.test.exe ./internal/job/jobcmd/`, run on Windows with `-test.run TestRootsRepoPaths`. It compiles (row 8). |
| 8 | Checks | HOLDS. `go test -count=1 ./internal/job/jobcmd/ ./internal/cli/` exit 0, both `ok`. `GOOS=windows go vet ./internal/job/jobcmd/` exit 0. `GOOS=darwin go vet ./internal/job/jobcmd/` exit 0. `GOOS=windows go test -c -o /dev/null ./internal/job/jobcmd/` exit 0. `make lint` (`go vet ./...`) exit 0. `gofmt -l internal/job/jobcmd` prints nothing. |
| R1 | Packages that import the changed one | `go list` shows `internal/cli` imports `internal/job/jobcmd`; both tested above (cli 63s, ok). Whole-module test and cross-repo set are left to the orchestrator. |
| R2 | New tests fail on base, pass on the lane | HOLDS in the form a tests-only lane allows. The code on base is the code on the lane, so the tests pass on both. To show they bite, I mutated `command.go:636` to `if false && !filepath.IsLocal(r.Path)` (reverted with `git checkout`, `git status` clean after): `TestRootsRepoPaths` failed with `got <nil>, want project demo: repo path "../x" must stay inside the project folder` (also the absolute-path case and the bad-after-good case). |
| R3 | Tests only, no existing test edited, no guard/gate/cap | HOLDS. `git diff --stat origin/main..HEAD` shows only project_record_test.go and this report. Helpers reused: `fixture`, `call`; new helpers are prefixed `pr`. No test writes, removes or reads approval.json. No chmod, no special files. |
| R4 | Commits | One commit, one-sentence message, no attribution line. No personal paths, hosts or emails in the file. |

## What changed

- New `internal/job/jobcmd/project_record_test.go` (7 test functions, 23 subtests). Uses the real fixture (`jobtest.New`) and rewrites task.md and project.md per case.
- This report.

## Outcome notes

- The code did what the brief expected in every row; no bug found, no assertion left out.
- For the task.md parse case the full message today is `task.md: line 1: missing frontmatter block` (from `frontmatter.Parse`); that is what is asserted, which also starts with `task.md: `.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Row 7 (Windows forms of :636-637) needs a Windows run, and nothing else needs a macOS run beyond the vet above (macOS `go vet` is compile-only proof).

If lane M-plugin-held-f-e lands first, rebase and run `go test -count=1 ./internal/job/jobcmd/` again.

DONE
