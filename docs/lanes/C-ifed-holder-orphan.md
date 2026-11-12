# C-ifed-holder-orphan

Chosen fix: the holder opens the FIFO with O_RDONLY|O_NONBLOCK, clears non-blocking, then reads. With no writer left the read gives EOF at once; with the test's O_RDWR end open it blocks until that closes. This is the smallest change and needs no deadline or session kill. Only `internal/eval/if_process_drain_test.go` changed.

| Row | Result |
|---|---|
| `go test -count=3 -run 'TestIfEd' ./internal/eval/` | HOLDS, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/eval  0.231s` |
| no `TestIfEdHelperProcess` process left after that run | NOT VERIFIED: `pgrep`, `grep /proc` and `ls /proc` are refused by the allow list. The new test checks the holder's pid is gone, and its Cleanup SIGKILLs the pid. |
| `go test -count=1 ./internal/eval/` | HOLDS, exit 0: `ok ... internal/eval 10.069s` |
| `make lint` | HOLDS, exit 0 (go vet ./..., no output) |
| New test fails on base, passes on lane | HOLDS. With the old blocking open: `--- FAIL: TestIfEdHolderEndsWithoutWriter (20.05s)  if_process_drain_test.go:137: the holder (pid 4021002) was still alive 20s after its FIFO had no writer`. With the fix: PASS (0.05s). |
| Test meaning kept (runProcess waits while a descendant holds stdout; "context canceled") | HOLDS: TestIfEdContextEnds unchanged and passes. Parent now prints `started <pid>`; the test reads only the first line. |
| Importers of internal/eval: evalcmd, land | HOLDS, exit 0 for both |
| Windows build `GOOS=windows go build ./...` | HOLDS, exit 0 |
| macOS check `GOOS=darwin go vet ./internal/eval/` | HOLDS, exit 0 |
| Walk tests `TestHsIpWalksEveryCommand\|TestIlGuestSiteList`, `internal/job/lint` | HOLDS, exit 0 |
| No existing test edited or deleted | HOLDS (only the parent's `started` line gained a pid, in the helper) |
| No guard or gate added | HOLDS |

NOT VERIFIED on a live screen: none, there is no UI.
