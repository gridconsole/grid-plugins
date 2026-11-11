# C-win-verify-names-test

Base: plugin main 55c46cbd. Code commit: see the head sha in the hand-off report.

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1 | The seven cmd.exe forms are accepted | HOLDS. `go test ./internal/jobgraph/ -run TestVerifyWindowsForms` exit 0 on the lane (`internal/jobgraph/verify_windows_forms_lane_test.go`). Fails on base 55c46cbd: `verify_windows_forms_lane_test.go:48: got verify.command must name a test, valid true` for `go.exe test ./...`, `GO.EXE test ./...`, `npm.cmd test`, `C:\Go\bin\go.exe test ./...`, `"C:\Program Files\Go\bin\go.exe" test ./...`, `cargo.exe test`, `node.exe --test tests\a.test.mjs` (plus two extra rows I added: `C:\Go\bin\GO.EXE test ./...` and `go.exe build ./... && C:\Go\bin\go.exe test ./...`). |
| 2 | Still refused: `go.exe build ./...`, `echo.exe test`, `C:\tools\make.exe check`, `go.exe vet ./...`, `$(go test)`, `node.exe -e "test"` | HOLDS. Same test, refused rows, pass on base and on the lane (they were not in base's failure list). I also pinned `C:\Go\bin\go.exe test $(x)` and `... test (x)` as refused. |
| 3 | TestVerifyCommands unchanged and passes; new file repeats none of its rows | HOLDS. `go test ./internal/jobgraph/` exit 0. `floor_test.go` is not in the diff. None of the new file's rows is in floor_test.go:132-142. |
| 4 | POSIX escapes keep today's answer | HOLDS. `go\ test`, `echo hello\; go test ./...`, `echo C:\\Go\\bin\\go.exe test` and `GO test ./...` are rows of the new test; they pass on base (refused) and on the lane (refused). The second reading is skipped for the first three because a backslash stands before a space, `;` or another backslash. |
| 5a | `go test ./internal/jobgraph/ ./internal/job/...` | HOLDS. exit 0: jobgraph, job, job/frontmatter, job/jobcmd, job/lint ok (jobtest has no test files). |
| 5b | `GOOS=windows go vet ./internal/jobgraph/` | HOLDS. exit 0. |
| 5c | `make lint` | HOLDS. exit 0 (runs `go vet ./...`). |
| R1 | Importers of jobgraph tested | HOLDS. `go list` importers: internal/cli, internal/orchestrator. `go test ./internal/cli/ ./internal/orchestrator/` exit 0 (cli 76s, orchestrator 65s). |
| R2 | Windows and macOS builds | HOLDS. `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0. |
| R3 | Only allowed files; no guard, gate or cap; existing tests untouched | HOLDS. Changed: `internal/jobgraph/floor.go` (namesTest, commandWords, three small helpers) and the new test file. `ValidateVerify`, `testWord`, `internal/job/verify*.go` and `floor_test.go` are untouched. |
| R4 | Commits | HOLDS. One commit for code and test, one for this report; one-sentence messages, no attribution lines. |

## What changed

- `internal/jobgraph/floor.go`
  - `commandWords(command, escapes)`: a backslash escapes only when `escapes` is true.
  - `namesTest` reads the line as POSIX sh first. A second reading, with backslashes as plain characters, is made only when the first reading parsed and named no test, and `backslashesArePlain` finds a backslash and none stands before space, tab, newline, `;`, `&`, `|`, `'`, `"`, `#`, `$`, a backtick, `(`, `)`, `{`, `}`, another backslash or the end of the line. It returns true if either reading names a test.
  - `programName`: base after the last `/` or `\`; drops one trailing `.exe`, `.cmd`, `.bat` or `.com` (any case) and lower-cases the name only when one was dropped. Used for the switch and the testWord match.
  - `commandsNameTest` is the old loop body, moved unchanged apart from calling `programName`.
- New `internal/jobgraph/verify_windows_forms_lane_test.go` (untagged, so it runs on every host).

## Notes

- A name that is exactly `test.exe` (or `test.cmd`) used to match testWord as `test.exe`. It is now read as `test`, the shell builtin, and skipped, like `./test` is today. This follows the brief's order (drop the extension before the switch); nothing in the repo tests it.
- A line made only of backslash paths with a POSIX-escape-looking character, such as `"C:\dir\" go.exe test`, keeps the first reading's answer by design.

## NOT VERIFIED on a live screen

Nothing here is UI. Verify on a real Windows host was not run; the check is a pure string function and is tested on this Linux host and cross-built for Windows and macOS.
