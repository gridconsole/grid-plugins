# C-run-job-post-findings: grid job post reports the lint findings render will refuse on

Base: main at 78b08bc9. Test commit 71beccb3 (tests only, on main's code); implementation commit follows it.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Test that fails on main | HOLDS | On 71beccb3 with main's code (the three source files stashed): `go test -count=1 -run 'TestJobPost(Reports|Prints|Clean|Findings)' ./internal/cli/` fails. First failing line: `job_post_findings_test.go:100: no job.read-range finding for task:intent.md: {"id":"01M74AP9T4W...","type":"job",...,"job":"J-2",...}` (the output has no `findings`). Also `TestJobPostPrintsFindingsOnStderr` (stderr lacks the line), `TestJobPostCleanRecordMatchesJobLint` (`no findings array`), `TestJobPostFindingsUnavailableStillPosts` (`findings should be []`) fail on main. The fixture is the task folder of the run (intent.md `## Request {#request}`, inputs/H-website-docs-report.md 34 lines, inputs/draft/README.md 46 lines, inputs/old-docs/README.md); the record is `firstJob()` (task_new_first_job.go:25) with J-1's four `read` pointers. |
| 2. Post runs lint.CheckJSON with render's Options from the exported constructor | HOLDS | internal/cli/orchestrator.go `postFindings` calls `jobcmd.LintOptions` then `lint.CheckJSON`; `jobcmd.LintOptions` (internal/job/jobcmd/command.go) wraps `sources` and `(*world).lintOptions`, the same method render (`lint.Check(skeleton, w.lintOptions())`) and `grid job lint` now call. No copy of the Options code. |
| 3. Post still succeeds; findings in `--json` and on stderr | HOLDS | `go test -count=1 -run TestJobPost ./internal/cli/` exit 0. `TestJobPostReportsTheFindingsRenderRefusesOn` asserts exit 0, `findings` beside the record's own fields, and that the stored record (type job, `read`, id) is the one posted. `TestJobPostPrintsFindingsOnStderr` asserts the exact line `grid job post: J-2: job.read-range: Read pointer has no range: task:intent.md (render will refuse this Job)` and that stdout is still the record id alone. `TestJobPostWarningHasNoRefusalText` asserts a warning has no bracketed part. |
| 4. Clean record; `--person` and non-job types not linted | HOLDS | `TestJobPostCleanRecordMatchesJobLint`: J-2's read list gives findings equal (reflect.DeepEqual) to `grid job lint <file> --task <task> --json`, no error, no "render will refuse". `TestJobPostPersonJobIsNotLinted` (no `findings`, no stderr, with and without `--json`) and `TestJobPostOtherTypesAreNotLinted` (`--type control`, no `findings`, no stderr). |
| 5. No flag, inventory and generate unchanged | HOLDS | `make generate` then `git status --short` prints nothing. contract/commands.json and contract/schemas untouched (`git diff --stat 78b08bc9 HEAD` lists only the files below). |
| 6. Options cannot be built | HOLDS | `TestJobPostFindingsUnavailableStillPosts`: the project record has a repo path `../outside`, `sources` fails, the post exits 0, the stored record exists, `"findings":[]`, one stderr line `grid job post: J-2: the findings could not be checked: ...`. Exit codes on main (same tests, main's code): every post in the new tests exits 0 on main (the exit check passes and the test fails later at the findings assertion; person and `--type control` tests pass on main). |
| 7. Tests, lint, Windows | HOLDS | `go test -count=1 ./cmd/grid/ ./internal/cli/ ./internal/job/... ./internal/orchestrator/` exit 0 (ok cli 36s, job, frontmatter, jobcmd, lint, orchestrator 33s). `make lint` exit 0. `GOOS=windows CGO_ENABLED=0 go vet ./internal/cli/` exit 0. `GOOS=darwin CGO_ENABLED=0 go build ./...` and `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |

Packages importing the changed ones (go list): jobcmd is imported by internal/cli; internal/cli is imported by cmd/grid. Tested: cmd/grid (no test files), internal/cli, internal/job/..., internal/orchestrator.

## Rules

| Rule | Result | Proof |
|---|---|---|
| Small commits, one-sentence messages, no attribution lines | HOLDS | `git log --format=%B 78b08bc9..HEAD`: two commits, no Co-Authored-By. |
| Public repo: no personal paths, hosts or emails | HOLDS | The diff holds none; test fixtures use temp folders. |
| New tests fail on base, pass on lane | HOLDS | Row 1; lane run `go test -count=1 -v -run 'TestJobPost(Reports|Prints|Clean|Person|Other|Findings|Warning)' ./internal/cli/` exit 0, all 7 PASS. |
| No existing test edited or deleted | HOLDS | Diff touches no existing test file. |
| No guard, gate, cap, hold or blocking hook | HOLDS | Post adds no refusal: the findings are printed after the write succeeded, and a failed Options build only prints a line. |
| Tests of changed packages and importers, lint, Windows and macOS builds | HOLDS | Row 7. |

## What changed

- internal/job/jobcmd/command.go: `(*world).lintOptions` and exported `LintOptions(task, Env)`; render and `grid job lint` use `lintOptions` (same Options, no behaviour change).
- internal/cli/job.go: the jobcmd Env construction moved into `jobcmdEnv` so post builds the same Env (no behaviour change).
- internal/cli/orchestrator.go: for `--type job` without `--person`, lint the body before the write and report after a successful write: `findings` in `--json` output beside the record's fields, one stderr line each otherwise.
- internal/cli/job_post_findings_test.go: new tests.

Note: render lints a skeleton made from the Job's metadata; post runs `lint.CheckJSON` on the record, as the brief says. The two can differ on a rule that only one of them runs; the J-1 rule (`job.read-range`) is in both.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Not run: a real worker assigning the Job and render refusing it (needs the app and `claude`); the tests cover the post side only.
