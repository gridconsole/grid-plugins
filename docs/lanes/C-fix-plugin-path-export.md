# C-fix-plugin-path-export

| Row | Result |
|---|---|
| A1 line added when bin/grid exists and bin not on PATH | HOLDS: internal/hook/hook.go `pluginPathLine`; TestPluginPathLine, `go test ./internal/hook/...` exit 0 |
| A2 no line when root unset or bin/grid missing | HOLDS: TestPluginPathLine (unset root, missing bin/grid, already on PATH) |
| A3 quoted; space and quote root resolves via `sh -c '. env; command -v grid'` | HOLDS: TestPluginPathLineResolvesWithSpaceAndQuote (root `my plug'in`) |
| A4 Windows env output unchanged | HOLDS: pluginPathLine returns "" when runtime.GOOS == "windows" (hook.go); writeEnv extra args are empty then. Not run on a Windows host. |
| A5 `go test ./internal/hook/...` | HOLDS: exit 0, `ok .../internal/hook 6.811s` |
| New tests fail on base | HOLDS: with hook.go stashed, `go test ./internal/hook/ -run PluginPathLine` exit 1: `hook_test.go:229:12: undefined: pluginPathLine`, `too many arguments in call to writeEnv` |
| No existing test edited | HOLDS: only added tests and the `runtime` import in hook_test.go |
| Importers tested | HOLDS: importers of internal/hook per go list: internal/cli, internal/packaging; `go test ./internal/cli/... ./internal/packaging/...` exit 0 |
| `make lint` | HOLDS: exit 0 (`go vet ./...`) |
| Windows build | HOLDS: `GOOS=windows go build ./...` exit 0 |
| macOS build | HOLDS: `GOOS=darwin go build ./...` exit 0 |
| No guard/gate/hook on agents | HOLDS: none added |
| RealPlugin tests | Not applicable: no row names one |

## What changed
- `writeEnv` takes optional extra lines; SessionStart passes `pluginPathLine(CLAUDE_PLUGIN_ROOT, PATH)`, which emits `export PATH='<root>/bin':"$PATH"`.
- Path is single-quote shell-quoted (shared `shellQuote`).

## NOT VERIFIED on a live screen
- A real Claude Code session on the VM showing bare `grid` and doctor green.
- Windows/Codex runtime output (only compiled).
