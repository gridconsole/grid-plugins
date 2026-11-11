# Lane E-defaults-plugin

Unset projects accept tasks and the Grid home can move, each with a test.

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. Code that decides whether a project accepts a task | HOLDS | `internal/task/create.go:30` (`Store.New`) calls `Store.resolve` at `internal/task/task.go:108`, which calls `where.Resolver.Resolve` at `internal/where/where.go:84`. `Resolve` refuses only an unavailable named project (`ErrProjectUnregistered`, `internal/where/personal.go:17`, raised at `where.go:125` and in `explicit`) or a caller the Personal rule bars. It reads no per-project "accepts tasks" setting: `project.Record` (`internal/project/project.go:36`) has id, folders, repos and keys only, so an unset project is accepted. |
| 1. Places that build a Grid home path without gridhome | HOLDS | Search `/.grid`, `".grid"`, `os.UserHomeDir`, `GRID_HOME`. One path built by hand in production code: `internal/guard/context.go:40` (`filepath.Join(env["HOME"], ".grid")`), now moved to `gridhome.Dir`. `internal/cli/cli.go:72` calls `os.UserHomeDir` and passes it to `gridhome.Dir` at `cli.go:75` (already through gridhome). `internal/eval/runner.go:143` and `:256` build `<sandbox home>/.grid` inside the eval's own temp sandbox and set `GRID_HOME` to it for the child process; left as is, it is a harness fixture, not a Grid home lookup. `internal/settings/claude.go:60-61` are the `~/.grid-person` deny rules for Claude, another folder. All other hits are error text and comments. |
| 2. Project with no setting accepts tasks | HOLDS | `internal/task/unset_project_test.go` (`TestProjectWithNoSettingAcceptsTasks`, three cases: folder with no project.md, record holding only its folder, project.md with empty front matter). `go test ./internal/task -run TestProjectWithNoSettingAcceptsTasks -count=1` exit 0. It also passes on base: the default already held in code, so this test pins it and does not fail on base. |
| 3. Every Grid home path goes through gridhome | HOLDS | `internal/cli/grid_home_moves_test.go` (`TestGridHomeMovesWithGRIDHOME`): real `OSEnv`, `HOME` set to a temp "real home", `GRID_HOME` set to another temp folder; runs `task new`, `brain propose` and `accept`, `claude settings show`; checks the task and VERSION appear under the moved home and the stand-in real home holds nothing. `go test ./internal/cli -run TestGridHomeMovesWithGRIDHOME -count=1` exit 0 (also passes on base, since cli already used gridhome). `internal/guard/context_home_test.go` (`TestContextGridHomeFollowsGridhome`) fails on base and passes on the lane. Base failing line: `context_home_test.go:28: moved home, spelled loosely: home ".../data/../data/grid/" ... want ".../data/grid"`. |
| 4. go test and make lint: 0 fail | HOLDS for the changed set | `go test -count=1 ./internal/guard ./internal/hook ./internal/cli ./internal/packaging ./internal/task` exit 0 (guard, plus its importer hook, plus importers of hook: cli and packaging; and task). `go list` import chain: guard <- hook <- cli, packaging. `make lint` (go vet ./...) exit 0. The whole-module test is the orchestrator's. |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| gofmt | HOLDS | `gofmt -l internal/guard internal/cli internal/task` printed nothing |
| No guard, gate, cap, hold or hook added | HOLDS | The change is one read of GRID_HOME, moved into `gridhome.Dir`, and three tests. No new refusal. |
| No existing test edited or deleted | HOLDS | `git diff origin/main --stat` shows only new test files and `internal/guard/context.go`. |
| No attribution lines, no personal paths | HOLDS | Commits carry a one-sentence message only. Tests use temp folders and `/home/example`. |
| Out of scope | HOLDS | Roles, approver groups, SSH worker install not touched. |

## What changed

- `internal/guard/context.go`: the guard's Grid home now comes from `gridhome.Dir` (GRID_HOME, else `<HOME>/.grid`). A loosely spelled GRID_HOME (trailing slash, `..`) is cleaned the way the CLI cleans it, so the guards and the CLI judge one folder. When GRID_PROJECTS is set the old behaviour is kept.
- New tests: `internal/task/unset_project_test.go`, `internal/cli/grid_home_moves_test.go`, `internal/guard/context_home_test.go`.

## Notes

- The brief's rows 2 and 3 were already true in code for the task and CLI paths. Those two tests pin the defaults and do not fail on base. Only the guard test fails on base.
- There is no "accepts tasks" project setting anywhere in the code; the default is that nothing refuses.

## NOT VERIFIED on a live screen

Nothing in this lane touches a UI. The dev UI check with a real `GRID_HOME` move is the orchestrator's.
