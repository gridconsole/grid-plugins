# C-pcov3-jobcmd-trace-problems

Tests for the problem text of `grid job trace` on unsafe or broken task folders. Tests only: one new file, `internal/job/jobcmd/trace_problems_test.go`. No non-test file changed (`trace.go` was changed three times to prove the tests fail on a wrong string, then restored with `git checkout`; `git status` was clean afterwards).

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1 Coverage before / after | HOLDS | Before (untouched clone, Linux): `total: (statements) 84.6%` (package line 84.7%). After: `total: (statements) 87.3%`. Command: `go test -count=1 -coverprofile=/tmp/C-pcov3-jobcmd-trace-problems-jobcmd.out ./internal/job/jobcmd/ && go tool cover -func=... \| tail -1`, exit 0. After: tracePath, traceProject 100%; Trace 96.8%; traceScanTask 90.9%; runTrace 95.0%. |
| 2 Home and project | HOLDS | `TestTraceHomeAndProjectErrors` (no home :157; unreadable records :128 via a `project.md` with no front matter and a filter that names no folder; ambiguous id :134 via two projects with the same id, also through the command as exit 1 with `job trace: project id is ambiguous`; a prefix of an id gives project not found). tasks/ a regular file: `TestTraceProjectTasksFile` gets the problem `project "beta": cannot list tasks` (:185), so that is the answer for the brief's trigger. "cannot list project task folders" (:161) is NOT REACHED by tasks/ being a file, because `gridhome.TasksDirs` only reads `projects/` and never touches `tasks/`; it is reached when `projects` itself is a regular file, and that is asserted (subtest "projects is a regular file"). |
| 3 Symlinks | HOLDS | `TestTracePathRefusals`: symlinked part (link inside the root) -> `symbolic link refused` (:110-111); a plain missing part gives the not-exist error from `job.Within`'s EvalSymlinks, which runs before the Lstat, so the Lstat branch (:107-108) is reached with `absent/../b` (Within cleans the `..`), asserted as an `lstat` not-exist PathError. `TestTraceLinkedTaskFolder`: a `T-03-link` symlink to a task folder gives exactly `{Task: T-03-link, Text: "task folder is unreadable or unsafe"}` and exactly one row (target traced once). Skips with `t.Skipf` when `os.Symlink` fails (`tpLink`). Also `TestTraceProjectTaskFolderUnsafe`, `TestTraceBundleSymlinkProblems` for :180, :245, :271. |
| 4 Task problems | HOLDS | `TestTraceTaskProblems`, one subtest each: task folder a regular file; task.md a folder; intent.md missing; task.md naming another project. Each asserts the exact problem and that T-01/J-1 still gives its row. |
| 5 Bundle problems | HOLDS | `TestTraceBundleProblems`: jobs/ a regular file -> `cannot list Job bundles`; bundle a regular file -> `Job bundle is unreadable or unsafe`; inputs.json a folder -> `inputs.json must be a regular file`; inputs.json `not json` -> `invalid inputs.json`. The other bundle/task still traces. |
| 6 runTrace exit | HOLDS | `TestTraceCommandProblems`: exit 0 with problems, text lines `problem: "T-01" "J-2": invalid inputs.json` and `problem: "T-01" "J-3": inputs.json must be a regular file`, and the same two in the `--json` problems list. `TestTraceCommandNoHome`: `Env.GridHome ""` exits 1, stderr exactly `job trace: cannot find the Grid home; set GRID_HOME\n`; with `--json`, exit 1 and stdout holds the one problem with that text, no stderr. |
| 7 Checks | HOLDS | `go test -count=1 ./internal/job/jobcmd/` exit 0 (`ok`). `GOOS=windows go vet ./internal/job/jobcmd/` exit 0. `GOOS=darwin go vet ./internal/job/jobcmd/` exit 0. `GOOS=windows go test -c -o /dev/null ./internal/job/jobcmd/` exit 0. `make lint` exit 0 (`go vet ./...`). `gofmt -l internal/job/jobcmd` empty. |
| Importing packages | HOLDS | `go list` over imports and test imports: only `internal/cli` imports `internal/job/jobcmd`. `go test -count=1 ./internal/cli/` exit 0 (`ok`, 55.9s). |
| New tests fail on base | HOLDS (by mutation) | The tests assert code that already exists, so on base (no test file) there is nothing to fail. To show they bite, three strings in trace.go were changed and the tests run: `FAIL TestTracePathRefusals/symlinked_part: got link, want symbolic link refused`; `FAIL TestTraceHomeAndProjectErrors/ambiguous_project_id: got ambiguous`; `FAIL TestTraceBundleProblems/inputs.json_is_not_json: got {Task:T-01 Job:J-2 Text:bad inputs}, want {... Text:invalid inputs.json}`; `TestTraceCommandProblems` failed too. trace.go was then restored. |
| No existing test edited or deleted | HOLDS | `git diff origin/main..HEAD --stat`: one added file plus this report. |
| No guard, gate, cap or hold added | HOLDS | Test file only. |
| No approval.json | HOLDS | None of the new tests writes, removes or asserts on it. |
| No personal paths, hosts or emails | HOLDS | Test file uses `t.TempDir()` only. |

## What changed

- `internal/job/jobcmd/trace_problems_test.go` (new): `TestTracePathRefusals`, `TestTraceHomeAndProjectErrors`, `TestTraceProjectTasksFile`, `TestTraceProjectTaskFolderUnsafe`, `TestTraceLinkedTaskFolder`, `TestTraceTaskProblems`, `TestTraceBundleProblems`, `TestTraceBundleSymlinkProblems`, `TestTraceCommandProblems`, `TestTraceCommandNoHome`. Helpers are prefixed `tp` and reuse `traceTask`, `traceTestBundle`, `traceWrite`, `traceCall`, `ownerResolver`.
- Where the code differs from the brief: the brief's "missing part" Lstat trigger and the "tasks/ a file" trigger for :161 (see rows 2 and 3). The code wins and is asserted.
- Not added (brief says not picked): failing writers for :310, :322, :364, :376; chmod-only read errors. Lines :266 and :281 (stat/read errors after a successful check) have no trigger without chmod or a race.

## NOT VERIFIED

- Windows and macOS runs: compiled only. Run `GOOS=windows go test -c -o /dev/null ./internal/job/jobcmd/` (done, exit 0); needs a Windows run of `go test ./internal/job/jobcmd/` and a macOS run (the symlink subtests skip on Windows without the privilege).
- Whole-module test and cross-repo set: left to the orchestrator at landing. If lane M-plugin-held-f-e lands first, rebase and rerun `go test -count=1 ./internal/job/jobcmd/`.
