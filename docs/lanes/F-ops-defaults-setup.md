# F-ops-defaults-setup

Setup prints the ops defaults in one line; nothing imports old sessions.

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1. Where setup prints its summary; any path from old Claude sessions to tasks | HOLDS | Summary: `internal/settings/claude.go` `runClaudeSetup`, the non-JSON preview block (the `fmt.Fprintln(o.Stdout, pretty.String())` line, followed by the new line); entry `internal/cli/settings.go:170` `runSetup` → `settings.RunClaudeSetup`. Import path: none found. `grep -rnE "\.claude/projects" --include=*.go internal cmd` (non-test) has no hit. The only code with "session" in a migrate role is `internal/migrate/plan.go:752-772` `pluginSessions`, which copies the plugin's own hook pointer files from `CLAUDE_PLUGIN_DATA/sessions`; it reads no Claude transcript and makes no task. Readers that only show sessions (`internal/session`, `internal/usage`) are unchanged. |
| 2. Summary carries the line | HOLDS | `internal/settings/claude.go` const `opsDefaultsLine`, printed once after the preview. Text: "No telemetry. Crash reports and a daily copy (7 days) stay in your Grid home. Old sessions are not imported." Printed in the non-JSON path only, so `--json` output is unchanged. |
| 3. Go test runs setup into a temp home and finds the line once | HOLDS | `internal/cli/setup_ops_defaults_test.go` `TestSetupSummaryCarriesOpsDefaultsOnce` (write, repeat, remove: exactly 1 each). Fails on base (claude.go stashed): `setup_ops_defaults_test.go:19: [setup --yes]: ops defaults line found 0 times`. Passes on lane: `go test ./internal/cli` exit 0. Also `TestSetupJSONHasNoOpsDefaultsLine`. |
| 4. Go test greps the module for telemetry hosts and fails on a hit | HOLDS | `internal/packaging/no_telemetry_test.go` `TestModuleNamesNoTelemetryHost` plus `TestTelemetryPatternSamples`. Planted `https://o1.ingest.sentry.io/2` in README.md, test failed: `no_telemetry_test.go:44: README.md:2 names a telemetry host or library: ".sentry.io"`; plant reverted with `git checkout README.md`. On the lane head it passes (exit 0). `docs/` is not scanned (lane reports quote past hosts); the env-name words `DISABLE_TELEMETRY` in `internal/settings/claude_user.go` are not hosts and do not match. |
| 5. go test and make lint: 0 fail | HOLDS for changed packages | `go test ./internal/settings ./internal/cli` exit 0; `go test ./internal/packaging/... ./internal/doctor ./internal/guard ./internal/hook ./cmd/...` exit 0 (settings first run failed on the second golden, fixed, rerun ok). `make lint` (`go vet ./...`) exit 0. Whole-module test is the orchestrator's at landing. |
| Importers of the changed package | HOLDS | `go list` imports of `internal/settings`: `cmd/grid`, `internal/cli`, `internal/doctor`, `internal/guard`, `internal/hook` (and `internal/packaging`, `internal/packaging/cmd` import `cli`). All tested above, all ok. |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| gofmt | HOLDS | `gofmt -l internal` prints nothing |
| New tests fail on base, pass on lane | HOLDS | see rows 3 and 4 |
| No existing test edited | HOLDS with one note | No `_test.go` file edited. Two golden data files changed because the output gained the line: `internal/settings/testdata/claude/fresh.json` and `second.txt` (used by `TestClaudeFreshAndSecondRun` and `TestSetupClaudeCLI`). Each gained exactly the one new line. |
| No guard, gate, cap, hold or blocking hook added | HOLDS | Only a printed line and tests. |
| Small commits, one-sentence messages, no attribution | HOLDS | `git log origin/main..HEAD` |
| Public repo: no personal paths, hosts, emails | HOLDS | Added files carry none. |
| No deletions of tests | HOLDS | none deleted |

## What changed

- `internal/settings/claude.go`: unexported const `opsDefaultsLine`, printed once in the non-JSON setup summary (unexported because `TestSettingsStoreChokePoint` rejects exported bindings in this package).
- Goldens `fresh.json`, `second.txt`: the extra line.
- New tests: `internal/cli/setup_ops_defaults_test.go`, `internal/packaging/no_telemetry_test.go`.

## Out of scope / notes

- The line states defaults; crash-report and daily-copy code belong to app lanes (E-ops-crashlog). Nothing here implements them, so the line is true only once those land.
- The `grid setup <project> --answers` path prints "Setup answers recorded for <project>", not the Claude summary; it carries no line.

## NOT VERIFIED on a live screen

- How the line reads in a real terminal beside the prompt `Write these? [y/N]`. Run `grid setup` in a terminal.
