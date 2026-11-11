# Lane C-pcov3-jobcmd-render-show-refusals

Tests for the refusals of `grid job render|lint|show` (jobcmd command.go). One new file, `internal/job/jobcmd/command_refusals_test.go`. No non-test file changed.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 Coverage before/after | HOLDS | before (untouched clone, linux): `total: (statements) 84.4%`; after: `total: (statements) 86.1%`. Command: `go test -count=1 -coverprofile=/tmp/C-pcov3-jobcmd-render-show-refusals-jobcmd.out ./internal/job/jobcmd/ && go tool cover -func=... \| tail -1`, exit 0. After is above before. |
| 2 Flag refusals | HOLDS, one difference | `TestJobFlagAndCommandRefusals` (command_refusals_test.go:34): --dir on render, lint, show (exit 2, `job: --dir is only supported by verify`); `--task`/`--budget`/`--dir` with no value (`job: --X needs a value`); --budget 0, 2001, x (`job: --budget must be between 1 and 2000`); unknown flag (`job: unknown flag --bogus`); unknown command (`job: unknown job command "frobnicate"; use render, lint or show`); each also proves no `jobs/` folder was made. Also the --json form. `TestRenderAcceptsTheBudgetEnds`: --budget 2000 renders (exit 0). **Code wins:** --budget 1 passes parse (`parse` returns budget 1) but render exits 1 on this fixture with `brain budget too small for the elision banner` (from job.Render); the test asserts that. |
| 3 History append failure | HOLDS | `TestRenderHistoryFailureStillRenders`: history.md made a directory; render exits 0, stderr is `grid job render: history: history must be a regular file\n` twice (one per note given: deploy and the recalled export-format), brief.md and inputs.json written, history.md still a directory. AppendHistory does fail on a directory. |
| 4 Guest lint, --task another invited task | HOLDS | `TestGuestLintTaskNamingAnotherInvitedTask/guest`: guest `gus` invited (project-roles.json) to T-01 and T-02 (copy); `lint <T-01 brief> --task T-02` exits 1, stderr exactly `job: task not found\n`, stdout empty, no host path; the same file with `--task T-01` exits 0. `/owner`: the same call exits 0, no stderr. |
| 5 show refusals | HOLDS, one difference | `TestShowRefusals`: `show T-99 J-1` with the real resolver exits 1 with `job: task T-99 needs --project: this folder is in no project with a record` (the where resolver answers before :436; **code wins**); with `Env.ResolveTask` returning ("", nil) it exits 1 with `job: cannot find task "T-99"; pass a task folder (one holding task.md, intent.md or plan.md)` (:436). Task given twice: `job: give the task once`. Zero saved Jobs: `job: task has 0 saved Jobs; specify a Job id`; two: `... 2 saved Jobs ...`. One saved Job, `show --task T-01 --json`: ok, id J-1, Brief equals the saved file. `show X-1 --task T-01`: `job: invalid Job id`. No existing test asserts "task has" or show's "invalid Job id" (feedjob_save_test.go:171 is the save path), so none was dropped. |
| 6 Broken files | HOLDS | `TestShowBrokenFiles`: brief replaced by `not a brief`; `show T-01 J-1` exits 1 with `job: ` + the exact `job.DecodeBrief` error, brief unchanged. Feed: task.md given a ULID id and `feed/<ULID>.jsonl` made a directory; show exits 1, stderr `job: read <path>/feed/<ULID>.jsonl: is a directory`; the test asserts the `job: ` prefix and the file name (OS-neutral), brief and feed directory unchanged. |
| 7 Checks | HOLDS | `go test -count=1 ./internal/job/jobcmd/` ok, 0 fail (exit 0); `GOOS=windows go vet ./internal/job/jobcmd/` exit 0; `GOOS=darwin go vet ./internal/job/jobcmd/` exit 0; `GOOS=windows go test -c -o /dev/null ./internal/job/jobcmd/` exit 0; `make lint` exit 0 (`go vet ./...`); `gofmt -l internal/job/jobcmd/` empty. |
| Importers | HOLDS | `go list` (Imports, TestImports, XTestImports) of packages importing jobcmd: `internal/cli`. `go test -count=1 ./internal/cli/` ok (57.6s, exit 0). |
| New tests fail on base | NOT APPLICABLE | The lane adds tests only, for behaviour main already has; on base the file does not exist, so there is nothing to fail. No non-test file changed, so no assertion can be shown failing on base. |
| No existing test edited/deleted | HOLDS | `git diff origin/main --stat` lists only the new test file and this record. |
| No guard/gate/cap | HOLDS | Tests only. |
| No approval.json | HOLDS | The new tests never write, remove or assert on approval.json (they reuse `givenFixture`, whose helper is outside this file). |

## Not covered, as the brief said

- :147 "brain index changed during render": NOT REACHED. The package has no seam between the two `w.generation()` reads: `readFeed` is the only variable, `brain.Store.Search` (called by recall between the reads) does not rebuild the index, and `job.Render` only reads. No row.
- :130-135 `job.plan-approval` exit 2: dead code (no rule emits it; plan rule removed, no_plan_rule_test.go). Not removed.
- :108-125, :201/:209/:234, :351-391 and :510: no row, per the brief.

## What changed

- Added `internal/job/jobcmd/command_refusals_test.go` (reuses `fixture`, `call`, `givenFixture`, `newBuffers`; own helpers `refusalRun`, `refusalWant`).
- Added this record.

## NOT VERIFIED on a live screen

None; there is no UI. The Windows and macOS builds are only compiled (`GOOS=windows|darwin go vet`, `GOOS=windows go test -c`); a run needs a Windows or macOS host: `go test -count=1 ./internal/job/jobcmd/`.
