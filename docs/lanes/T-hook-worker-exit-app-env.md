# T-hook-worker-exit-app-env

Repo: plugin. Branch lane-T-hook-worker-exit-app-env, from main 3ccec5cb (the brief names 330256ce; main has moved to 3ccec5cb and the base proof below ran on 3ccec5cb). Code head 04d47a21; this report is committed after it.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. A hook test that fails on main | HOLDS | `TestAppEnvWorkerExitEndsTheRecordErrored`, internal/hook/exit_app_env_test.go:43, through `eventSetup` / `f.run`. On a detached worktree of main 3ccec5cb (`go test -count=1 ./internal/hook/ -run TestAppEnv`, exit 1): `exit_app_env_test.go:43: a crash in a Job session with the app's names did not end the record Errored: {... State:Liveness unknown ... Ended:<nil> ...}`. The mixed-name test fails on main too (`exit_app_env_test.go:156: mixed names did not mark a Worker`, both subtests). |
| 2. Same SessionEnd ends the record Errored and stores the reason under the Job's key; a posted result still writes nothing | HOLDS | `TestAppEnvWorkerExitEndsTheRecordErrored` asserts `State == Errored`, `Ended != nil`, `ExitReason == "crash"` and `Exits["exit::J-1::"] == "crash"` (the key of events.go:297, the same one exit_test.go's fixture produces). `TestAppEnvWorkerExitSkipsAJobThatAlreadyHasAResult` uses the app's names and the contract result fixture: record not ended, no exit stored. Both pass on the lane: `go test -count=1 -race ./internal/hook/` exit 0. |
| 3. Marker is (GRID_FEED_TASK or GRID_TASK) and GRID_JOB and (GRID_WORKER or GRID_WORKER_ID); one test per missing piece | HOLDS | internal/hook/events.go:274-278 (`hasTask`, `hasWorker`). `TestAppEnvWorkerExitNeedsTheWholeMarker` has `without GRID_JOB` (the plain-session case: GRID_TASK, no GRID_JOB), `without both worker ids`, `without both task names`; each asserts `Ended == nil`, not Errored, no `ExitReason`, no `Exits` (the assertions of exit_test.go:79-85). `TestAppEnvWorkerExitAcceptsMixedNames` covers old task name + app worker name and app task name + old worker name. All pass. |
| 4. A CLI test that fails on main; after the change both act on GRID_TASK's task; GRID_FEED_TASK beats GRID_TASK; `--task` beats both | HOLDS | internal/cli/app_env_task_test.go, GRID_ROLE=worker, GRID_TASK set, GRID_FEED_TASK unset, cwd `<home>/projects/example/worktrees/J-1` outside the task, no `--task`. On main 3ccec5cb (`go test -count=1 ./internal/cli/ -run TestAppEnv`, exit 1): `app_env_task_test.go:59: feed read with only GRID_TASK: exit 1 out  err grid feed: no task selected; use --task` and `app_env_task_test.go:68: job post from .../worktrees/J-1 with no --task: exit 1 no task selected; use --task`. On the lane both pass (`TestAppEnvFeedReadDefaultsToGridTask`, `TestAppEnvJobPostDefaultsToGridTask`). Order: `TestAppEnvFeedTaskOrder` (GRID_TASK alone; GRID_FEED_TASK beats GRID_TASK; `--task` beats both; `--task` beats GRID_TASK) checks which task folder gets the Feed; `TestAppEnvJobPostTaskOrder` does the same for `grid job post`. Code: internal/cli/feed.go:193-196 and internal/cli/orchestrator.go:141-146. |
| 5. Nothing new is refused | HOLDS | `TestAppEnvMCPReadsGridTaskAndRefusesNothing` through `mcpCall`: GRID_TASK set, GRID_FEED_TASK unset: `feed_read` with `{}` reads GRID_TASK's task and not the other; `feed_read` with `task` naming another task answers that task with no `isError`; `feed_post` naming another task is not refused and lands there. `TestAppEnvWorkerIDIsNotCheckedAsTheWriter` (4 GRID_WORKER_ID values, GRID_WORKER unset): a `--task` CLI post and an MCP post under GRID_FEED_TASK pass; this test passes on main as well (it guards, it does not change). `TestAppEnvWorkerIDWithGridTaskIsNotRefused`: CLI and MCP posts under GRID_TASK with GRID_WORKER_ID=not-a-worker pass. `git diff main -U0 -- internal/cli/feed.go` has one hunk, at 193-196 inside `feedStore`; none inside `sessionStore` (feed.go:231-251 on the lane, 227-247 on main) and none at the GRID_WORKER check (feed.go:334 on the lane, 330 on main). The lane's only other diff is this report under docs/lanes, which the runbook asks for. |
| 6. `git diff main --stat` lists only the Touches files; the four named files are untouched and their tests pass | HOLDS | `git diff main --stat` (exit 0): docs/feed-mcp.md, internal/cli/app_env_task_test.go, internal/cli/feed.go, internal/cli/orchestrator.go, internal/hook/events.go, internal/hook/exit_app_env_test.go. exit_test.go, job_post_agent_test.go, mcp_test.go and testdata/golden/mcp-session.golden are not in it. `go test -count=1 -v -run 'TestWorkerExit\|TestJobPost\|TestMCP\|TestCodexFeedCLI' ./internal/hook/ ./internal/cli/` exit 0, both ok. |
| 7. `go test -count=1 -race ./internal/hook/ ./internal/cli/` 0 fail; vet exit 0 | HOLDS | Run on the final code: `ok internal/hook 4.535s`, `ok internal/cli 61.294s`, exit 0. `go vet ./internal/hook/ ./internal/cli/` exit 0. |

