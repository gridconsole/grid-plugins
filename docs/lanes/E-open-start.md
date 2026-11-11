# E-open-start: grid open starts grid-server and grid-worker when they are not running

Head: see the report line from the hand-off (two commits on lane-E-open-start, on top of main 38a3d1b2).

## Proof table

Exit codes below are the shell exit of the command named. A call that printed `ok` or nothing exited 0.

| Row | Result | Proof |
|---|---|---|
| 1. How the binaries are found today; lock or pid file | HOLDS | **Today nothing in the plugin finds or starts either binary.** `grep -rn "grid-server" --include=*.go .` has no code hit (only role names in test strings). `grid-worker` in Go is the agent name and the presence reader: `internal/worker/worker.go:1-3` ("never starts workers"), `internal/cli/worker.go:67`, `internal/cli/cli.go` (`grid worker` list). `app-url`: read at `internal/cli/open.go:21` (`AppURLFile`) and `:27`; the app side that writes it is not in this repo (`docs/lanes/P-grid-open.md:16`). **This lane's lookup**: `internal/cli/open_start.go:88-122`: the folder of the grid program, the folder of its symlink target, `<Grid home>/bin`, then PATH (a test sets `AppStart.Dirs` to search only a fake folder). **Liveness**: worker = presence `reported` from `internal/worker/worker.go:90-137` (fresh worker.json plus matching worker.lock), read at `open_start.go:51`; server = `app-url` holds an address that accepts a TCP connection, `open_start.go:68`. **Lock**: `<Grid home>/open-start.lock`, `open_start.go:130-136`, taken with `fslock.OpenNoFollow` and `fslock.LockExclusive`, the helper `.task.lock` uses (`internal/feed/identity.go:118`). No pid file. |
| 2. Start when not running, wait for app-url, open; start nothing when running | HOLDS | Hook at `internal/cli/open.go:126-131`, start in `open_start.go:125-200`. `TestOpenStartsWorkerAndServerWhenNotRunning` (no app-url, no worker: both start, page opens at the address the server wrote), `TestOpenStartsNothingWhenTheAppRuns` (zero starts, empty stderr), `TestOpenReplacesAStaleAppURL` (app-url names a dead port: server restarted, worker not, file replaced), `TestOpenServerThatWritesNoAddressFails` (no app-url within the wait: exit 1, no browser). `go test ./internal/cli -run TestOpen -count=1` exit 0. `GRID_APP_URL` set: the app is not ours, nothing starts (`TestOpenGridAppURLStartsNothing`). |
| 3. Two opens at once start each binary once | HOLDS | `TestOpenParallelStartsEachProgramOnce`: two `runOpen` calls released together against fake shell programs that append to a marker file; worker 1, server 1, both open the page. Lock held from the first liveness read to the server's address (`open_start.go:130-141`); the second call re-reads liveness under the lock (`:142`). Fails on base, below. |
| 4. darwin, linux, windows | HOLDS (compile and vet; not run on Windows or macOS) | `internal/cli/open_start_unix.go` (`//go:build unix`, `Setsid`), `open_start_windows.go` (`DETACHED_PROCESS\|CREATE_NEW_PROCESS_GROUP`), `open_start_other.go` (`!unix && !windows`, no-op). `GOOS=windows go vet ./internal/cli/...` exit 0. `GOOS=darwin go vet ./internal/cli/...` exit 0. `CGO_ENABLED=0 GOOS=windows go build ./...` exit 0. `CGO_ENABLED=0 GOOS=darwin go build ./...` exit 0. |
| 5. Missing binary: one plain line, exit 3, no browser | HOLDS | `open_start.go:160-169`; `TestOpenMissingProgramExitsThreeAndStartsNothing`: exit 3, stderr is one line naming `grid-server` and the folder searched, stdout empty, no browser, and the worker is not started either (nothing half up). Line: `grid open: grid-server not found (looked in <folders> and PATH). Install the Grid app, then run grid open again.` |
| 6. go test ./internal/cli/... and make lint | HOLDS | `go test ./internal/cli/... -count=1` exit 0 (`ok internal/cli 22.457s`). `make lint` exit 0 (`go vet ./...` and `gofmt -l .` clean). |
| Importers of internal/cli tested | HOLDS | `go list` (imports, test imports) gives: cmd/grid, contract/formats, internal/golden, internal/mcp, internal/packaging, internal/question, internal/settings. `go test ./cmd/grid/... ./contract/formats/... ./internal/golden/... ./internal/mcp/... ./internal/packaging/... ./internal/question/... ./internal/settings/... -count=1` exit 0 (cmd/grid has no test files). |
| New tests fail on base | HOLDS | With `internal/cli/open.go` put back to main's and everything else of the lane in place (so the tests compile), five fail: `open_start_test.go:127: starts: worker 0, server 0, want 1 and 1`; `:187: two opens at once: worker started 0 times, server 0, want 1 and 1`; `:211: starts: server 0, worker 0, want 1 and 0 (the worker ran)`; `:225: exit 0, want 3: grid open: the app's address is not known. Start the app, then run grid open again.`; `:263: exit 0, want 1: ...`. On pure main the test file does not compile (`Env.AppStart` does not exist); that was not run. `open.go` was restored to the lane's version afterwards (`git status` clean). The two other new tests (`StartsNothingWhenTheAppRuns`, `GridAppURLStartsNothing`) pass on base by design: they pin that nothing starts. |
| No existing test edited or deleted | HOLDS | `git diff origin/main --stat -- internal/cli` lists no existing test file; the five existing `TestOpen*` tests pass unchanged. Nothing deleted. |
| Small commits, one sentence, no attribution | HOLDS | Two commits, one sentence each, no trailer lines; no personal paths, hosts or emails in the diff. |
| No guard, gate, cap, hold on agents | HOLDS | None added. The lock only makes two `grid open` calls start the app one after the other; it is released when the first has the server's address or after the 30 s wait. Nothing blocks an agent. |

## What changed

- `internal/cli/open.go`: when `Env.AppStart` is set and `GRID_APP_URL` is empty, `runOpen` calls `ensure` before it reads the address. Nothing else in the file changed.
- `internal/cli/open_start.go` (new): liveness (worker presence, server connect), program lookup, the lock, detached start with output in `<Grid home>/logs/<name>.log` and `GRID_HOME` passed on, removal of a stale `app-url`, the wait (30 s), the exit 3 line.
- `internal/cli/open_start_unix.go`, `_windows.go`, `_other.go` (new): `detach`.
- `internal/cli/cli.go`: **outside the brief's file list**, two lines: the `Env.AppStart` field and `AppStart: &AppStart{}` in `OSEnv()`. Needed so the existing `TestOpen*` tests, which build an `Env` without it and expect no start, stay unedited, while the real command (built by `OSEnv`) starts the app. No other place sets it.
- `internal/cli/open_start_test.go` (new): seven tests with fake shell programs (skipped on Windows).

## Open items for the owner

- Where the Grid app installs `grid-server` and `grid-worker` is not written in this repo. The lookup order (beside grid, its symlink target, `<Grid home>/bin`, PATH) is my choice; change `searchDirs` if the app installs elsewhere.
- A second `grid open` a few seconds after a start, before the new worker has written worker.json, would try to start a second worker. The worker's own one-lock-per-machine rule (`internal/worker/worker.go:16-18`) is what refuses it; no test covers that case.
- Server liveness is a TCP connect to the `app-url` address, not an HTTP request.
- A stale `app-url` is deleted before the server starts; a live server whose port refused one connect would lose its file until it is restarted.
- A server that does not write `app-url` within 30 s gives exit 1 and a line naming `logs/grid-server.log`.

## NOT VERIFIED on a live screen

- The real `grid-server` and `grid-worker`, and the real browser opening on a rebooted machine.
- Windows and macOS at run time: only vet and build were run. The Windows detach flags (no console, own process group) and the unix `Setsid` (survives closing the terminal) have not been seen to outlive the terminal.
