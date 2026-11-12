# C-ak-open-home-without-task

Base: plugin main 7a4bc0c2 (the brief named 2221cb86; this clone is at 7a4bc0c2). On base, open.go:138 `resolveTaskOperand(env, operand)` with an empty operand returns task_operand.go:47 `give a task id or folder, or run inside a task folder`, so `grid open` outside a task did not open `#/`.

## Proof table

| Row | Result |
|---|---|
| 1 `TestAkOpenHomeOutsideProject` | HOLDS. Outside every project, `grid open` exits 0 and hands the browser `http://127.0.0.1:8484/api/launch?code=<wbOpenCode>#/`; without the code file `http://127.0.0.1:8484/#/` and stderr is exactly the no-launch-code line. Red on base: `open outside a project: exit 1, stderr "grid open: give a task id or folder, or run inside a task folder\n"` |
| 2 `TestAkOpenHomeInProjectFolder`, `TestAkOpenHomeJSON` | HOLDS, with one difference from the row text. In the project folder `grid open --json` prints one field `url` = `http://127.0.0.1:8484/#/`; with no address it prints `{"url":"#/"}`. The line "grid open: the app's address is not known. Start the app, then run grid open again." is on stderr for plain `grid open` only: with `--json` the existing code returns before it (open.go, the `if *asJSON` inside `if base == ""`), and the brief says to carry on unchanged. The test asserts stderr empty for `--json` and the line for plain. Red on base (project folder): `exit 1, stderr "grid open: give a task id or folder, or run inside a task folder\n"` |
| 3 `TestAkOpenTaskStillOpensTask` | HOLDS. From the task folder, no operand opens `<base>/#/t/<project>/<id>/overview`; `grid open T-99` exits non-zero and opens nothing. |
| 4 `go test -count=1 -v ./internal/cli/ -run 'TestAkOpen\|TestOpen\|TestWbOpen\|TestWaOpen\|TestGuestProjectParity'` | HOLDS, exit 0 (`ok ... 6.393s`); `--- PASS:` for TestAkOpenHomeOutsideProject, TestAkOpenHomeInProjectFolder, TestAkOpenHomeJSON, TestAkOpenTaskStillOpensTask; TestGuestProjectParity and TestOpen* pass unedited; no FAIL. |
| 5 walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok (0.597s); `go test -count=1 ./internal/job/lint/` ok (0.376s). |
| 6 `make lint`, `make cross` | HOLDS, both exit 0 (go vet, gofmt clean; darwin/arm64 and windows/amd64 vet and test compile). |
| Packages that import internal/cli | `go list` gives cmd/grid, lib. `go test -count=1 ./internal/cli/ ./cmd/grid/ ./lib/ ./internal/job/lint/`: cli ok 116s, cmd/grid no test files, lib ok, lint ok. |
| Rules: new tests red on base | HOLDS, pasted above (open.go stashed, three tests failed). |
| Rules: no existing test edited or deleted | HOLDS; only new file internal/cli/ak_open_home_test.go. |
| Rules: no guard, gate, cap | HOLDS; none added. |
| Rules: *RealPlugin* tests | Not needed by any row. |
| App-side walk tests | Not applicable (plugin repo). |

## What changed

- internal/cli/open.go: when the operand is empty, `runOpen` resolves the cwd; no error and `TaskPath == ""` selects route `#/` and skips the task lookup. A resolve error or a folder in a task goes on through the unchanged code (so a guest outside an invited project still fails). Non-empty operands are untouched. The rest (ensure, launch code, JSON, no-address line) is the same code path.
- internal/cli/ak_open_home_test.go: four tests, helpers prefixed `akOpen`.
- docs/lanes/C-ak-open-home-without-task.md: this report (outside the Touches list, required by the runbook).

## NOT VERIFIED on a live screen

That the real app renders Home at `#/` after `/api/launch` redirects; no browser was run.

Verdict: DONE
