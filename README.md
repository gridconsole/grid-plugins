# Grid plugin

The Grid plugin for Claude Code and Codex: agents, skills, hooks and one Go command line, `grid`, that does every write to task folders, the Feed and the brain.

This repo replaces the earlier JavaScript plugin. The source stays internal; the public plugin tree is `dist/plugin` (docs/release.md).

## Build and test

    make build    # ./dist/grid
    make dist     # five platform binaries (macOS, Linux, Windows), installable dist/plugin stamped 0.2.<commit count>, SHA256SUMS
    make generate # refresh committed plugin metadata and launcher
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

`bin/grid` selects the packaged binary for macOS or Linux on arm64 or amd64, or the Windows binary under Git Bash; `bin/grid.cmd` runs the Windows binary from cmd. To publish, see [release](docs/release.md). In a source checkout the launcher builds `dist/grid` itself when it is missing or older than the sources (needs Go), or uses `dist/plugin/bin/` after `make dist`. It follows shell symlinks and needs no Go or Node runtime. See [packaging](docs/packaging.md) for the generated layout and checksums.

## Commands

`grid help` lists every command group with its verbs; [contract/commands.json](contract/commands.json) is the full table. The groups:

- `grid version`, `grid where`, `grid open`, `grid doctor`: version, location, open the task page, machine health.
- `grid hook`: answer hook events; set and list hook levels.
- `grid task`, `grid plan`, `grid job`, `grid lease`, `grid run`: tasks, plans, Jobs, leases, and the task holder starting one agent per assigned Job in its own worktree.
- `grid feed`, `grid mcp`: the Feed and its stdio MCP server.
- `grid question`, `grid answer`, `grid approve`, `grid deliver`, `grid proof`, `grid history`, `grid land`: questions, answers, gates, outcome, proof, history, landing.
- `grid brain`: the brain notes and topics.
- `grid env`: dev environments.
- `grid person`, `grid routine`, `grid worker`, `grid session`, `grid intent`, `grid usage`, `grid account`: people, routines, workers, sessions, worker requests, usage, accounts.
- `grid agent`, `grid claude`, `grid workflow`, `grid skill`, `grid setup`: agent, Claude, workflow and skill settings, and project setup.
- `grid project`, `grid project-md`, `grid update`, `grid signal`, `grid artifact`, `grid eval`, `grid migrate`: projects, project updates, inbox signals, artifacts, evaluations, home upgrade.
- `grid shot`: crop a screenshot to a box, and compare two screenshots (below).

The `--json` shapes are in [contract/README.md](contract/README.md).

    grid task new "Fix the search label" --size small --json
    grid task show T-01 --json
    grid task list --state queued --json
    grid task new "Sort my notes" --project personal

Tasks use the current folder's project, or Personal when no project applies. `--project` accepts a registered project's name or id, or `personal`. A new task preserves the quoted request exactly in `intent.md`, starts in `state: queued`, and defaults to `size: auto`. `--workflow` stores a workflow name; omitting it leaves selection to the workflow rules. Creation does not start work. `show` needs no id inside a task folder or its descendants. `list` reads one project and optionally filters by an exact state.

`grid mcp` serves the five Feed tools (`feed_post`, `feed_read`, `feed_state`, `feed_types`, `feed_handover`) over stdio MCP. The plugin's `.mcp.json` starts `${CLAUDE_PLUGIN_ROOT}/bin/grid mcp` as the `grid-feed` server, so Claude Code needs nothing more. To run it by hand, build with `make build` and start `./dist/grid mcp` with `GRID_HOME` and the session variables (`GRID_FEED_TASK`, `GRID_JOB`, `GRID_ATTEMPT`, ...) set; it speaks JSON-RPC lines on stdin and stdout. Codex uses `grid feed` in its shell without Feed MCP configuration. See [Feed MCP and Codex](docs/feed-mcp.md) for session context, tools, and examples.

## Screenshots: numbers first, then a crop

A full-page PNG costs far more to read than its answer is worth, and a PNG wider than 1568 px is scaled down before a model sees it, which blurs small text. `grid shot` lets a check compare numbers first and open only the part that differs. It reads and writes PNG files by path and needs no task or project.

    grid shot diff before.png after.png            # text
    grid shot diff before.png after.png --json
    grid shot crop after.png --box 0,640,1440,480  # writes after-crop-0-640-1440-480.png
    grid shot crop after.png --max 1200 --out small.png

- `grid shot diff <png-a> <png-b> [--tile <px>] [--threshold <0-255>] [--json]` prints the boxes where the pictures differ. Text is `differs <p>% in <n> boxes`, then one `x,y,w,h  <p>%` line per box, top to bottom; the box is written as `crop --box` reads it, and the percentage is the share of that box's pixels that differ. The same pictures print `same <w>x<h>`. `--json` has `width_a`, `height_a`, `width_b`, `height_b`, `differs`, `percent` (of the larger picture) and `boxes`, a list (empty when nothing differs) of `x`, `y`, `w`, `h`, `pixels`, `percent`. A channel difference up to `--threshold` (default 8) counts as the same, which hides compression noise; `--threshold 0` compares exactly. Differing pixels in tiles of `--tile` pixels (default 16) that touch join one box. When the sizes differ, the area only one picture has counts as different. It exits 0 whether or not the pictures differ; read `differs`.
- `grid shot crop <png> [--box x,y,w,h] [--max <px>] [--out <png>] [--json]` cuts the box (x, y from the top left, in pixels; default the whole picture) and scales it down, never up, so its long side is at most `--max` (default 1568). Without `--out` the file is `<png>-crop-<x>-<y>-<w>-<h>.png` beside the input. A box outside the picture is refused and names the picture size. `--json` has `file`, `box`, `width`, `height`, `source_width`, `source_height`, `scale`.

