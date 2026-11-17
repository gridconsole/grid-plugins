# C-ak-open-home-without-task: `grid open` with no task and no enclosing task folder opens Grid's Home page instead of failing

Repo: plugin. Clone <lanes>/C-ak-open-home-without-task on lane-C-ak-open-home-without-task from main. Model: sonnet, high. About 40 calls.
Review: none
Why: scope T380 of SCOPES-11nov-0320a.md, owner question Q4 on its default (`grid open` in a project folder with no task opens Home `#/`). On plugin main 2221cb86 `runOpen` (internal/cli/open.go:99) sends an empty operand to `resolveTaskOperand` (open.go:138). For an empty operand that function resolves the cwd (internal/cli/task_operand.go:41-44) and, when the folder is in no task, returns `errors.New("give a task id or folder, or run inside a task folder")` (task_operand.go:46-47). runOpen then returns through taskOperandError (open.go:139) before `env.AppStart.ensure` (open.go:153-158) and before the launch URL (open.go:169-177). So a new person, who has no task yet, runs `grid open` in a terminal and gets no app and no page. The route is built only as a task route (open.go:148, `"#/t/" + project + "/" + id + "/overview"`). The README lists `grid open` for "open the task page" (README.md:30); this lane changes no help text, no flag and no contract row (contract/commands.json:80-81 stays).
Base check: quote the base sha, open.go:138 and task_operand.go:47. If `grid open` with no operand outside a task folder already opens `#/`, stop NOT DONE and name the line.

## What to build (one branch in runOpen)
- In runOpen, when the operand is empty: resolve the cwd as today. If the resolve answers without error and names no task (`TaskPath == ""`), use the route `#/` and carry on through the rest of runOpen unchanged: `ensure` when AppStart is set and GRID_APP_URL is empty, the launch code (`<base>/api/launch?code=<code>#/`) on a local app, `<base>/#/` otherwise, `--json` prints `{"url": ...}`, and without a known address it prints `#/` and the existing line "grid open: the app's address is not known. Start the app, then run grid open again." (open.go:166). Exit 0.
- Unchanged: an empty operand inside a task folder opens that task; a non-empty operand goes through resolveTaskOperand exactly as today, so `grid open T-99` still fails (TestOpenUnknownTaskFails, open_test.go:115). A resolve that answers with an error (for example a guest outside an invited project, where.go:124-126 ErrProjectUnregistered) still fails as today: only "no error, no task" becomes Home. The guest parity walk (internal/cli/guest_parity_test.go:145 runs `open` with no operand from the harness root; TestGuestProjectParity :354) must pass unedited.
- Inside a registered project folder with no task: Home `#/` too (Q4 default), not the project board.
Touches: internal/cli/open.go (the empty-operand branch only); new test file internal/cli/ak_open_home_test.go. No change to task_operand.go (other commands share it), contract/commands.json, internal/cli/cli.go, inprocess.go (hsIpTable already holds `open`, inprocess.go:36) or golden help.
Helpers: every package-level helper in the new file starts with `akOpen`, tests with `TestAkOpen`. The package already has openFixture, openFixtureAt (open_test.go:16, :22), wbOpenHome, wbOpenBase, wbOpenCode (wb_open_launch_url_test.go:12-34) and startFixture (open_start_test.go:35); reuse them, define none of those names.
Shared file: open.go was last edited by C-wb2-open-no-code-line, which is on plugin main. No other brief in this folder touches the plugin.
Cross-repo: changes no `grid job post`, `grid task new`, approve, answer, refusal code or Feed record shape; `git -C <ref>/app.git grep -n '"open"' main -- '*realplugin*'` on app 7c0c499f finds only `"open"` as a question or job state (et_person_job_answer_realplugin_test.go:177, fj_question_defaulted_realplugin_test.go:98, fx_job_show_realplugin_test.go:84), no rig that runs `grid open`.
Needs tmp/design: no.
Skills: go-conventions (or the Go skill named in the builder runbook).

## Acceptance rows
1. `TestAkOpenHomeOutsideProject`: with app-url `http://127.0.0.1:8484/` and the launch-code file as wbOpenHome writes them, `grid open` with no operand run from a folder outside every project exits 0 and hands the browser exactly `http://127.0.0.1:8484/api/launch?code=<wbOpenCode>#/`; without the code file it hands `http://127.0.0.1:8484/#/` and stderr holds "grid open: no launch code was found; the page opens without one." (open.go:175). Red on base: exit non-zero with "give a task id or folder, or run inside a task folder" (task_operand.go:47); paste that line.
2. `TestAkOpenHomeInProjectFolder` and `TestAkOpenHomeJSON`: run from the project folder of openFixture with no task folder as cwd, `grid open --json` prints exactly one field `url` ending in `/#/`; with no app-url and no GRID_APP_URL it prints `{"url":"#/"}` and stderr holds "grid open: the app's address is not known. Start the app, then run grid open again." (open.go:166). Red on base: as row 1.
3. `TestAkOpenTaskStillOpensTask`: from inside the task folder of openFixtureAt, `grid open` with no operand still hands the browser the task route `#/t/<project>/<id>/overview` (open.go:148), and `grid open T-99` still exits non-zero with no address opened.
4. `go test -count=1 -v ./internal/cli/ -run 'TestAkOpen|TestOpen|TestWbOpen|TestWaOpen|TestGuestProjectParity'` exits 0 with a `--- PASS:` line for each TestAkOpen test and no `--- FAIL`.
5. Plugin walk rows: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` and `go test -count=1 ./internal/job/lint/` exit 0.
6. `make lint` and `make cross` exit 0 (Makefile:27, :33).

## Rules
- Follow the builder runbook and its Definition of Done proof table: one line per acceptance row, HOLDS with command and exit code or file:line, or NOT DONE.
- Proof is what you can run in your sandbox: go test and make targets. Do not run claude or curl, start no background server, touch no real billing, deploy nothing.
- A row whose answer differs is reported with the request and the response as seen; do not change production code beyond the file named in Touches.
- Owner decisions: no Grid-made guards, caps or limits; approvals and gates come from the workflow with a default; everything is a Job; tasks carry on by themselves until a person point.
- No guard, gate, cap or limit on agents. Remove nothing. No pushing anywhere.
- New tests go in the one new file named in Touches, never as rows in a shared test table.

Report must end with DONE or NOT DONE.
