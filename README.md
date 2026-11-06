# Grid plugins

Grid is an agentic development environment: an opinionated way of working with coding agents, with the task as the central unit of work. Each task gets its own folder, a planner, a written plan you approve, briefs for every step, and a recorded result.

This repo holds the open-source agent plugins that carry Grid's workflow. They work in a plain terminal session, with or without the Grid app.

| Folder | For |
| --- | --- |
| `plugins/grid` | Claude Code: agents, skills and hooks |
| `plugins/grid-codex` | Codex: hook adapters that run the same scripts as `plugins/grid` |

## Install in Claude Code

```sh
claude plugin marketplace add gridconsole/grid-plugins
claude plugin install grid@grid
```

Restart Claude Code. `/agents` then lists the `grid:*` agents.

### Update

```sh
claude plugin marketplace update grid
claude plugin update grid@grid
```

### Requirements

- Claude Code 2.1.78 or later.
- Node.js on your PATH. The hooks are small Node scripts.

## What you get

**Agents.** Each step of a task goes to an agent with one role:

| Agent | Role |
| --- | --- |
| `grid-planner` | Plans a task in its folder |
| `grid-brief-writer` | Writes the brief for a step |
| `grid-scout` | Maps where things are in the code |
| `grid-researcher` | Researches a question from docs, source and the web |
| `grid-designer` | Writes a design spec |
| `grid-worker` | Carries out a brief: code, UI, docs or files |
| `grid-runner` | Runs a build, test or check |
| `grid-reviewer` | Reviews finished work |
| `grid-checker` | Checks pages in a browser, read-only |
| `grid-operator` | Does browser work that changes state |
| `grid-curator` | Turns what a task learned into project notes |

**Skills** for each part of a task: `task`, `intake`, `research`, `plan`, `execute`, `questions`, `deliver`, `resume`, `views` and more.

**Hooks** that keep the work honest. A subagent does not start on an incomplete brief. A session does not write past a plan nobody approved. A session does not stop with a task half written. Every main session is told to hand work to the agents above.

## Where Grid keeps its data

Grid writes nothing into your repos. Tasks, plans and notes live in the Grid data folder, `~/.grid/<name>/projects/<project>/`.

## Without the Grid app

The Grid app shows the plan and gives you the Approve button. Without it, a session stops at the plan. To let it carry on, restart Claude Code with `GRID_PLAN_GUARD=off`.

## Codex

`plugins/grid-codex` is the source of the Codex plugin. The Grid app stages it for each Codex account, adding the shared hooks and skills. See its README for the layout.

## Licence

MIT. See [LICENSE](LICENSE).
