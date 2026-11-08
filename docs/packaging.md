# Packaging

Run `make dist` with the Go version in go.mod to create this offline layout:

```text
dist/
  SHA256SUMS
  plugin/
    .claude-plugin/plugin.json
    .claude-plugin/marketplace.json
    .codex-plugin/plugin.json
    hooks/hooks.json
    bin/grid
    bin/grid-darwin-arm64
    bin/grid-darwin-amd64
    bin/grid-linux-amd64
    bin/grid-linux-arm64
    skills/  agents/  LICENSE
```

Install `dist/plugin` as the plugin directory; keep its relative paths intact.
The local Claude marketplace entry points at that same directory (`./`).
Both manifests share skills and hooks; Codex uses its compatibility manifest.
Codex supplies `CLAUDE_PLUGIN_ROOT` for compatibility with the shared hook command
([OpenAI plugin packaging](https://developers.openai.com/plugins/build/plugins)).
`bin/grid` follows shell symlinks, maps uname to a shipped binary, and uses exec
so arguments, stdin, output and exit status pass through unchanged.
Unsupported OS/architecture and missing executables produce an error on stderr.
In a source checkout (go.mod beside it) it uses a `dist/plugin/bin/` binary or `dist/grid` when
no `.go` file is newer, and otherwise builds `dist/grid` first; a hook stays silent and exits 0 when
that build is impossible. Releases never build and never touch the network.
`make build` creates only the native development CLI at `dist/grid`.

The release version is `git describe --tags --always --dirty`, stamped into
`internal/version.Version` through `-ldflags` and into both release manifests.
`make dist VERSION=<version>` is available for source archives without Git.
Builds disable cgo and VCS metadata, use `-trimpath` and `-buildid=`, and pin
architecture baselines. Identical source, version and Go toolchain produce
identical bytes; use a clean tagged checkout for a release.
Windows/amd64 is compiled and discarded as a build-only check, not shipped.

SHA256SUMS covers every regular file under plugin, sorted by relative path.
Verify from dist with `sha256sum -c SHA256SUMS` on Linux or
`shasum -a 256 -c SHA256SUMS` on macOS. Only successful builds write the sums.

`internal/hook/events.go` owns dispatch and hook registration metadata.
`internal/packaging` generates both manifests, marketplace, hooks and launcher.
Run `make generate` after edits; committed development manifests use the default
version in `internal/version`, while dist uses the Git version without editing them.
`make test` checks generated files, hook coverage, skill command names, launcher
behavior and checksum output without downloads; `make lint` checks Go style.
Skills and agents are copied unchanged, with executable script modes preserved.

This bundle needs neither Node nor Go at runtime. Download/cache installation,
app-provided binaries, setup links and Codex agent TOML rendering remain outside
this packaging slice. No remote release is published by `make dist`.
