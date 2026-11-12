# C-wb-open-launch-url-plugin: `grid open` carries the app's one-time launch code

Verdict: NOT DONE. One part of row 2 (the "no launch code was found" stderr line) conflicts with an existing test that row 4 runs. Everything else holds.

Base: plugin `e3031970` (lane-C-wa-open-start-args, stacked on origin/main `2b47f621`). App: lane-C-wb-launch-code-cookie `e984027e`. The checks below ran on `5a5c3125`; the later commits change only this file.

## Proof table

| Row | State | Proof |
|---|---|---|
| Base check | HOLDS | Plugin `e3031970`: internal/cli/open.go:152 was `target := base + "/" + route`, built from the base and the route alone. App `e984027e`: cmd/grid-server/launch.go:23 has `const wbLaunchFile = "launch-code"`, and :71 has `writeFileAtomicMode(filepath.Join(l.home, wbLaunchFile), l.code+"\n", 0o600)`. The URL is `/api/launch?code=` (the `wbLaunchFile` comment; launch.go:19 is the brief's cite), read in `wbLaunchServe` with `r.URL.Query().Get("code")` (:190 is the brief's cite). |
| 1 TestWbOpenJSONCarriesTheCode | HOLDS | `go test -count=1 -v -run 'TestOpen\|TestWbOpen\|TestWaOpen' ./internal/cli/` exit 0, `--- PASS: TestWbOpenJSONCarriesTheCode`. The `--json` url and the address handed to `env.OpenURL` are both `http://127.0.0.1:8484/api/launch?code=<code>#/t/<project>/<ulid>/overview`. Red on base: `wb_open_launch_url_test.go:73: --json url "http://127.0.0.1:8484/#/t/personal/01ARZ3NDEKTSV4RRFFQ69G5FAV/overview", want "http://127.0.0.1:8484/api/launch?code=q7-Lm_2xZ9pR4tV8wK0jH3nB6cF1yD5sG-aE_uJ0iOk#/t/personal/01ARZ3NDEKTSV4RRFFQ69G5FAV/overview"` |
| 2 TestWbOpenNoCodeFileOpensPlainURL, URL part | HOLDS | Same run, `--- PASS: TestWbOpenNoCodeFileOpensPlainURL`. With no code file, the `--json` url and the opened address equal `base + "/" + route` byte for byte, exit 0. With `GRID_APP_URL` set and a code file present, both equal `https://grid.example.test/` + route and carry no code (open.go:152, :170). The test passes on base too: it pins today's behaviour, and its only part that could be red on base is the stderr line below. |
| 2 TestWbOpenNoCodeFileOpensPlainURL, stderr line | NOT DONE | Conflicts with an existing test. `TestOpenStartsNothingWhenTheAppRuns` (internal/cli/open_start_test.go:205-206) requires empty stderr when the app runs with app-url and no launch-code file (`f.running()` writes only app-url, open_start_test.go:100). That is the brief's "no code file" path. Production always sets `AppStart` (internal/cli/cli.go:118), so gating the line on `AppStart` would make it dead code. Mismatch proof: with the line added after open.go:175, the row 4 run fails with `--- FAIL: TestOpenStartsNothingWhenTheAppRuns` / `open_start_test.go:206: stderr "grid open: no launch code was found; the page opens without one.\n", want nothing`. The line was then removed with `git checkout -- internal/cli/open.go`, and `git status --short` was empty. Editing that test is outside Touches and forbidden by the runbook. Owner call: allow the line and change that test, or drop the line from the brief. The line is one `Fprintln` in an `else` of open.go:171. |
| 3 TestWbOpenCodeNotOnStderr | HOLDS | Same run, `--- PASS: TestWbOpenCodeNotOnStderr`. Four runs check that stderr never holds the code while the url does: row 1's `--json`, row 1's browser run, a no-browser run (`OpenURL` errors, so stdout holds the code url and stderr says only "no browser was opened"), and row 2's `GRID_APP_URL` + code-file runs. Red on base: `wb_open_launch_url_test.go:107: --json url "http://127.0.0.1:8484/#/t/personal/01ARZ3NDEKTSV4RRFFQ69G5FAV/overview", want "http://127.0.0.1:8484/api/launch?code=q7-Lm_2xZ9pR4tV8wK0jH3nB6cF1yD5sG-aE_uJ0iOk#/t/personal/01ARZ3NDEKTSV4RRFFQ69G5FAV/overview"` |
| 4 row-4 run | HOLDS | With `-v`, piped into grep: 20 `--- PASS:` lines, including the three TestWbOpen tests, no `--- FAIL`, and an `ok` line. Without `-v` and without a pipe, `go test -count=1 -run 'TestOpen\|TestWbOpen\|TestWaOpen' ./internal/cli/` exit 0 (`ok github.com/gridconsole/grid-plugins/internal/cli 1.494s`). |
| 5 lint and cross | HOLDS | `make lint` exit 0 (`go vet ./...`). `make cross` exit 0 (darwin/arm64 and windows/amd64: vet and test compile). |
| Changed package and importers | HOLDS | Importers of internal/cli, from `go list -f '{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E 'grid-plugins/internal/cli( \|$)'`: cmd/grid, contract/formats, internal/account, internal/braincmd, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test -count=1 ./internal/cli/` plus those 11 packages: exit 0, every package `ok` (cmd/grid has no test files). |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0. `go test -count=1 ./internal/job/lint/` exit 0. |
| New tests red on base | HOLDS for rows 1 and 3 | The red lines are pasted above. Row 2's test is green on base, as explained in its URL row. |
| No existing test edited or deleted | HOLDS | `git diff --stat e3031970..HEAD` shows only internal/cli/open.go, internal/cli/wb_open_launch_url_test.go and this file. |
| Touches only | HOLDS | open.go, the one new test file, and this proof table, which the runbook requires. |
| Names | HOLDS | New package-level names are `wbOpenLaunchFile` and `wbOpenLaunchCode` (open.go:43, :46), plus `wbOpenCode`, `wbOpenBase`, `wbOpenHome`, `wbOpenJSONURL`, `wbOpenBrowse`, `wbOpenNoCodeOnStderr` and the `TestWbOpen*` tests. Before the change, `grep -rn "wbOpen\|LaunchCode\|launchCode\|launch-code" --include=*.go` found nothing. |
| No guard, gate, cap or hold on agents | HOLDS | The change only adds a query parameter to the address `grid open` opens. A missing code never fails the command. |
| gofmt | HOLDS | `gofmt -l internal/cli/open.go internal/cli/wb_open_launch_url_test.go` printed nothing. |
| RealPlugin tests | n/a | This lane adds and needs no *RealPlugin* test. In the app at `main`, `git grep -n '"open"' main -- '*realplugin*'` finds only JSON state values, so no app rig runs `grid open`. |
| Commits | HOLDS | Commits have one-sentence messages and no attribution lines. This file holds no personal paths, hosts or emails. |

## What changed

- `internal/cli/open.go`:
  - `wbOpenLaunchCode` reads the first line of `launch-code` in the Grid home, after `ensure` has run. It returns "" when the file is missing or empty.
  - When the app is local (`GRID_APP_URL` is unset) and a code is found, the target is `<base>/api/launch?code=<QueryEscape(code)>#/t/<project>/<id>/overview`. Otherwise it is the plain target, as before.
  - The one target goes to the browser, to the `--json` url field, and to the stdout lines (`opened <url>`, or the address printed when no browser opened).
  - With `GRID_APP_URL` set, the file is not read at all, so a remote app never receives this machine's code.
- `internal/cli/wb_open_launch_url_test.go`: the three acceptance tests and their `wbOpen` helpers.

## Notes for the backlog

- Stdout carries the code in the `opened <url>` line and in the no-browser line. The brief restricts only stderr and other files, and the no-browser fallback needs the full address. The code is single-use and the app rotates it after each trade (app launch.go:107).
- `--json` also opens the browser, as before, so the code in its url has usually been traded already.
- A stale code, or two `grid open` runs racing for one code, leads to the app's 401 page. Running `grid open` again reads the rotated code.
- The app trades the code only on Host `127.0.0.1` (app launch.go `wbLaunchLoopback`). `grid open` starts the server with `--addr 127.0.0.1:0` (open_start.go `appStartArgs`). That the server then writes app-url with `127.0.0.1` was not checked here.

## NOT VERIFIED on a live screen

- That a real browser follows the 303 to `/`, keeps the `#/t/...` route, and that the page then calls `/api` with the cookie. This needs the running app.

NOT DONE
