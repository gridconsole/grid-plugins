# C-ko-orch-tree-stream-waits: tree times move a test clock and the stream cancel test waits on its spawn line

Lane: C-ko-orch-tree-stream-waits, repo plugin. Base sha (origin/main and local main): 21f02736. The brief names main dc301fce; `git merge-base --is-ancestor dc301fce HEAD` prints `ancestor`, and the sleeps sit at the lines the brief gives. Both sleeps are on the base: `git grep -n 'time\.Sleep' -- internal/orchestrator/tree_times_test.go internal/orchestrator/stream_test.go` on 21f02736 printed `internal/orchestrator/stream_test.go:387:` and `internal/orchestrator/tree_times_test.go:131:`.

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1 | `git grep -n 'time\.Sleep'` in the two files prints nothing, exit 1 | HOLDS. Printed nothing on the lane head (base printed :131 and :387). The shell reports no match as exit 1, so the Bash tool showed an empty result. |
| 2 | `go test -count=1 -v -run 'TestTreeReadyAtIsTheAfterClose\|TestStreamRunKeepsLeaseOnCancel' ./internal/orchestrator/`: both `--- PASS:`, no `--- SKIP` | HOLDS, exit 0. `--- PASS: TestStreamRunKeepsLeaseOnCancel (0.13s)`, `--- PASS: TestTreeReadyAtIsTheAfterClose (0.98s)`, `ok  github.com/gridconsole/grid-plugins/internal/orchestrator 1.259s`. No SKIP line. |
| 3 | `go test -count=3 ./internal/orchestrator/`: exit 0 | HOLDS, with `-timeout 30m` added. `go test -count=3 -timeout 30m ./internal/orchestrator/`: `ok  github.com/gridconsole/grid-plugins/internal/orchestrator 619.852s`, exit 0. The command exactly as the brief writes it (default 10 min timeout) is NOT proven: the first attempt hit the default (see the note below), and 619.852 s is above 600 s, so on this VM the bare command can hit the default whatever this lane does. A `-race` run of the two tests (47 s) overlapped the start of the 619.852 s run. |
| 4 | `git diff main --stat` names only the two Touches files | HOLDS. `internal/orchestrator/stream_test.go | 37 +++++++---`, `internal/orchestrator/tree_times_test.go | 5 ++-`, `2 files changed, 35 insertions(+), 7 deletions(-)` (before this report file was added). `orchestrator_test.go` is not edited. |
| 5 | Walk tests after `git merge main` (`Already up to date.`) | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `ok .../internal/cli 0.310s`, exit 0. `go test -count=1 ./internal/job/lint/`: `ok .../internal/job/lint 0.324s`, exit 0. |
| 6a | `make lint` exit 0 | HOLDS. `make lint` is `go vet ./...`; it printed only the command line and no finding; make returned without an error. |
| 6b | `make cross` exit 0 | HOLDS. darwin/arm64 and windows/amd64 vet and test compile; the background task reported `completed (exit code 0)`. |
| R1 | Small commits, one-sentence messages, no attribution lines | HOLDS. One code commit with a one-sentence message and one report commit; no Co-Authored-By or session line (the owner's rule wins over the reminder). |
| R2 | New tests fail on base and pass on the lane | HOLDS, with nothing to fail: no new test was added. Both tests already existed and passed on base with their sleeps; the change removes the wait and keeps every assertion. Proof that they pass without the sleeps: row 2, and `go test -race -count=5 -run 'TestStreamRunKeepsLeaseOnCancel\|TestTreeReadyAtIsTheAfterClose' ./internal/orchestrator/`: `ok ... 47.344s`, exit 0). |
| R3 | No existing test edited to pass, none deleted | HOLDS. The assertions of both tests are unchanged: `ready_at == closed`, `ready_at != post2`, the `pickup_seconds` check, the Run return `context.Canceled`, and the lease still held after cancel. No test deleted. |
| R4 | No guard, gate, cap, hold or blocking hook on agents | HOLDS. Test-only change; no production file. |
| R5 | Tests of the changed package and every package that imports it | HOLDS. `go list` shows the importers of `internal/orchestrator`: `internal/cli`, `internal/mcp`, `internal/migrate`. `go test -count=1 ./internal/mcp/ ./internal/migrate/ ./internal/cli/`: `ok mcp 0.956s`, `ok migrate 34.971s`, `ok cli 166.188s`, exit 0. The changed package itself: row 3. |
| R6 | Windows and macOS builds | HOLDS. Row 6b. |
| R7 | Repo-wide walk tests of the plugin | HOLDS. Row 5. The app-only walk tests (webui, plugincli, grid-server, slices) belong to the app repo and do not apply to this repo. |
| R8 | No sleep, no poll loop, no elapsed-time assertion, no timeout as proof | HOLDS. The tree test moves `f.now`. The stream test waits on a channel in a `select`; its only deadline is the 10 s failure bound of that select, whose other case is the spawn event. |

