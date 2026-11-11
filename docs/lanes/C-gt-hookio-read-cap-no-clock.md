# C-gt-hookio-read-cap-no-clock

TestReadCap proves the read cap with a channel instead of a 25 ms race against the scheduler.

Code head: 74443a4e (the one code commit on main). A second commit adds only this report.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1a. `TestReadCap` passes | HOLDS | `go test -count=1 -run '^TestReadCap$' -v ./internal/hookio/` exit 0: `--- PASS: TestReadCap (0.00s)` / `PASS` / `ok  github.com/gridconsole/grid-plugins/internal/hookio 0.009s` |
| 1b. 50 runs | HOLDS | `go test -count=50 -run '^TestReadCap$' ./internal/hookio/` exit 0: `ok  github.com/gridconsole/grid-plugins/internal/hookio 0.152s`. Also `go test -race -count=20 ...` exit 0. |
| 2. no `time.Since`/`time.Sleep` in the test | HOLDS | `git grep -n -E 'time\.(Since|Sleep)' -- internal/hookio/hookio_test.go` printed nothing (exit 1, no match). The only clock left in TestReadCap is two 30 s `time.After` failure bounds (hookio_test.go:59, :68); they only fail the test and name what never happened. |
| 3a. break 1: `case <-after:` returns `nil, false` | HOLDS | `go test -count=1 -run '^TestReadCap$' ./internal/hookio/` exit 1: `--- FAIL: TestReadCap (0.00s)` / `hookio_test.go:66: <nil> false` / `FAIL`. Restored with `git checkout -- internal/hookio/hookio.go`. |
| 3b. break 2: `read` passes a nil channel | HOLDS | `go test -timeout 60s -count=1 -run '^TestReadCap$' ./internal/hookio/` ends in `panic: test timed out after 1m0s` / `running tests:` / `TestReadCap (1m0s)`. Restored with `git checkout -- internal/hookio/hookio.go`. |
| 3c. after both restores | HOLDS | `git status --short` printed nothing. `git diff main --stat`: `internal/hookio/hookio.go 13 +++++++--`, `internal/hookio/hookio_test.go 66 +++++++++++++++++++++++++++++++++++-------`, `2 files changed, 66 insertions(+), 13 deletions(-)`. Neither break was committed. |
| 4. whole hookio package | HOLDS | `go test -count=1 -v ./internal/hookio/` exit 0: all 12 tests PASS, including TestReadByteLimit (5 subtests), TestReadStopsAtLimit, TestReadNothingToRead, TestReadCutInputKeepsCwdAndAgentType. |
| 5a. diff names only the two Touches files | HOLDS | `git diff main --stat` at code head 74443a4e names `internal/hookio/hookio.go` and `internal/hookio/hookio_test.go` only (output in 3c). The report commit after it adds `docs/lanes/C-gt-hookio-read-cap-no-clock.md`, as every lane here does. |
| 5b. gofmt | HOLDS | `gofmt -l internal/hookio` printed nothing, exit 0. |
| 5c. vet, Linux and Windows | HOLDS | `go vet ./internal/hookio/` exit 0. `GOOS=windows go vet ./internal/hookio/` exit 0. |
| Test fails on base | HOLDS | With `internal/hookio/hookio.go` checked out from main: `go test -count=1 -run '^TestReadCap$' ./internal/hookio/` exit 1: `internal/hookio/hookio_test.go:51:16: undefined: readUntil` / `FAIL ... [build failed]`. The old test failed on base only by chance under load; the new one cannot compile without the seam. Restored with `git checkout HEAD -- internal/hookio/hookio.go`. |
| Packages that import hookio | HOLDS | `go list -f '{{range .Deps}}{{if eq . "github.com/gridconsole/grid-plugins/internal/hookio"}}{{$.ImportPath}}{{"\n"}}{{end}}{{end}}' ./...` lists cmd/grid, internal/cli, internal/guard, internal/hook, internal/packaging, internal/packaging/cmd, lib. `go test -count=1 ./cmd/grid ./internal/cli ./internal/guard ./internal/hook ./internal/hookio ./internal/packaging ./internal/packaging/cmd ./lib` exit 0: ok for cli 43.4 s, guard, hook, hookio, packaging, packaging/cmd, lib; cmd/grid has no test files. |
| Lint | HOLDS | `make lint` exit 0 (go vet ./... and gofmt -l . print nothing). |
| Windows and macOS builds | HOLDS | `make cross` exit 0: `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile`, every package `ok`. |
| No test deleted, renamed or skipped; no existing test edited | HOLDS | Only the body of TestReadCap changed (same name, same failure words: `t.Fatal(got, large)`). The helper `readerFunc` already existed; no package-level type or helper was added. No `t.Skip`. |
| No guard, gate, cap or hold added on agents | HOLDS | The seam is a function split. The 3 s cap, the byte limit and what `Read` returns for any caller are unchanged. |
| Tests named *RealPlugin* | Not applicable | The brief names none, and the change touches no plugin file; nothing to skip or run. |
| No personal paths, hosts or emails in the diff | HOLDS | The diff holds only Go code and this report. |
| Commit rules | HOLDS | One-sentence commit messages; no attribution lines. |

## What changed

- `internal/hookio/hookio.go`: `read(r, cap, limit)` now makes the timer, stops it on return, and calls the new `readUntil(r, after <-chan time.Time, limit)`. `readUntil` holds the old body unchanged except that `case <-timer.C:` is `case <-after:`. The `r == nil` check moved into `readUntil`; `read(nil, ...)` still returns `nil, false`. The one difference: the timer is now created before the reader goroutine starts, not just after. It makes no observable change, because the select has always run after the goroutine started.
- `internal/hookio/hookio_test.go` (TestReadCap only): drives `readUntil` with its own `after` channel and a reader that returns the JSON on its first Read; on its second Read it closes `second` and blocks until `t.Cleanup` releases it with `io.EOF`. The test waits for `second`, closes `after`, then asserts `got.ToolName == "Read"` and `large == false`. The elapsed-time check is gone. One call through `read` itself stays: a reader whose Read blocks until `t.Cleanup`, with a 1 ms cap, must return `(nil, false)`.
- Ordering that makes it race-free: the first chunk is written to the buffer under the mutex before the loop's second Read is called, so `<-second` happens after the buffer holds the JSON. `go test -race -count=20` passes.

## NOT VERIFIED on a live screen

Nothing here is a UI change. I did not run the test under heavy load (load 50 to 130 on a landing box); the argument is structural: no assertion in TestReadCap depends on how long a goroutine waits.
