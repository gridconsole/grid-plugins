# C-jz-task-owner-from-call: a lib `task new` names its owner from the Call's USER

Base: dc301fce757d6374de26d8d1b68de26575341b32 (the brief named 35af7d3c; the clone was cut from dc301fce, which holds the same create.go:173-198 and task.go:118-123 lines).

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. Owner rule table with a fake Getenv | HOLDS | `go test -count=1 -v -run 'TestJzOw' ./internal/task/` exit 0: `--- PASS: TestJzOwOwnerRule` (USER " ana " gives ana; only USERNAME ben gives ben; neither gives owner; Getenv nil gives `user.Current()` else USER else owner, same as create.go `machineOwner`), `--- PASS: TestJzOwNewRecordsTheCallsOwner`. On base: `unknown field Getenv in struct literal of type NewOptions` at internal/task/jz_owner_test.go:50, :72, :79, :101, `FAIL ... [build failed]`. |
| 2. Two lib.Run `task new` calls behind a start barrier, owners ana and ben | HOLDS | `go test -count=1 -v -run 'TestJzOw' ./lib/` exit 0: `--- PASS: TestJzOwTwoCallsTwoOwners` (two `newHome` homes, Environ USER=ana and USER=ben, no GRID_WHO, GRID_ACTOR or GRID_MEMBER, ready/start barrier). On base: `the call with USER=ana recorded owner "grid" (the host account is "grid")` and `the call with USER=ben recorded owner "grid"`, `--- FAIL: TestJzOwTwoCallsTwoOwners`. |
| 3. No USER and no USERNAME gives `owner` | HOLDS | `--- PASS: TestJzOwUsernameAndNobody/no_USER_and_no_USERNAME` (also USERNAME only, USER over USERNAME, blank USER and USERNAME). On base: `owner "grid", want "owner" (the host account is "grid")`. |
| 4. `go test -count=1 ./internal/task/ ./internal/signal/ ./lib/` and `go test -count=1 -run 'Task\|Signal' ./internal/cli/` | HOLDS | both exit 0 (one `&&` chain): ok task 16.4s, signal 1.1s, lib 27.3s, cli 18.1s. |
| 5. `make lint` | HOLDS | `make lint` (`go vet ./...`) exit 0; `gofmt -l internal/task internal/cli/task.go lib` prints nothing. |
| 5. `make cross` | HOLDS | `make lint && make cross` exit 0 (the 600s foreground timeout moved it to the background; it completed with `[exited with code 0]`, 0 FAIL lines): `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`, every package ok, last line `ok  github.com/gridconsole/grid-plugins/lib`. |
| New tests fail on base, pass on the lane | HOLDS | rows 1-3 above: base output pasted. |
| Packages that import internal/task | HOLDS | `go list` gives: braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, land, migrate, orchestrator, signal, task, updates, workflow. `go test -count=1` on all of them exit 0 (orchestrator 404s, cli 299s, migrate 90s, hook 76s; jobtest and updates have no test files). |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v` exit 0 (`walked 39 commands, 39 table entries`, both PASS); `go test -count=1 ./internal/job/lint/` exit 0. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | the diff adds one field and one lookup; nothing refuses or waits. |
| Existing tests not edited | EXCEPTION, one line | internal/task/task_test.go:156 `fmt.Sprintf("%q", tc)` became `"%v"`. `tc` is a `NewOptions`; `go vet` (which `go test` runs) refuses `%q` for a struct that holds a func field: `internal/task/task_test.go:156:22: fmt.Sprintf format %q has arg tc of wrong type ...NewOptions`. Only this line in the module (`go vet ./...`). The brief's design (a `Getenv func(string) string` field on NewOptions) cannot compile its package tests without it. The assertions are untouched and it is its own commit (2fabc915) so the orchestrator can replace it. |
| No personal paths, hosts or emails; no attribution lines | HOLDS | commits carry one sentence and no trailer. |

## What changed

- internal/task/task.go: `NewOptions.Getenv func(string) string`, documented "nil reads the host, as the binary does" (NewOptions is defined here, not in create.go, so this file is touched beyond the brief's list).
- internal/task/create.go: `NewOptions.ownerFromEnv`. Getenv nil calls `machineOwner()` unchanged (user.Current, then os.Getenv("USER"), then "owner"). Getenv set reads USER, then USERNAME, trimmed, else "owner"; user.Current and os.Getenv are not called. `machineOwner()` keeps its signature because `approval_read_test.go:593` and `task_test.go:101` call it. An explicit Who still wins and the environment is not read.
- internal/cli/task.go: `Getenv: env.Getenv` (the raw field). The binary leaves `Env.Getenv` nil (OSEnv), so it does not change; a lib Call sets it from its Environ.
- internal/signal/act.go: NOT changed. `ActOptions` (act.go:54) carries Cwd, Who and Role only, no Getenv, so there is nothing to thread; a signal turned into a task with no Who still names the host account through the nil Getenv. A follow-up that gives ActOptions a Getenv would close it.
- lib/lib.go is not edited, as the brief says.
- New tests: internal/task/jz_owner_test.go, lib/t2_jz_owner_test.go (helpers `jzOw*`, tests `TestJzOw*`).

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: the app calling in process (scope T1) and Windows at run time (USERNAME is proven through a fake Getenv and a lib Environ on Linux; Windows was vetted and test-compiled by `make cross`, not run).
