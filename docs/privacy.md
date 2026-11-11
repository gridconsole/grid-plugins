# Privacy: what the plugin reads and writes outside the Grid home

Grid keeps its data in the Grid home (`~/.grid`, or `GRID_HOME`). It sends no telemetry: crash reports and the daily copy stay in the Grid home (`internal/settings/claude.go:239`). This page lists every file or folder outside the Grid home that the plugin reads or writes, which command or hook does it, and whether the person asked. Line numbers are on plugin base `7d516e1c`. `internal/hook/ex_outside_home_test.go` pins what SessionStart writes outside the Grid home.

| Path outside the Grid home | Read or write | Verb or hook | Asked or automatic | Where |
|---|---|---|---|---|
| `~/.claude/agents/Explore.md` and `~/.claude/agents/grid-*.md` (the folder `agents` next to `settings.json`, so under `CLAUDE_CONFIG_DIR` when that is set) | write | SessionStart at every level except `off`; also `grid agent sync` | automatic at SessionStart; asked for `grid agent sync` | `internal/hook/hook.go:104-106`, `internal/hook/hook.go:140-154`, `internal/settings/agentsync.go:47-53`, `internal/settings/agentsync.go:123`, `internal/cli/agent_sync.go:54` |
| `$CLAUDE_ENV_FILE` (a file Claude Code names; nothing is written when it is unset) | append | SessionStart at every level, `off` included | automatic | `internal/hook/hook.go:125-130`, `internal/hook/hook.go:156-174`, `internal/hook/settings_test.go:104` |
| the Claude Code settings file `~/.claude/settings.json` (or under `CLAUDE_CONFIG_DIR`) | read and write | `grid setup` (permissions, autoMode, and with `--sandbox` the sandbox row), `grid claude` and the budget rows | asked | `internal/settings/claude.go:256-264`, `internal/settings/claude.go:744`, `internal/settings/claude.go:797`, `internal/cli/settings.go:170-181`, `internal/cli/budget.go:89`, `internal/settings/claude_user.go:223-234`, `internal/settings/budget.go:939` |
| `~/.codex/config.toml` (or under `CODEX_HOME`) | read only | the budget rows, `grid claude` | asked | `internal/settings/budget.go:611`, `internal/settings/budget.go:632-637`, `internal/settings/claude_user.go:451` |
| the Claude config folder (`CLAUDE_CONFIG_DIR`, else `~/.claude`) | read only | the account commands; snapshots go to the Grid home | asked | `internal/account/windows.go:58-63`, `internal/account/windows.go:52-54` |
| `~/.claude-<name>` or `~/.codex-<name>`, with links to the files in `~/.claude` or `~/.codex` | write | `grid account folder`; never a credential, history or project file | asked | `internal/account/account.go:318-331`, `internal/account/account.go:352`, `internal/cli/account.go:135` |
| `~/.config/systemd/user`, `~/Library/LaunchAgents`, `~/.grid-routine-timers` (timer files; Windows also runs `schtasks`) | write | routine scheduling | asked | `internal/routine/timer_systemd.go:23`, `internal/routine/timer_plist.go:27`, `internal/routine/timer_schtasks.go:36`, `internal/routine/schedule.go:306-309` |
| the person's repos and worktrees | run a command; write the worktree backlink and plugin session pointer files | `grid land` runs the project's own land command in the folder; a Grid home migration rewrites the backlinks | asked | `internal/land/land.go:238-242`, `internal/migrate/worktree.go:118-124` |
| the support bundle path given with `--out` | write | `grid doctor --bundle` | asked | `internal/cli/doctor.go:16`, `internal/doctor/bundle.go:128-149` |
| temp files: `.grid-claude-*` beside the settings file, `.<name>.*` beside an atomic write, `grid-eval-*` in the OS temp folder | write, then rename or remove | settings writes, atomic writes, `grid eval` | follows the verb that made them | `internal/settings/claude.go:619`, `internal/atomicfile/atomicfile.go:23`, `internal/eval/runner.go:99` |
| `dist/` under a checkout of this repo | write | the release build only | asked | `internal/packaging/dist.go:39` |

## Not written

Every other file-writing call under `internal/` (not tests) writes inside the Grid home, in a project or task folder that lives in the Grid home, or in a test helper (`internal/golden`, `internal/job/jobtest`, `internal/env/testdata`). SessionStart never creates a project or task record. An agent file in the agents folder with no Grid marker is the person's and is never changed: `SyncAgents` skips it (`internal/settings/agentsync.go:55-60`).

## What SessionStart appends to `CLAUDE_ENV_FILE`

A blank line, then `export GRID_HOME='…'`, `export GRID_PROJECTS='…'`, and `export GRID_TASK='…'` when a task resolves or `unset GRID_TASK` when none does. The earlier bytes of the file are kept.
