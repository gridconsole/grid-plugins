# C-fix-plugin-mcp-stale-notice

| Row | Status | Proof |
|---|---|---|
| A1 notice on tools/call, version from `$CLAUDE_PLUGIN_ROOT/.claude-plugin/plugin.json`, read at most once per 60 s | HOLDS | internal/mcp/stale.go (diskVersion, withStaleNotice), hooked at internal/mcp/server.go tools/call; `TestStaleNoticeAppearsAndClears`, `TestStaleNoticeReadAtMostOncePerMinute` pass |
| A2 same version / unreadable file / unset root: result unchanged | HOLDS | `TestStaleNoticeSilentCases` passes (one content entry, equal to the unset-root result) |
| A3 fake plugin root: line appears, changing back removes it | HOLDS | `TestStaleNoticeAppearsAndClears` |
| A4 initialize unchanged | HOLDS | `TestInitializeUnchangedByStaleRoot`; initialize code path not edited |
| A5 `go test ./internal/mcp/...` | HOLDS | exit 0, `ok .../internal/mcp` |
| New tests fail on base | HOLDS | with server.go reverted, `TestStaleNoticeAppearsAndClears` FAIL `stale_test.go:58: no notice: [...]` (also ReadAtMostOncePerMinute) |
| No existing test edited | HOLDS | `git status`: only server.go modified, two files new |
| Packages importing mcp | internal/cli (go list); tests: see below |
| `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...`, `GOOS=darwin go build ./...` exit 0 |
| No guard/gate on agents | HOLDS | the notice is an extra text line; nothing blocks |
| Public repo hygiene | HOLDS | no paths, hosts or emails added |

## What changed
- `internal/mcp/stale.go`: reads plugin.json version (cached 60 s per root), appends the notice line to a tools/call result (success and tool-error results).
- `internal/mcp/server.go`: two call sites wrap the result.
- `internal/mcp/stale_test.go`: four tests.

The notice compares with `Server.Version`. The package keeps one process-wide cache, which is fine since `grid mcp` is one process per session.

## NOT VERIFIED on a live screen
- A real restart-needed session in Claude Code showing the line.