The `browser` and `visual-design-review` skills use these in order: element boxes and computed sizes, then `grid shot diff`, then `grid shot crop` of a box that differs.

## Doctor

`grid doctor` checks the Grid home, writability, format version, git, the grid launcher on PATH, plugin hooks, and the setup allow block in Claude settings. The allow block check requires every allow rule written by setup: `Bash(grid *)`, `Read(<anchor>/**)`, and `Edit(<anchor>/**)`, using the same home-relative anchor. It reads the setup destination (`CLAUDE_CONFIG_DIR/settings.json`, or `~/.claude/settings.json` by default) without changing it and follows settings symlinks to regular, readable files just as setup does. It reports missing rules or an unreadable, malformed, missing, dangling, or nonregular settings file. When the Grid home is unavailable, the allow block check reports `skipped: Grid home is unavailable` without adding a failure.

Each failing check includes a fix hint. For missing rules or a missing settings file, the hint is `Run grid setup`. For invalid JSON, unreadable files, dangling symlinks, or nonregular files, it is `Fix or move the Claude settings file, then run grid setup.` These hints contain no paths or settings values. `grid setup --remove` takes the block out. The block also holds Edit deny rules for `settings.json`, `.settings.lock` and `workflows.md` in the Grid home, one folder below it and in a project's folder, and for the `.host` folder, so Claude Code keeps agents off the files the host guard protects even when that guard only warns. `grid setup --sandbox` also writes `"sandbox": {"enabled": true}` to switch on Claude Code's own sandbox for this user, and `grid setup --remove` takes out only the entries setup wrote. `grid doctor --json` includes the check in `checks`. `grid doctor --bundle` writes a local support archive with the settings path redacted from diagnostics and logs.

## Routines

`grid routine add` creates a home routine; `--project <name>` selects a registered
project. Routine front matter in the brain's `routines` config topic controls its
limits. Omitted keys inherit front matter policy from `workflows.md`: entry
keys take precedence over `projects/<name>/workflows.md`, then the workspace
`workflows.md` in the Grid home, then the built-in defaults below. Home routines
use workspace policy directly. The owner may change each value:

| Key | Default | Allowed values |
| --- | --- | --- |
| `routine_timeout` | `10m` | Go duration from `1m` to `2h`, inclusive |
| `routine_max_turns` | `10` | Integer from 1 to 100, inclusive |
| `routine_budget` | `1` | Decimal number greater than 0 and at most 50 USD |

Set these fields when adding a routine with `--routine-timeout`,
`--routine-max-turns`, and `--routine-budget`, or edit the same-named front matter
keys. Numeric fields must be YAML numbers, not quoted strings. For example:

```sh
grid routine add daily --every 24h --run signals \
  --routine-timeout 20m --routine-max-turns 3 --routine-budget 0.25
```

```yaml
routine_timeout: 20m
routine_max_turns: 3
routine_budget: 0.25
```

A skill routine that reads a mail, chat or calendar source lists that source's
tools in its front matter, or with a repeated `--tool` flag on `grid routine add`:

```yaml
tools:
  - mcp__mail__search_messages
  - mcp__mail__read_message
```

Write each name in full, as the source's tool list gives it. The routine adds the
names to its allow list as they are, after `Read`, `Glob`, `Grep`, `grid brain`
and `grid signal`, and names them in its prompt; it adds no other tool. List the
tools that read and no tool that sends, replies or deletes: the `signals` skill
only reads. A name may hold no comma, space or control character. A routine
without `tools` runs with the allow list it had before. The timer guard below
also records `tools`, so adding or changing one makes timer runs refuse until
`grid routine schedule` approves them again.

`grid routine run daily` resolves entry and workflow policy on each run. Skill
routines pass the budget and turn limits to the agent; commands starting with
`grid` use only the timeout. Invalid policy refuses add or run with exit 2 and a
message naming the key, without executing work or changing `last_run`. Invalid
inherited values name the workspace or project level, never a routine, supplied
value, or filesystem path; each level is validated even if an override masks it.
Task-level `workflow.md` never supplies routine limits. Root and project
`workflow.md` files are also ignored; `grid routine run` prints one warning per
old file containing `routine_*` front matter keys, naming its relative location
and the corresponding `workflows.md` destination.
`grid routine list --json` and `grid routine run daily --json` include a `limits`
object with all three effective values, including defaults. The entry lock,
interval guard for timer invocations, and existing caller gates still apply.

The timer guard records `run`, `every`, and the resolved timeout, maximum turns,
and budget whenever an entry supplies any limit key or workflow policy sets a
non-default limit. Entries without limit keys and with default workflow policy
retain their original schedule hash. Changing guarded values makes timer runs
refuse until `grid routine schedule` approves them again; manual runs remain
explicit requests. Limits use a fixed order (timeout, turns, budget) and canonical
numeric values, so equivalent spellings such as `10m` and `600s` agree. Run state,
notes, and unrelated workflow fields do not affect the guard.

## Layout

    cmd/grid/        entry point
    internal/        packages: cli, task, feed, mcp, where, doctor, hook, project, gitx, gridhome, version
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
