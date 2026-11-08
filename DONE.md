# Lane L10

Implemented with the Go standard library. The module and shared resolver packages are unchanged. Changes to `internal/cli/cli.go` only register replacement handlers in the existing command table.

## Commands

- `grid version --check --manifest-url https://example.com/grid/releases/latest.json`
- `GRID_RELEASE_MANIFEST_URL=https://example.com/grid/releases/latest.json grid version --check --json`
- `grid version --check --timeout 500ms --app-version 0.0.1 --session-id example-session`
- `grid doctor --hooks-file ./hooks/hooks.json` (also supports `--json`).
- `printf '%s' '{"source":"startup"}' | grid hook sessionstart`

The release manifest is a JSON object such as `{"version":"1.2.3","contract":"1","app_min":"0.1.0"}`. Only `version` is required. Versions use semantic version precedence, including prereleases, optional leading `v`, and ignored build metadata. Unknown current build versions report that comparison is unavailable. No release endpoint is hard-coded.

The timeout defaults to two seconds and covers the HTTP request, redirects and body. Responses are limited to 64 KiB. Successful results are atomically cached for 24 hours in `GRID_HOME/cache/version-<URL hash>.json`. Separate manifest URLs have separate caches. Failed refreshes retain a previous result and label it stale; without a previous result they report unavailable. Failed checks, malformed manifests and cache write failures are advisory and exit zero. Failure results are not cached, so the next call can recover immediately. Invalid CLI arguments return usage status 2.

`GRID_APP_VERSION` and `GRID_SESSION_ID` supply the corresponding version flags. The app warning compares the installed app against this binary's `app_min`, not the minimum for a newer release. With a session id and writable home, an exclusive marker under `cache/version-sessions/` suppresses repeat warnings. No app version means no app warning. Without a session id or usable marker storage, the warning is printed on every check that finds incompatibility. Plain `grid version` and `grid version --json` keep their original output.

Doctor checks the home directory, actual create/remove writability, format version, git, executable lookup for `grid`, and plugin hook JSON structure. Every check has a JSON `fix` field; plain output prints hints on failures. Hook discovery uses `--hooks-file`, then `CLAUDE_PLUGIN_ROOT/hooks/hooks.json`, then `./hooks/hooks.json`. Validation requires a SessionStart group, valid matcher expressions, nonempty command hooks and positive optional timeouts; it does not execute hook commands. An unhealthy report exits 1.

SessionStart appends shell-quoted `GRID_HOME`, derived `GRID_PROJECTS`, and the task reference returned by `grid where` to `CLAUDE_ENV_FILE`. It unsets `GRID_TASK` when no task resolves. Existing file content is retained, including content without a trailing newline. `GRID_TASK` and `GRID_PROJECT` provide explicit resolution inputs. No env-file variable means context-only output. Input cwd wins over process cwd. Empty stdin uses process cwd; malformed, oversized or unreadable input emits nothing. Resolution errors, incompatible existing format versions and env-file write errors also emit nothing and exit zero. Unknown events and invocation errors are silent. SessionStart does not create project/task records or renew leases.

Context includes the Grid home, project/task resolution, task ownership, Grid agent roles, shared knowledge and the terminal approval route. All start sources, including compact, receive it in `hookSpecificOutput.additionalContext`. Unicode-safe truncation keeps context below 10,000 characters.

## Tests

Normal environment:

```sh
make test
make lint
```

Focused tests:

```sh
go test ./internal/version ./internal/doctor ./internal/hook
go test ./internal/cli -run 'TestVersion|TestDoctor|TestHook'
```

Tests use `httptest` servers and `t.TempDir()`. Coverage includes release ordering, expiry, offline fallback, URL isolation, corruption, timeout, invalid responses, unavailable cache storage, once-per-session app warnings, malformed hooks, missing executables, format failures, write failures, shell quoting, all SessionStart sources, explicit task resolution, no project creation, silent errors and the context ceiling. Golden files cover plain/JSON doctor output and SessionStart context.

Validation in this runner: the focused package and CLI tests pass, and `make lint` passes with `GOCACHE="$PWD/.cache/go-build"`. Full `make test` currently encounters an unrelated `/tmp/.git` ancestor in the existing resolver tests. A clean temporary test root is required; approval to use one outside the lane is pending. The shared resolver code was not changed to mask this environment issue.

