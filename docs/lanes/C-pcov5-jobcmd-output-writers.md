# C-pcov5-jobcmd-output-writers

Test-only lane: one new file, `internal/job/jobcmd/output_writers_test.go` (helpers all prefixed `ow`). Base: main d0400870.

## Proof table

| Row | Result |
|---|---|
| 1 Coverage before | HOLDS: `go test -count=1 -coverprofile=<own file> ./internal/job/jobcmd/` exit 0, `coverage: 92.9% of statements` |
| 2 Usage write (command.go:88-89) | HOLDS: `TestOWUsageWriteFails` passes; profile line `command.go:88.4,89.1 1 1` |
| 3 output JSON (:351-352) | HOLDS: `TestOWOutputWriteFails` (control `lint <brief> --json` exit 0; closed stdout exit 1); profile `command.go:351.4,352.1 1 1` |
| 4 output plain (:390-391) | HOLDS: same test, control prints exactly `path: <display path>\nok: true\n`, closed stdout exit 1; profile `command.go:390.3,391.1 1 1` |
| 5 output error line (:358-359) | HOLDS: `TestOWErrorLineWriteFails` (`lint a b` exit 2 with healthy stderr, exit 1 with closed stderr); profile `command.go:358.4,359.1 1 1` |
| 6 lint task (:201-202) | HOLDS: `TestOWLintTaskNotResolved`, exit 1, stderr exactly `job: task T-99 needs --project: this folder is in no project with a record\n`; profile `command.go:201.5,202.1 1 1` |
| 7 show read (:234-235) | HOLDS: `TestOWShowBriefIsDirectory`, exit 1, stderr prefix `job: read ` and contains `T-01/jobs/J-1/brief.md` (built with filepath.Join); profile `command.go:234.4,235.1 1 1` |
| 8 Trace query check (trace.go:154-155) | HOLDS: `TestOWTraceQueryCheck`, error text exact; profile `trace.go:154.3,155.1 1 1` |
| 9 Non-task folder (trace.go:190) | HOLDS: `TestOWTraceSkipsNonTaskFolder`, one T-01 row, no problems (traceTestRows asserts `[]`); profile `trace.go:190.5,190.13 1 1` |
| 10 trace writers | HOLDS: `TestOWTraceWritersFail` (json, plain, help, error line subtests; healthy-buffer controls first); each asserts `traceExitFailed`; profile `trace.go:364.4,365.1`, `:376.3,377.1`, `:322.5,323.1`, `:310.4,311.1` all count 1 |
| 11 Coverage after and gates | HOLDS: rerun prints `coverage: 94.7% of statements` (before 92.9%); `go test -count=1 ./internal/job/jobcmd/` exit 0; `make lint` (go vet ./...) exit 0; `GOOS=windows go vet ./...` exit 0 |
| Importers tested | `go list` shows `internal/cli` as the only package importing jobcmd; `go test -count=1 ./internal/cli/` exit 0 |
| Windows and macOS builds | HOLDS: `GOOS=windows go build ./...` exit 0, `GOOS=darwin go build ./...` exit 0 |
| New tests fail on base | NOT APPLICABLE: the lane adds tests for code that already behaves as the rows say, so they pass on base once the file exists; the proof is the blocks' count 0 before (brief) and count 1 after (profile lines above). Nothing to paste as failing. |
| Only the one file changed | HOLDS: code change is one new file; this report is the only other file (`docs/lanes/`) |
| No existing test edited or deleted | HOLDS: `git diff origin/main..HEAD --stat` shows only added files |
| No guard/gate/cap on agents; no approval.json; no Approve/TryApprove | HOLDS: test file has none (grep) |
| Nothing Unix-only | HOLDS: no chmod, Mkfifo, syscall or symlink; OS error text not asserted; `GOOS=windows go vet ./...` exit 0 |
| No attribution lines, no personal paths | HOLDS |

## What changed

`internal/job/jobcmd/output_writers_test.go`: `owClosedWriter` (every Write errors), `owSavedBrief` (renders T-01 1 on the fixture and returns the saved brief), `owRun`, `owTraceHome`, and eight tests covering rows 2-10.

## Notes

- `command.go:90` (`return 0` after a successful usage write) still has count 0 in this package's own profile; it is not one of the brief's rows.
- Coverage profiles were written to files under `/srv/gridwork/tmp` (mktemp and mkdir were refused in the sandbox), not to a shell variable folder.
- If lane-M-plugin-held-f-e lands first (edits jobtest.go), rerun `go test -count=1 -run TestOW ./internal/job/jobcmd/`.

## NOT VERIFIED on a live screen

Nothing here is UI; no live check is needed.
