# C-sg-open-app-log-rotate

Base 31c7f182. Code commits fe5b8dda, 60b081f1 touch only `internal/cli/open_start.go` and `internal/cli/sg_app_log_rotate_test.go`; this file is the runbook's proof table, in its own commit.

## Base check
- `git grep -n 'rotat' main -- internal/cli/open_start.go`: empty, exit 1.
- OpenFile at base: `open_start.go:227` `logf, err := os.OpenFile(filepath.Join(logDir, name+".log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)`.
- Red on base (new tests, production unchanged):
  - `--- FAIL: TestSgRotatesAtStart` `no grid-server.log.1: ... no such file or directory`
  - `--- FAIL: TestSgKeepsSeven` `.1 starts "m1\n", want marker m0` (and .2 to .6), `old .6 marker still there`
  - The other three TestSg tests assert behaviour the base already has (no rotate when small or running, append on a failed shift), so they pass on base by design.

## Proof table
| Row | Result |
|---|---|
| 1 TestSgRotatesAtStart, TestSgKeepsSeven | HOLDS: `go test -count=1 -v ./internal/cli -run 'TestSg\|TestOpen'` exit 0, `--- PASS` for both |
| 2 TestSgSmallLogAppends, TestSgRunningProgramNotRotated | HOLDS: same run, `--- PASS` |
| 3 TestSgFailedShiftStillStarts | HOLDS: same run, `--- PASS` (Unix only) |
| 4 TestSg x5 and TestOpenStartsWorkerAndServerWhenNotRunning | HOLDS: same run exit 0, six PASS lines |
| 5 `make lint` | HOLDS: exit 0 (go vet) |
| 5 `make cross` | HOLDS: exit 0 |
| 5 `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS: ok, exit 0 |
| 5 `go test -count=1 ./internal/job/lint/` | HOLDS: ok, exit 0 |
| 5 `git diff main --stat` | HOLDS for code commits: only open_start.go and the new test; plus this doc |
| Packages importing the change | `go list` importers of internal/cli: cmd/grid, lib. `go test -count=1 ./internal/cli ./cmd/grid ./lib`: ok, ok, no test files; exit 0 |
| Existing tests edited or deleted | none |
| New guard/gate/cap on agents | none; a failed shift never stops a start and prints nothing |
| Repo-wide walk tests (plugin) | HOLDS: the two walk commands above, both exit 0 |
| RealPlugin tests | not named by a row; none run |

## What changed
`open_start.go`: constants `appLogLimit = 10 << 20`, `appLogFiles = 7`; `shiftAppLog(path)` called in `startDetached` between MkdirAll and OpenFile. It shifts when the file is `>=` the limit: `os.Remove` of `.6` (a non-empty folder fails here, by design), then renames `.5`→`.6` … `.log`→`.log.1`, skipping IsNotExist; the first other error stops it with nothing undone.

## NOT VERIFIED on a live screen
Nothing needs a screen. Not run against the real grid-server (fake scripts only).
