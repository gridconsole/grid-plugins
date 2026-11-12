# C-we2-kind-usage-line

| Row | Result |
|---|---|
| Usage line has `[--kind setup]` after `[--workflow <name>]` | HOLDS: internal/cli/task.go:19, testdata/golden/task-help.golden:2 |
| `go test -count=1 -run 'TestTaskRenameDeclineHelp\|TestWeKind' ./internal/cli/ ./internal/task/...` | HOLDS, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/cli 0.083s`, `ok  .../internal/task 0.013s [no tests to run]` |
| `make lint` | HOLDS, exit 0: last line `go vet ./...` |
| New tests fail on base | n/a: no new test; the existing golden test is updated with the usage line |
| Plugin walk tests | HOLDS, exit 0: `ok .../internal/cli` (TestHsIpWalksEveryCommand, TestIlGuestSiteList), `ok .../internal/job/lint` |
| Windows and macOS builds | HOLDS, exit 0: `GOOS=windows go build ./...`, `GOOS=darwin go build ./...` |
| Packages importing internal/cli | NOT DONE: not listed with go list; cmd-level tests are left to the orchestrator's whole-module run |
| RealPlugin tests | n/a |

Changed: one usage string in internal/cli/task.go and the same line in the golden.
NOT VERIFIED on a live screen: nothing (CLI text only).
