# Grid plugin

The Grid plugin for Claude Code and Codex: agents, skills, hooks and one Go command line, `grid`, that does every write to task folders, the Feed and the brain.

This repo replaces the earlier JavaScript plugin. It is public and MIT licensed.

## Build and test

    make build    # ./dist/grid
    make test
    make lint     # go vet and a gofmt check
    make golden   # golden-file tests; UPDATE_GOLDEN=1 make golden rewrites them

`bin/grid` is a dev launcher: it follows symlinks to this checkout, runs `go build` into `dist/grid` on every call (a no-op when nothing changed) and runs that. A hook call whose build fails or takes over 8 seconds prints nothing and exits 0.

## Commands working today

`grid version`, `grid where`, `grid doctor`, `grid hook <event>`. The other command groups are listed by `grid help` and print "not built yet". The `--json` shapes are in [contract/README.md](contract/README.md).

To do: `grid mcp`, the stdio server with the Feed tools. The plugin's `.mcp.json` comes back with it.

## Layout

    cmd/grid/        entry point
    internal/        packages: cli, where, doctor, hook, project, gitx, gridhome, version
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
