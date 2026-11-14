# Grid plugins

Grid is an agentic development environment: an opinionated way of working with coding agents, with the task as the central unit of work. It keeps tasks, Jobs and shared knowledge (the brain) in one folder on your machine, and gives agents one command line, `grid`, that makes every write to them.

This repo holds the open-source plugin that carries Grid's workflow. It works in a plain terminal session, with or without the Grid app.

| Folder | For |
| --- | --- |
| `plugins/grid` | Claude Code and Codex: agents, skills, hooks and the `grid` binary for each platform |

## Install in Claude Code

```sh
claude plugin marketplace add gridconsole/grid-plugins
claude plugin install grid@grid
```

Then run `grid setup` once. See [plugins/grid/README.md](plugins/grid/README.md) for what it writes and what the plugin holds.

### Update

```sh
claude plugin marketplace update grid
claude plugin update grid@grid
```

## Licence

MIT. See [LICENSE](LICENSE).
