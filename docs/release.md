# Publishing the plugin

**Owner runs this; not done by agents.** Agents build and test on the VM only. Nothing here is pushed,
published or deployed by `make dist` or by any lane.

People install the plugin on any machine, with no Go, Node or build step:

```sh
claude plugin marketplace add gridconsole/grid-plugins
claude plugin install grid@grid
```

`claude plugin update` picks up a new release because every publish stamps a higher version:
`0.1.<commit count>`, the number of commits reachable from HEAD of the source repo
(`git rev-list --count HEAD`). The stamp is made in one place, `make dist`, and goes into
`.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`, `.claude-plugin/marketplace.json`
and into `grid version` of every binary.

## What goes into the public repo

The public repo's root is the plugin root: exactly the contents of `dist/plugin`, nothing else.

```text
.claude-plugin/plugin.json  .claude-plugin/marketplace.json  .codex-plugin/plugin.json
.mcp.json  hooks/hooks.json  LICENSE
bin/grid                      sh entry: macOS, Linux, and Windows under Git Bash
bin/grid.cmd                  Windows cmd entry
bin/grid-darwin-arm64  bin/grid-darwin-amd64  bin/grid-linux-amd64  bin/grid-linux-arm64
bin/grid-windows-amd64.exe
skills/  agents/  agent-templates/
```

The hooks and `.mcp.json` run `${CLAUDE_PLUGIN_ROOT}/bin/grid`; the entry picks the binary for the
machine from `uname` (or runs the `.exe` on Windows), so no Go is needed at install time.

## What stays out

Everything that is not in `dist/plugin` stays in the internal source repo, because `make dist`
copies only `skills/`, `agents/`, `agent-templates/` and `LICENSE` beside the generated files and
the binaries. In particular these never reach the public repo:

- `evals/` and `corpus/`, `golden/`, `recordings/`, `fixtures/`, `testdata/`: internal. The evals
  hold task material that is not for publication (see D-privacy-audit).
- `internal/`, `cmd/`, `contract/`, `docs/` (including `docs/lanes/`), `views/`, `topics/`,
  `record.sh`, `go.mod`, `go.sum`, `Makefile`: the source and its working papers.
- `dist/SHA256SUMS` stays beside `dist/plugin`, not inside it.

Step 3 below checks the tree before it is committed.

## Steps

Run in a full clone of the source repo (a shallow clone counts too few commits and stamps a version
lower than the last release, so `claude plugin update` would not move), on the commit to publish,
with go.mod's Go version installed.

1. Check the source is clean and green:

   ```sh
   git status --short                # prints nothing
   git rev-list --count HEAD         # N: this release will be 0.1.N
   make test lint
   ```

2. Build the release. This writes `dist/plugin` and `dist/SHA256SUMS`, and compiles all five
   binaries (darwin arm64 and amd64, linux amd64 and arm64, windows amd64):

   ```sh
   make dist
   (cd dist && sha256sum -c SHA256SUMS)       # macOS: shasum -a 256 -c SHA256SUMS
   grep '"version"' dist/plugin/.claude-plugin/plugin.json   # "0.1.N"
   ```

3. Check what will be public. The listing must match the tree above, and nothing may name a
   person, a host or a local path:

   ```sh
   ls -A dist/plugin dist/plugin/bin
   grep -rIn '/home/\|/Users/\|/srv/\|@gmail' dist/plugin     # prints nothing
   ```

4. Put `dist/plugin` at the root of the public repo and push (this is the publish). This replaces
   the whole tree of the public repo's default branch, so it assumes that repo holds only releases;
   if it also holds the source, decide that first (the install command above reads the default branch):

   ```sh
   N=$(git rev-list --count HEAD)
   PUB=$(mktemp -d)
   git clone git@github.com:gridconsole/grid-plugins.git "$PUB"
   find "$PUB" -mindepth 1 -maxdepth 1 ! -name .git -exec rm -rf {} +
   cp -R dist/plugin/. "$PUB"/
   git -C "$PUB" add -A
   git -C "$PUB" commit -m "Release 0.1.$N"
   git -C "$PUB" push origin HEAD
   ```

   `cp -R` keeps the executable bits, and git stores them. Each release adds about 85 MB of binaries
   to the public history. The version comes from the source repo's commit count, not from the public
   repo's, so the public history may be squashed to one commit at any time without lowering it.

5. Check the install from a machine, or a scratch HOME, that has no Go:

   ```sh
   HOME=$(mktemp -d) claude plugin marketplace add gridconsole/grid-plugins
   HOME=$HOME claude plugin install grid@grid
   ```

   Then run the installed `bin/grid version`; it prints `grid 0.1.N`. On an existing install,
   `claude plugin update grid@grid` moves it to 0.1.N.

`make dist RELEASE=0.1.N` overrides the stamp for a source archive that has no git history.
