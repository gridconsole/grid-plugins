# C-pcov2-home-format-records: proof table

Tests only: four new test files, no non-test file changed. Platform: linux, same go toolchain as the runs below.

## Row 1: coverage before and after (`go test -count=1 -coverprofile=<file> ./internal/<pkg>/ && go tool cover -func=<file> | tail -1`)

Profiles were written to /srv/gridwork/tmp/C-pcov2-<pkg>-{before,after}.out (mkdir was refused in the sandbox, so no mktemp folder).

| pkg | before (untouched clone, linux) | after |
|---|---|---|
| gridhome | `total: (statements) 86.7%` | `total: (statements) 96.4%` |
| localdata | `total: (statements) 76.3%` | `total: (statements) 86.6%` |
| project | `total: (statements) 92.1%` | `total: (statements) 97.8%` |
| people | `total: (statements) 91.4%` | `total: (statements) 97.1%` |

HOLDS: each after is above its own before. FormatProblem line: `internal/gridhome/gridhome.go:151:	FormatProblem	100.0%`.

## Rows 2-6 (all in the new files, each test run in the row-7 command, exit 0)

| Row | Status | Where |
|---|---|---|
| 2 FormatProblem n+1 / n-1 / n | HOLDS | internal/gridhome/format_problem_test.go:11 TestFormatProblemTexts |
| 2 Real("rel/x") after t.Chdir | HOLDS | format_problem_test.go:23 TestRealMakesARelativePathAbsolute |
| 2 ReadMachine / Machine on a regular file path; TasksDirs with projects a file | HOLDS | format_problem_test.go:49 TestMachineBelowARegularFile, :59 TestTasksDirsWhenProjectsIsAFile (both t.Skip on Windows: "Windows answers such a path as not-exist.") |
| 3 Init("") and Safe("") text; Init on a regular file | HOLDS | internal/localdata/init_refusals_test.go:30 TestNoHomeIsRefused, :40 TestInitHomeIsARegularFile (file stays "x") |
| 3 VERSION symlink -> ErrUnsafePath, still a symlink | HOLDS | init_refusals_test.go:53 TestInitRefusesAVersionSymlink (t.Skip when os.Symlink fails) |
| 3 VERSION directory -> "VERSION is not a regular file", still a directory | HOLDS | init_refusals_test.go:75 TestInitRefusesAVersionDirectory |
| 3 VERSION "abc\n" and FormatVersion+1 -> texts; content unchanged | HOLDS | init_refusals_test.go:87 TestInitRefusesABadOrNewerVersion |
| 4 Machine("") errors; machine symlink -> ErrUnsafePath | HOLDS | init_refusals_test.go:30, :107 TestMachineRefusesASymlink |
| 4 machine "hello\n" -> "machine file does not contain a ULID", file still "hello\n" | HOLDS | init_refusals_test.go:125 TestMachineNeverReplacesAForeignFile |
| 4 LockExisting on a directory: error, nil unlock | HOLDS | init_refusals_test.go:141 TestLockExistingOnADirectory |
| 5 Load/Write with projects a regular file | HOLDS | internal/project/load_errors_test.go:10 (t.Skip on Windows, ENOTDIR reason) |
| 5 stray projects/notes.txt -> exactly [a] | HOLDS | load_errors_test.go:26 TestLoadSkipsAStrayFile |
| 5 projects/b/project.md a directory -> error | HOLDS | load_errors_test.go:43 TestLoadRefusesAProjectMdDirectory |
| 5 Format(Record{}) == "---\n---\n" | HOLDS | load_errors_test.go:53 TestFormatOfAnEmptyRecord (Format returns ([]byte, error); the test checks both) |
| 6 Validate table of 8 entries, exact texts; roles owner/member/guest nil | HOLDS | internal/people/validate_test.go:13 TestValidateRefusals (role "admin" is the control), :46 TestValidateAcceptsEachRole |

## Row 7 and rules

| Item | Status | Command and result |
|---|---|---|
| Four packages pass | HOLDS | `go test -count=1 ./internal/gridhome/ ./internal/localdata/ ./internal/project/ ./internal/people/` -> all `ok`, exit 0 |
| Importers pass | HOLDS | Direct importers of the four (from `go list -f` over Imports, TestImports, XTestImports): account agent brain braincmd cli doctor env gitx guard hook job/jobcmd land migrate ports projectmd projects proof routine settings signal task usage where worker workflow golden mcp contract/formats. `go test -count=1` on that list plus the four -> 32 lines, all `ok`, none FAIL |
| Windows build | HOLDS (compiled, needs Windows run) | `GOOS=windows go test -c -o /dev/null ./internal/<pkg>/` for each of the four -> exit 0 |
| macOS build | HOLDS (compiled, needs macOS run) | `GOOS=darwin go test -c -o /dev/null ./internal/<pkg>/` for each of the four -> exit 0 |
| Lint | HOLDS | `make lint` (go vet ./...) -> exit 0; `gofmt -l` on the four dirs -> empty |
| Tests only, no non-test file | HOLDS | `git diff --stat origin/main..HEAD` lists only the four `_test.go` files and this report |
| No existing test file edited | HOLDS | same diff |
| New tests fail on base | NOT APPLICABLE | The tests pin behaviour the code already has (coverage lane, no code change), so they pass on base too. No failing lines to paste. They were not shown to fail by mutating code, because the brief forbids a non-test change. |
| No guard, gate, cap or hold added | HOLDS | tests only |
| Names | HOLDS | New helpers (wantErr, readFile, aFile, validOwner) did not clash with the packages' existing ones; the packages compile and vet. |

## What changed

Four new test files, one per package: gridhome/format_problem_test.go, localdata/init_refusals_test.go, project/load_errors_test.go, people/validate_test.go. No bug exposed; no row dropped.

## NOT VERIFIED

- The Windows skips and the symlink skips ran only as compiles; the tests were not run on Windows or macOS (`GOOS=windows go test ./internal/<pkg>/` on a Windows host).
- No UI in this lane.

DONE
