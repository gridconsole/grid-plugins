# L31 packaging handoff

Built `make dist`: four grid binaries (darwin/arm64, darwin/amd64,
linux/amd64, linux/arm64), a Windows/amd64 compile-only check, an installable
`dist/plugin` directory and `dist/SHA256SUMS`. The version comes from
`git describe --tags --always --dirty` through internal/version linker flags.
Builds disable cgo and VCS metadata, trim paths, remove build IDs and pin CPU
baselines. The shell launcher selects a bundled binary, follows shell links,
forwards arguments/stdin/status, and explains missing or unsupported platforms.

One hook registry now drives dispatch, hook.Handles and generated hooks.
The generator owns both plugin manifests, the local Claude marketplace entry,
hooks config and launcher. Tests reject stale generated files, unhandled hook
events and skill command names absent from cli.Implemented. Skill files and
agent files are copied unchanged; no skill prose was edited. Packaging is
documented in docs/packaging.md (under 60 lines).

Run `make test` and `make lint`. New tests use local fixtures and fake platform
binaries; they make no network requests. Tests ran with the default TMPDIR.
Both passed on Linux, as did `GOOS=windows go build ./...` and
`GOOS=darwin go build ./...`. `make dist` built every requested target.
Two consecutive distributions had identical SHA256SUMS, every checksum verified,
the native launcher reported the stamped version, and its Go build ID was empty.

Intended differences from the old plugin (references name the read-only spec):

- Go replaces the Node hook scripts and separate grid-feed/grid-brain wrappers;
  there is one launcher and one process per event. References:
  old-plugin-map.md:5, old-plugin-map.md:53, old-plugin-map.md:54,
  plugin.md:233, plugin.md:235, plugin-design.md:123, plugin-design.md:469.
- Only currently handled events are registered: SessionStart, PreToolUse and
  PermissionRequest. The old plugin's broader hook list is not advertised as
  implemented. The dispatcher registry and tests keep the config synchronized.
  References: old-plugin-map.md:9, old-plugin-map.md:22, plugin.md:226.
- Two generated manifests share the same skills and hooks; a generated local
  Claude marketplace entry points at the package root. The old manifest was
  Claude-only. References: old-plugin/.claude-plugin/plugin.json:1,
  plugin-design.md:473. Codex's compatibility manifest and shared environment
  variable follow https://developers.openai.com/plugins/build/plugins.
- Four native release builds and SHA256SUMS replace runtime JavaScript;
  Windows is a compile-only check with no released binary or launcher support.
  References: plugin-design.md:443, migration.md:70.
- This slice bundles binaries for offline use instead of downloading into the
  Grid home or selecting the app's copy. References: plugin-design.md:123,
  plugin-design.md:469, plugin-design.md:484. No download or install is attempted.

Not completed: first-use downloads, pinned remote checksums, app-cache selection,
setup links and Codex agent TOML rendering. The checkout has no release endpoint
or implemented setup command, and these installation behaviors exceed this
bundle/build slice (plugin-design.md:123, plugin-design.md:473).
There are no skill Markdown files in this checkout; the command scanner is
covered with synthetic text and will check all future skill Markdown files.
No native macOS runner or interactive Claude/Codex install was available;
macOS was cross-compiled and launcher behavior was exercised with fake uname.
No release was published, and Windows runtime behavior is not claimed.
