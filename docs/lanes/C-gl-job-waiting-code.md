# C-gl-job-waiting-code: the job_waiting object carries `"code":"job_waiting"`

Base: plugin main 2d92aa42fcbf20c2491c6ca34558fcac9d56a911. On base the object at internal/cli/task_write.go:197-202 had no `code` key, so the brief applies.

## Proof table

| Row / rule | Result |
| --- | --- |
| 1. TestGlJobWaitingCode: keys error, code, job, waiting_for, reason in that order, code `job_waiting`, exit 3, DisallowUnknownFields into a five-field struct | HOLDS. internal/cli/gl_job_waiting_code_test.go; `go test -count=1 -v -run 'TestGlJobWaitingCode\|TestFu2ApproveJobWaiting' ./internal/cli/` exit 0, `--- PASS: TestGlJobWaitingCode`. On base it fails: `gl_job_waiting_code_test.go:48: keys error,job,waiting_for,reason, stdout {"error":"job_waiting","job":"J-2","waiting_for":"J-1","reason":"waiting for J-1 to be accepted"}` / `--- FAIL: TestGlJobWaitingCode` |
| 2. Without --json stdout empty, stderr unchanged | HOLDS. TestFu2ApproveJobWaitingWithoutJSON (fu2_approve_job_waiting_test.go:99-111, unedited) asserts stdout empty and stderr exactly `grid approve: J-2 cannot be answered yet: waiting for J-1 to be accepted\n`. It PASSES on base (run before the fix) and on the lane. The `--json` branch is the only code changed; the stderr line (task_write.go:194) is untouched. |
| 3. `go test -count=1 -v -run 'TestGlJobWaitingCode\|TestFu2ApproveJobWaiting' ./internal/cli/` | HOLDS, exit 0: PASS TestFu2ApproveJobWaitingJSON, TestFu2ApproveJobWaitingWithoutJSON, TestFu2ApproveJobWaitingRoles, TestGlJobWaitingCode; `ok`, 0 fail |
| 4. contract/README.md:32 shows five-key object; `git grep -n '"error":"job_waiting"'` lists no body without `code` outside the no-snapshot test | HOLDS. README.md:32 has `{"error":"job_waiting","code":"job_waiting","job":...}`. git grep lists README.md:32 (with code), fu2_approve_job_waiting_test.go:67 (with code), and docs/lanes/C-fu2-approve-job-waiting-plugin.md:28 (a past lane report without code; see notes). The no-snapshot test has no job_waiting body. |
| 5a. `go test -count=1 ./internal/cli/` | HOLDS, exit 0, `ok ... 36.254s` |
| 5b. `make lint` | HOLDS, exit 0 (go vet ./... and gofmt -l, no output) |
| 5c. `git diff 2d92aa42 --stat` names only Touches files | HOLDS apart from this report; see the stat in the final report |
| Importers of internal/cli (go list) | cmd/grid (no test files), lib. `go test -count=1 ./cmd/grid ./lib` exit 0: `ok .../lib` |
| Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| No attribution lines in the commit | HOLDS. One-sentence message, no trailer. |
| No guard/gate/cap added; nothing removed; existing test edited only as the brief names | HOLDS. Only the `fu2WaitingBody` constant changed (fu2_approve_job_waiting_test.go:67). Exit code 3, stderr and the no-snapshot path are unchanged. |
| RealPlugin tests | Not part of this brief. No row names one. |

## What changed

- internal/cli/task_write.go:197-203: `Code string \`json:"code"\`` as the second field of the job_waiting struct, value "job_waiting". No other key.
- internal/cli/fu2_approve_job_waiting_test.go:67: `fu2WaitingBody` gains `"code":"job_waiting"` after `error`.
- contract/README.md:32: the documented object gains the `code` key.
- internal/cli/gl_job_waiting_code_test.go (new): TestGlJobWaitingCode.

## Notes

- docs/lanes/C-fu2-approve-job-waiting-plugin.md:28 is a past lane report that still quotes the old four-key body. It is outside Touches, so I left it.
- TestFu2ApproveJobWaitingJSON compares with an unordered map, so key order is proved only by TestGlJobWaitingCode.

## NOT VERIFIED on a live screen

Nothing here is UI. The app parser of backlog-11nov-ge/C-ge-approve-job-waiting-app.md was not run; the object is the one it accepts (five keys, `code` extra).
