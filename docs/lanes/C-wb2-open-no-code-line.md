# C-wb2-open-no-code-line

Stacked on lane-C-wb-open-launch-url-plugin (not landed; in this branch).

## Proof table

| Row | Result |
|---|---|
| 1. `grid open` prints `grid open: no launch code was found; the page opens without one.` on stderr when the launch-code file is missing | HOLDS: internal/cli/open.go:171-175 (the else of `if code := wbOpenLaunchCode(env); code != ""`, inside `if local`) |
| 2. TestOpenStartsNothingWhenTheAppRuns expects exactly that line; every other check kept (nothing started, URL opened) | HOLDS: internal/cli/open_start_test.go:205-207 (only the stderr check changed). Red on base: with open.go stashed, `go test -count=1 -run TestOpenStartsNothingWhenTheAppRuns ./internal/cli/` printed `open_start_test.go:206: stderr "", want "grid open: no launch code was found; the page opens without one.\n"` and `FAIL` |
| 3a. `go test -count=1 ./internal/cli/ -run TestOpen` | HOLDS, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/cli 1.357s` |
| 3b. wb row 1 TestWbOpenJSONCarriesTheCode | HOLDS: `--- PASS: TestWbOpenJSONCarriesTheCode` |
| 3c. wb row 2 TestWbOpenNoCodeFileOpensPlainURL | HOLDS: `--- PASS: TestWbOpenNoCodeFileOpensPlainURL` |
| 3d. wb row 3 TestWbOpenCodeNotOnStderr | HOLDS: `--- PASS: TestWbOpenCodeNotOnStderr` |
| 3e. wb row 4 `go test -count=1 -v -run 'TestOpen\|TestWbOpen\|TestWaOpen' ./internal/cli/` | HOLDS, exit 0: 20 `--- PASS:` lines, no `--- FAIL`; last lines `--- PASS: TestWbOpenCodeNotOnStderr (0.00s)`, `PASS`, `ok  github.com/gridconsole/grid-plugins/internal/cli 1.474s`. Touches = C-wb-open's Touches plus open_start_test.go |
| 3f. wb row 5 `make lint` | HOLDS, exit 0 (`go vet ./...`, no output) |
| 3g. wb row 5 `make cross` (darwin/arm64 and windows/amd64 vet and test compile) | HOLDS, exit 0, all packages `ok` |
| Importers of internal/cli (`go list` of imports): cmd/grid, lib | HOLDS: `go test -count=1 ./internal/cli/ ./lib/ ./cmd/grid/` exit 0: cli ok 77.145s, lib ok 33.693s, cmd/grid no test files |
| Walk tests | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok 0.112s; `go test -count=1 ./internal/job/lint/` ok 0.258s |
| Commit style | HOLDS: one commit, one-sentence message, no attribution lines; `git status --short` clean |
| No guard, gate, cap or hold on agents | HOLDS: none added |
| RealPlugin tests | Not applicable: no *RealPlugin* row in the brief |

## What changed

- internal/cli/open.go: one `Fprintln` to stderr in the else branch where no launch code was read (local app only; with GRID_APP_URL set nothing is printed, as before this lane).
- internal/cli/open_start_test.go: TestOpenStartsNothingWhenTheAppRuns now expects that line, and only that line, on stderr.

## NOT VERIFIED on a live screen

Nothing here needs a live screen. No real browser was opened.

Verdict: DONE
