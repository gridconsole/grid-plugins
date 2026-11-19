# Publishing the plugin

**One command, run by the owner after a merge to main: `make publish`.** Agents build and test on the VM
only; nothing is pushed by `make dist` or by any lane.

People install the plugin on any machine, with no Go, Node or build step:

```sh
claude plugin marketplace add gridconsole/grid-plugins
claude plugin install grid@grid
```

`claude plugin update` picks up a new release because every publish stamps a higher version:
`0.2.<commit count>`, the number of commits reachable from HEAD of the source repo
(`git rev-list --count HEAD`). The stamp is made in `make dist`, and goes into
`plugins/grid/.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`, the plugin's own
`marketplace.json` and into `grid version` of every binary. `publish.sh` stamps the same version into
the `version` field of the root `.claude-plugin/marketplace.json`.

## The public repo

`github` (https://github.com/gridconsole/grid-plugins.git) holds only snapshot commits on `main`, each
"Publish <version> from <sha>." (the sha is the source commit). The root has exactly:

```text
.claude-plugin/marketplace.json   hand-kept in the public repo; only its version field is stamped
.gitignore  LICENSE  README.md    hand-kept in the public repo
plugins/grid/                     the contents of dist/plugin after make dist
```

`plugins/grid` holds `.claude-plugin/`, `.codex-plugin/`, `.mcp.json`, `hooks/`, `LICENSE`, `README.md`,
`bin/` (the `grid` sh entry, `grid.cmd` and the five binaries), `skills/`, `agents/` and
`agent-templates/`. The hooks and `.mcp.json` run `${CLAUDE_PLUGIN_ROOT}/bin/grid`; the entry picks the
binary for the machine from `uname`, so no Go is needed at install time.

Everything else (`evals/`, `internal/`, `cmd/`, `contract/`, `docs/`, `views/`, `topics/`, `go.mod`,
the `Makefile`) stays in the source repo.

## What `make publish` does

`scripts/publish.sh`:

1. Requires a clean tree on `main` (or on `fix-run-task` at the same sha as `main`) and runs `make dist`.
2. Keeps a clone of github `main` in `.cache/publish` (gitignored) and resets it to `github/main`.
3. Replaces `plugins/grid` with `dist/plugin` and stamps the root `marketplace.json` version.
4. Refuses when the root layout is not exactly `.claude-plugin .gitignore LICENSE README.md plugins`,
   or when a text file holds a home path (`/Users/`, `/home/`), an IP address, `gmail`, `/srv/gridwork`
   or a key or token pattern. Nothing is committed then.
5. When nothing changed it says so and exits 0.
6. Otherwise commits "Publish <version> from <short sha>." with the configured git identity and pushes
   `main` as a plain fast-forward with `http.postBuffer=524288000`. It never forces.

`PUBLISH_PUSH=0 make publish` commits in `.cache/publish` without pushing, to look at the result first.

Each release adds about 85 MB of binaries to the public history. The version comes from the source
repo's commit count, so the public history may be squashed at any time without lowering it. Use a full
clone of the source (a shallow clone counts too few commits and stamps a lower version).

## Check the install

From a machine, or a scratch HOME, that has no Go. `H` is set once and used on both `claude` lines, so
your real `~/.claude` is not touched:

```sh
H=$(mktemp -d)
HOME=$H claude plugin marketplace add gridconsole/grid-plugins
HOME=$H claude plugin install grid@grid
```

Claude Code copies an installed plugin to `<HOME>/.claude/plugins/cache/<marketplace>/<plugin>/<version>/`:

```sh
G="$H/.claude/plugins/cache/grid/grid/0.2.N/bin/grid"      # N: the published version
PATH=/usr/bin:/bin "$G" version                             # prints: grid 0.2.N
```

`PATH=/usr/bin:/bin` leaves out Go, so a pass shows the entry ran the shipped binary. If the path does
not exist (the cache layout is Claude Code's), find the entry with
`find "$H/.claude/plugins" -path '*/bin/grid' -type f`. On an existing install,
`claude plugin update grid@grid` moves it to 0.2.N.

`make dist RELEASE=0.2.N` overrides the stamp for a source archive that has no git history.