## Intended differences from the old plugin

References use the supplied read-only documentation and old plugin paths.

- The Go dispatcher replaces separate Node SessionStart scripts. Source: `docs/plugin-design.md:90`, `docs/plugin-design.md:123`; old wiring: `old-plugin-map.md:8`.
- Resolution uses the existing `grid where` implementation and one Grid home, independent of the desktop app's selected workspace. Context appears outside an app workspace too. Source: `docs/plugin-design.md:127`, `docs/plugin-design.md:137`, `docs/plugin-design.md:151`; old behavior: `old-plugin/hooks/grid-env.mjs:13`, `old-plugin/hooks/grid-env.mjs:15`.
- Preexisting Grid env variables no longer suppress context. The resolved Grid home is exported, projects are derived from that home, and the resolved task reference is exported (or unset). Explicit task/project inputs take precedence. Source: `docs/plugin-design.md:103`, `docs/plugin-design.md:129`, `docs/plugin-design.md:137`; old early return and exports: `old-plugin/hooks/grid-env.mjs:13`, `old-plugin/hooks/grid-env.mjs:21`.
- Context is factual, bounded, and repeated on compact as well as startup, resume, clear and fork. It states that task files are written by grid and names the person-only terminal approval path. Source: `docs/plugin-design.md:90`, `docs/plugin-design.md:100`, `docs/plugin-design.md:114`; old imperative context: `old-plugin/hooks/grid-env.mjs:24`, `old-plugin/hooks/agent-routing.mjs:30`.
- Agent routing uses a compact role list from the new design instead of scanning old agent descriptions. It omits grid-brief-writer, explains that grid job renders briefs, and includes grid-library. Source: `docs/plugin-design.md:235`, `docs/plugin-design.md:245`, `docs/plugin-design.md:246`; old list generation: `old-plugin/hooks/agent-routing.mjs:21`.
- Version checking is an explicit advisory command with release caching and the installed app's minimum-version banner. It does not infer Claude Code compatibility from CLAUDE_PLUGIN_DATA. Source: `docs/plugin-design.md:119`, `docs/plugin-design.md:485`; old feature probe: `old-plugin/hooks/session-check.mjs:10`, `old-plugin/hooks/session-check.mjs:13`.
- Doctor checks the single home's VERSION and provides remediation for installation failures instead of relying on SessionStart's runtime warning. Source: `docs/plugin-design.md:119`, `docs/plugin-design.md:145`; old runtime warning: `old-plugin-map.md:30`.
- Invalid event JSON now produces no output rather than falling back to cwd, as required by this lane's silent-on-error rule. Missing stdin still falls back to cwd. Old fallback: `old-plugin/hooks/grid-env.mjs:12`; the existing Go skeleton also tolerated malformed JSON.

## Not implemented in this lane

- Topic-index rendering, stale load-folder refresh, compact Job/open-decision rendering, lineage attachment, usage logging and listing open tasks with free leases need the brain, Job, session and lease services. Their CLI entries are still skeletons here; no substitute schema or guessed state was written. Required by `docs/plugin-design.md:90`, `docs/plugin-design.md:147`, `docs/plugin-design.md:289`; old side effects mapped in `old-plugin-map.md:33` and `old-plugin-map.md:34`.
- Global format gating and migration remain the shared CLI/migration lane's responsibility. Doctor diagnoses versions, and SessionStart silently skips incompatible existing formats. There is no global dispatcher edit or automatic migration. Source: `docs/plugin-design.md:145`.
- `doctor --bundle` remains explicitly unimplemented. Source: `docs/plugin-design.md:119`.
- Release downloads, updates, app-version discovery and a production release URL are outside this brief. The manifest and env/flag contract above are explicit local choices because the supplied design does not define their wire format. The check never installs anything. Source: `docs/plugin-design.md:123`, `docs/plugin-design.md:484`, `docs/plugin-design.md:485`.
- No live Claude Code or Codex host was available for an integration session. Tests verify the documented JSON/env boundary; Codex persistence remains unverified in the design itself (`docs/plugin-design.md:104`).
