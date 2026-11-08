# Grid plugin

The Grid plugin for Claude Code and Codex: agents, skills, hooks and one Go command line, `grid`, that does every write to task folders, the Feed and the brain.

This repo replaces the earlier JavaScript plugin. It is public and MIT licensed.

## Build and test

    make build    # ./dist/grid
    make test
    make test-clean # same suite with the temp leak guard, also included in make test
    make lint     # go vet and a gofmt check
    make golden   # replays the legacy corpus and prints every case; UPDATE_GOLDEN=1 go test ./... rewrites the testdata golden files

`make test` runs `go test -count=1 ./...` with the caller's default temp location.
The guard compares directory entries before and after the suite and fails on new
entries with the test prefixes listed in `internal/testclean/main.go`. Existing
entries are left alone. Run it without other tests using the same prefixes in the
same system temp folder. Test subprocesses get a parent-owned `t.TempDir()` through
`internal/testenv`; in-process CLI tests temporarily set TMPDIR with `t.Setenv`.

`bin/grid` is a dev launcher: it follows symlinks to this checkout, runs `go build` into `dist/grid` on every call (a no-op when nothing changed) and runs that. A hook call whose build fails or takes over 8 seconds prints nothing and exits 0.

## Commands working today

`grid version`, `grid where`, `grid doctor`, `grid hook <event>`, and `grid task new|show|list`. The other command groups are listed by `grid help` and print "not built yet". The `--json` shapes are in [contract/README.md](contract/README.md).

    grid task new "Fix the search label" --size small --json
    grid task show T-01 --json
    grid task list --state queued --json
    grid task new "Sort my notes" --project personal

Tasks use the current folder's project, or Personal when no project applies. `--project` accepts a registered project's name or id, or `personal`. A new task preserves the quoted request exactly in `intent.md`, starts in `state: queued`, and defaults to `size: auto`. `--workflow` stores a workflow name; omitting it leaves selection to the workflow rules. Creation does not start work. `show` needs no id inside a task folder or its descendants. `list` reads one project and optionally filters by an exact state.

To do: `grid mcp`, the stdio server with the Feed tools. The plugin's `.mcp.json` comes back with it.

## Layout

    cmd/grid/        entry point
    internal/        packages: cli, task, where, doctor, hook, project, gitx, gridhome, version
    agents/ skills/  shipped with the plugin
    topics/          brain topic schemas
    contract/        Feed contract schemas and fixtures
    hooks/           hooks.json, calling `grid hook <event>`
    views/           page templates
    evals/ corpus/   eval suites and corpus tasks, never loaded in a session
    bin/grid         launcher
    testdata/        golden files

## Data folder

`GRID_HOME` overrides the data folder; the default is `~/.grid`. It holds `VERSION` (the format version), `machine` (this computer's id) and `projects/<name>/project.md`, each project's record: YAML front matter with its `id`, its folder per machine, its repos with their normalised remotes, and its match keys. The format is in `internal/project/project.go`.

Task folders are `projects/<name>/tasks/T-NN-slug/`; Personal tasks are `personal/tasks/T-NN-slug/` with `project: personal`. Display numbers increase across both locations. Creation takes an exclusive `flock` on `.task-alloc.lock` through `internal/fslock` and publishes a complete folder by rename. Linux and macOS are supported; elsewhere `grid task new` fails with the same "not supported" error as the lease and session commands. Project records are created on the first task write; task reads create nothing.
