# C-pcov5-jobcmd-save-verify

Proof table. Base is plugin main d0400870; the lane head is the commit after it that adds `internal/job/jobcmd/save_verify_failures_test.go`.

| Row | Result | Proof |
|---|---|---|
| 1 Coverage before | HOLDS | `go test -count=1 -coverprofile=<own folder>/before.out ./internal/job/jobcmd/` exit 0: `coverage: 92.9% of statements`; `go tool cover -func ... \| tail -1`: `total: (statements) 92.8%` (the profile total; the row's 92.9% is the package line). |
| 2 save.go:26-28 mkdir error | HOLDS | `TestJsvSaveJobsFolderCannotBeMade`, save_verify_failures_test.go; error starts `mkdir `, names `T-01/jobs`, the jobs file is unchanged. `go test -run TestJsv` exit 0. Block count 0 -> 1 (`save.go:27.3,28.1 1 1`). |
| 3 save.go:29-32 jobs leaves task | HOLDS | `TestJsvSaveJobsFolderLeavesTask`; error contains `source escapes its root`; the outside folder stays empty. Count 0 -> 1 (`save.go:31.3,32.1 1 1`). |
| 4 save.go:46-49 linked file | HOLDS | `TestJsvSaveLinkedFileIsDifferentInputs`; second save returns exactly `Job J-1 already has different saved inputs; create a new Job for revised work`; inputs.json and job.yaml unchanged. Count 0 -> 1 (`save.go:48.5,49.1 1 1`). |
| 5 verify --json closed stdout | HOLDS | `TestJsvVerifyReportWriteFails`: control exits 3 with stdout `exit: 3\ntests: unknown\ncommit: \npass: false\nreason: a reviewer must check this Job\n`; subtest `json_closed_stdout` exits 1. Count 0 -> 1 (`verify.go:43.4,44.1 1 1`). |
| 6 verify plain closed stdout | HOLDS | Subtest `plain_closed_stdout` exits 1. Count 0 -> 1 (`verify.go:51.4,52.1 1 1`). |
| 7 verify reason write | HOLDS | Subtest `plain_reason_line_fails` (first write passes, `reason:` fails) exits 1. Count 0 -> 1 (`verify.go:55.5,56.1 1 1`). |
| 8 loadVerifyJob not a directory | HOLDS | `TestJsvVerifyJobFolderIsFile`: exit 1, stderr starts `job: `; skipped on Windows. Count 0 -> 1 (`verify.go:68.3,69.1 1 1`). |
| 9 loadVerifyJob read error | HOLDS | `TestJsvVerifyJobYamlIsFolder`: exit 1, stderr starts `job: read ` and names `jobs/J-1/job.yaml`. Count 0 -> 1 (`verify.go:72.3,73.1 1 1`). |
| 10 Coverage after and gates | HOLDS | Same command exit 0: `coverage: 93.9% of statements`, total 93.9% (before 92.9% / 92.8%). `go test -count=1 ./internal/job/jobcmd/` exit 0. `make lint` (`go vet ./...`) exit 0. `GOOS=windows go vet ./...` exit 0. |
| Rule: tests of changed and importing packages | HOLDS | `go list` importers of jobcmd: `internal/cli`, `internal/job/jobcmd`. `go test -count=1 ./internal/job/jobcmd/ ./internal/cli/` exit 0. |
| Rule: Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0; `GOOS=darwin go vet ./internal/job/...` exit 0. |
| Rule: gofmt | HOLDS | `gofmt -l internal/job/jobcmd` printed nothing. |
| Rule: test-only, one new file, jsv prefix | HOLDS | Only `save_verify_failures_test.go` (plus this proof file). Helpers: jsvClosedWriter, jsvTask, jsvSaveReviewerJob, jsvSymlink, jsvRead; tests are `TestJsv*`. No chmod, Mkfifo or syscall; os.Symlink is behind a skip; OS error words are not asserted. No approval.json, no Approve/TryApprove. |
| Rule: new tests fail on base | NOT APPLICABLE | The tests cover code that already exists on base, so they pass there. What they add is coverage: the nine blocks above were count 0 in before.out and are count 1 in after.out. No failing line to paste. |
| Rule: no guard, gate, cap or hook | HOLDS | Nothing outside a test file changed. |
| Rule: small commits, no attribution | HOLDS | One commit with a one-sentence message and no attribution lines. |

## What changed

One new test file with six tests (nine rows, rows 5-7 as subtests of one test). No code change.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run on a real Windows or macOS host: only the cross-builds and `go vet` ran.

## Notes

- Row 1 asks for a `mktemp -d` folder. mktemp and mkdir outside the clone were refused in this sandbox, so the profiles went to an untracked `.jsvcov/` folder in the clone. I could not remove it (`rm` is not allowed). It is not committed; delete it.
- lane-M-plugin-held-f-e (jobtest.go) was not on main when I started; the fixture row (5-7) should be rerun if it lands first.
