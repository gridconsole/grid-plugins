# Grid

Grid is a plugin for Claude Code and Codex. It keeps tasks, Jobs and shared knowledge (the brain) in one folder on your machine, and gives agents one command line, `grid`, that makes every write to them.

The plugin ships the agents, skills and hooks, plus a prebuilt `grid` binary for each platform. You need no Go, Node or build step.

## Install

    claude plugin marketplace add gridconsole/grid-plugins
    claude plugin install grid@grid

`claude plugin update grid@grid` moves you to a newer release.

## First run

Run `grid setup` once. It asks before it writes, and adds these entries to your Claude Code settings:

- an allow rule for `grid` commands and for reading and editing the Grid home;
- deny rules that keep agents from editing task folders, the brain, a project's `project.md` and the settings files Grid guards, so the writes go through `grid`.

`grid setup --sandbox` also switches on Claude Code's own sandbox for your user. `grid setup --remove` takes out only what setup wrote. `grid doctor` checks the home, git, the launcher and these settings.

## Where Grid keeps its data

Everything Grid owns lives in the Grid home, `~/.grid` by default. Set `GRID_HOME` to use another folder. Project records and task files live there, outside your repositories.

Outside the Grid home Grid writes only to your Claude Code settings, `~/.claude/settings.json` (or `settings.json` in `CLAUDE_CONFIG_DIR`), and to `~/.claude/agents/`, where it renders agent files from the plugin's templates.

## What is in the plugin

- `bin/grid`: the launcher. It picks the binary for your machine and runs it. On Windows, `bin/grid.cmd` does the same from cmd.
- Five binaries: `grid-darwin-arm64`, `grid-darwin-amd64`, `grid-linux-amd64`, `grid-linux-arm64` and `grid-windows-amd64.exe`.
- `skills/`, `agents/` and `agent-templates/`, the hooks in `hooks/hooks.json`, and the Feed server in `.mcp.json`.

`grid help` lists every command.

## Licence

MIT. See [LICENSE](LICENSE).