## Rules

| Rule | Result | Proof |
|---|---|---|
| Small commits, one-sentence messages, no attribution lines | HOLDS | `git log main..HEAD` has four code commits and this report commit, one sentence each, no Co-Authored-By or session line. No personal path, host or email is in the diff. |
| New tests fail on base and pass on the lane; no existing test edited | HOLDS | The two new files are the only test changes; the failing lines are pasted in rows 1 and 4 (proved on a detached worktree of main, made inside the clone and removed with `git worktree remove -f`; my clone was never switched). No test deleted. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The diff only widens what is accepted: the hook marker accepts more names, the two CLI defaults gain a fallback. No refusal added; `sessionStore` and the GRID_WORKER check are untouched. `workerExit` still returns nothing, blocks nothing and prints nothing. |
| Packages that import the changed ones tested | HOLDS | `go list -f '{{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}} @{{.ImportPath}}' ./...` filtered for internal/hook and internal/cli gives: cmd/grid, contract/formats, internal/cli, internal/golden, internal/mcp, internal/packaging, internal/question, internal/settings. `go test -count=1` of cmd/grid (no test files), contract/formats, internal/golden, internal/mcp, internal/packaging, internal/question, internal/settings: exit 0, all ok. |
| Lint | HOLDS | `make lint` exit 0 (the first run flagged gofmt on my new cli test; fixed in the commit "Format the app-env task test."). |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| UI changes | not applicable | No UI file touched. |

## What changed

- internal/hook/events.go: `workerExit` marks a Worker by (GRID_FEED_TASK or GRID_TASK) and GRID_JOB and (GRID_WORKER or GRID_WORKER_ID); comment updated. The store update below it is untouched.
- internal/cli/feed.go: `feedStore` defaults the task to GRID_TASK when `--task` and GRID_FEED_TASK are empty. MCP `feed_*` goes through `feedStore` when GRID_FEED_TASK is unset, so it gets the same default with no edit to `sessionStore`.
- internal/cli/orchestrator.go: `grid job post` has the same order (`--task`, GRID_FEED_TASK, GRID_TASK, cwd); comment updated.
- docs/feed-mcp.md: one sentence in the GRID_FEED_TASK row.
- New tests: internal/hook/exit_app_env_test.go, internal/cli/app_env_task_test.go.

## Open items for the reviewer

- `taskDir` (hook) reads GRID_TASK first and the CLI reads GRID_FEED_TASK first. With the two set to different tasks the hook and the CLI would name different tasks. The brief says keep `taskDir`'s order; the app sets only GRID_TASK, so the two never differ in a real Job session.
- SessionStart's `writeEnv` exports GRID_TASK into a plain session too (hook.go:153-160). In such a session `grid feed` and `grid job post` now take GRID_TASK before the cwd, as the brief's order says; before, a session that moved to another task's folder got that folder's task. This only changes which task is chosen; it refuses nothing.

## NOT VERIFIED on a live screen

No screen is involved. Not run here by the brief's rules: a real Job session started by the app's worker (`claude`), the plugin loaded through I-worker-plugin-dir, and the Empty finish scorecard row reading the ended record. The orchestrator does that live check.
