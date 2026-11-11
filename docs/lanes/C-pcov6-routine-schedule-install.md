# C-pcov6-routine-schedule-install

Test-only lane: two new files in internal/routine, no behaviour change.

## Proof table

| Row | Result |
|---|---|
| 1 Coverage before | HOLDS. `go test -count=1 -coverprofile=/srv/gridwork/tmp/C-pcov6-routine-schedule-install-before.out ./internal/routine/` exit 0, `coverage: 79.5% of statements`. `go tool cover -func=...before.out \| tail -1` exit 0: `total: (statements) 78.8%` (the -func figure differs from the test figure; noted, not a failure). No profile in the clone. |
| 2 ExecOSRunner.Run | HOLDS. TestSIExecOSRunner (schedule_install_test.go): test binary with `-test.run=^$` returns nil; `si-no-such-program` error has prefix `si-no-such-program: exec: `. Profile: schedule.go:35.2,38.16 / 39.3,40.1 / 41.2,41.12 all count 1. |
| 3 plan refusals | HOLDS. TestSIPlanRefusals, four subtests, exact texts. Profile schedule.go:126.3, 129.3, 132.3, 136.4 count 1. |
| 4 outside home | HOLDS. TestSICheckFilesOutsideHome: exact `timer file is outside the home folder`, no runner call. schedule.go:183.4 count 1. |
| 5 not regular | HOLDS. TestSICheckFilesNotRegular: folder at timer path, exact `timer file is not a small regular file`. schedule.go:199.4 count 1. |
| 6 Lstat failure | HOLDS. TestSICheckFilesLstatFailure in `schedule_install_unix_test.go` (`//go:build unix`): prefix `lstat `, no runner call. schedule.go:193.4 count 1. |
| 7 readState | HOLDS. TestSIReadStateRefusals: `{` gives *json.SyntaxError (errors.As); foreign record gives exact `timer ownership record does not match`. schedule.go:172.3, 175.3 count 1; install return 269.3 count 1. |
| 8 log folder | HOLDS. TestSILogFolderFailure: prefix `mkdir `, state file not written (os.Stat is ErrNotExist), no runner call. schedule.go:273.4 count 1. |
| 9 another machine | HOLDS. TestSIAnotherMachine: `install` gives exact `routine belongs to another machine`; `Schedule` returns nil, no runner call. schedule.go:258.3 count 1. |
| 10 install refusals | HOLDS. TestSIInstallRefusals: `bad name` exact; `missing` satisfies errors.Is(os.ErrNotExist); Home `rel` exact. schedule.go:250.3, 255.3, 262.3 count 1. |
| 11 List failure | HOLDS. TestSIScheduleListFailure: exact `project level needs a valid project name`. schedule.go:234.3 count 1. |
| 12 changed file | HOLDS. TestSIChangedTimerFile: reinstall calls exactly [remove install], file restored to `marker + "\n"`; with fail set: exact `OS failure`, calls [remove], file still the edited text. schedule.go:285.5, 296.3, 297.4 count 1. |
| 13 After and gates | HOLDS. After: `go test -count=1 -coverprofile=...after.out ./internal/routine/` exit 0, `coverage: 83.1%`; `go tool cover -func \| tail -1` 82.5%. Before 79.5% (78.8% by -func). One `grep -E` on the after profile shows every named block at count 1. `go test -count=1 ./internal/routine/` exit 0. `make lint` exit 0 (runs `go vet ./...`). `GOOS=windows go vet ./internal/routine/` exit 0. `GOOS=darwin go vet ./internal/routine/` exit 0. |
| Importers | HOLDS. Only internal/cli imports internal/routine (cmd/grid and lib import cli). `go test -count=1 ./internal/routine/ ./internal/cli/ ./cmd/grid/ ./lib/` exit 0 (cmd/grid has no tests). |
| New tests fail on base | NOT APPLICABLE. They pin behaviour the base code already has, and they only add coverage; on base they pass. No bug found, so no assertion was kept failing. |
| No existing test edited or deleted | HOLDS. Diff is two new files plus this report. |
| No guard/gate/cap | HOLDS. No production code touched. |
| Unix-only code | HOLDS. No chmod, Mkfifo, syscall or symlink; the Lstat row is behind `//go:build unix`; OS error text is asserted by prefix up to the path. |
| Prefix | HOLDS. Helpers siBackend, siSetup, siWant, siCalls, siState; tests TestSI*. |
| approval.json / jobtest | HOLDS. Not used. |
| gofmt | HOLDS. `gofmt -l internal/routine/` prints nothing. |

## What changed

- internal/routine/schedule_install_test.go (new)
- internal/routine/schedule_install_unix_test.go (new, unix only)
- docs/lanes/C-pcov6-routine-schedule-install.md (this report)

## NOT VERIFIED on a live screen

Nothing here is UI. Not run on a real Windows or macOS host: only `go vet` under GOOS=windows and GOOS=darwin. The Lstat-through-a-file row runs only on unix hosts.

DONE