## What changed

`internal/orchestrator/tree_times_test.go`, TestTreeReadyAtIsTheAfterClose (lines 123-132):
- `f.s.Feed.Now = func() time.Time { return f.now }` right after `setup(t)`, before the first post, so every record time of this test comes from the fixture clock (`internal/feed/store.go:508-511`). Other tests keep the wall clock; `orchestrator_test.go` is not touched.
- `time.Sleep(20 * time.Millisecond)` became `f.now = f.now.Add(time.Second)`: the close of J-1 gets a later time than the post of J-2 by moving the clock. The J-2 post is at `f.now`, the close at `f.now+1s`, so `ready_at != post2` holds by construction. The pickup check still tells "from the close" apart from "from the post": from the close it is 0 s, from the post it would be 1 s.

`internal/orchestrator/stream_test.go`:
- New helper `koOtSpawnWatch` (writer): it forwards every write to the wrapped writer and, without blocking, sends on a buffered channel (capacity 1, `select` with `default`) when the written bytes hold `"spawn"`. It keeps no copy of the output.
- TestStreamRunKeepsLeaseOnCancel: after `h.stream`, `s.Out` is wrapped in it (`h.out` is still the buffer underneath). The poll loop with `time.Sleep(5 * time.Millisecond)` is gone. The test now selects on the channel and a `time.After(10 * time.Second)` failure bound, then checks `h.tenure(0).Holder != nil` once, then cancels. `Run` prints the spawn line after `AcquireTask` and `Tick` (`internal/orchestrator/stream.go:208-264`), so the lease is held when the line appears. `h.out` is not read until after Run returned: the test never reads it, and `go test -race` reports no race.
- `io` added to the imports; `defer cancel()` so a failed wait does not leave Run running.

Row 3 note: one pass of `./internal/orchestrator/` takes about 150 s on this VM (`ok ... 156.498s`), so `-count=3` passes the default 10 min `go test` timeout when the machine is busy. A first `-count=3` run, started while other test runs shared the CPU, hit `panic: test timed out after 10m0s` in `TestProposedJobsAndCheckpoints/scope_change` (a goroutine in `contract.Validate`, running, not blocked). That test is not one of the two changed, and the panic is the default timeout, not a failure of an assertion. The run for row 3 used `-timeout 30m` and took 619.852 s. I did not time the package on the base, so I cannot say the lane changed its duration; the two changed tests take well under a second each (0.13 s and 0.98 s) and the removed sleeps were 20 ms and 5 ms steps. The orchestrator or the brief's author may want `-timeout` in the row 3 command.

The brief lists the skill `go-conventions`; it is not in this session's skill list and the builder runbook names none, so I followed the existing style of the two files.

## Open items

- Row 3 bare command at the 10 min default: see the row 3 note. Needs `-timeout` on a slow or busy VM.

## NOT VERIFIED on a live screen

Nothing here needs a live screen. Tests only.

DONE
