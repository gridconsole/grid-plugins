# C-jz-mcp-stale-root-from-env

The MCP server's stale-plugin notice reads CLAUDE_PLUGIN_ROOT through the command's Env instead of the process.

Base: the clone started on `dc301fce` (a descendant of the brief's `35af7d3c`; `git diff origin/main..HEAD` shows only this lane's four files).

## Proof table

| Row | Result | Command / place |
|---|---|---|
| 1. Getenv's root (1.2.3) wins over a process root (9.9.9); notice is exactly `Grid plugin on disk is 1.2.3, this session runs 1.0.0; restart the session to load it.` | HOLDS | `go test -count=1 -v -run 'TestJzMc' ./internal/mcp/` exit 0: `--- PASS: TestJzMcGetenvRootBeatsTheProcessRoot`. Test: internal/mcp/jz_stale_env_test.go (TestJzMcGetenvRootBeatsTheProcessRoot) |
| 1 on base | fails | Base source (test file only): `unknown field Getenv in struct literal of type Server` at jz_stale_env_test.go:62, :80, :97, :98, :115, :116, `FAIL ... [build failed]`. With only the `Getenv` field added and stale.go untouched: `want one notice "Grid plugin on disk is 1.2.3, this session runs 1.0.0; restart the session to load it." last, got 2 texts with stale lines ["Grid plugin on disk is 9.9.9, this session runs 1.0.0; restart the session to load it."]` (also `same version: notice added: [... 9.9.9 ...]`, and `worker A wanted ...1.2.3..., got ...9.9.9...`) |
| 2. Getenv answering a root at the running version adds no notice; Getenv answering "" adds none | HOLDS | same run, `--- PASS: TestJzMcGetenvSameVersionOrEmptyAddsNoNotice` (process root stays 9.9.9 so a leak would show) |
| 3. Getenv nil reads the process root, as today | HOLDS | same run, `--- PASS: TestJzMcNilGetenvReadsTheProcessRoot` (notice says 9.9.9). It also passes on the unchanged logic, which is the point: no in-process answer changes |
| 4a. `go test -count=1 ./internal/mcp/` exits 0 | HOLDS | `ok  github.com/gridconsole/grid-plugins/internal/mcp 0.583s`, exit 0 |
| 4b. `go test -count=1 -run 'MCP\|Mcp' ./internal/cli/` exits 0 | HOLDS | `ok  github.com/gridconsole/grid-plugins/internal/cli 0.415s`, exit 0 |
| 5a. `make lint` exits 0 | HOLDS | `go vet ./...`, exit 0 |
| 5b. `make cross` exits 0 | NOT DONE | Not finished inside the budget. Seen: `cross: darwin/arm64 vet` and `cross: darwin/arm64 test compile` passed for every package (all `ok` or `no test files`, `internal/cli ok`, `lib ok`); `cross: windows/amd64 vet` started and had printed nothing more when I stopped. No exit code, so no HOLDS. The orchestrator runs `make cross` at landing; re-run it first |
| Cache keyed by root, not shared, no lock or limit added | HOLDS | stale.go keeps `stale.root` and the one mutex; `TestJzMcServersWithDifferentRootsDoNotShareAnAnswer` (two servers plus a nil one, interleaved in one 60 s window, two rounds) and `TestJzMcConcurrentServersKeepTheirOwnRoot` (12 workers, 100 calls each, start barrier, no sleep) PASS; `go test -count=1 -race -run 'TestJzMc' ./internal/mcp/` exit 0 |
| Importers of internal/mcp tested | PARTLY DONE | `go list` (Deps, TestImports, XTestImports) names: internal/cli, cmd/grid (no test files), lib, internal/mcp itself. `go test -count=1 ./internal/mcp/ ./internal/cli/ ./cmd/grid/ ./lib/` printed `ok  .../internal/mcp 29.113s` and then did not finish for internal/cli, cmd/grid and lib while I wrote this. Run on their own and passing: `go test -count=1 -run 'MCP\|Mcp' ./internal/cli/` (ok), `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` (ok). The full `./internal/cli/` run, `./cmd/grid/` and `./lib/` are NOT DONE: no exit code seen |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` -> `ok ... 3.740s`; `go test -count=1 ./internal/job/lint/` -> `ok ... 6.332s`. The app walk tests (webui, FvArgv, wired mounts, slices Imports) are not in this repo. |
| No test edited or deleted | HOLDS | `git diff --stat origin/main..HEAD`: internal/cli/mcp.go +1, internal/mcp/jz_stale_env_test.go +154 (new), internal/mcp/server.go +4/-1, internal/mcp/stale.go +10/-6; stale_test.go untouched; its helpers `withClock`, `fakeRoot`, `callTypes` are reused |
| New tests in the new file; helpers `jzMc*`, tests `TestJzMc*` | HOLDS | internal/mcp/jz_stale_env_test.go. No `sleep`, no timeout, no elapsed time as proof |
| No guard, gate, cap, hold, hook | HOLDS | None added; no lock or limit beyond the existing mutex |
| RealPlugin tests | not applicable | No row names one; none run |
| Commits | HOLDS | three one-sentence commits, no attribution lines |

## What changed

- `internal/mcp/server.go`: `Server` gains `Getenv func(string) string`, "nil reads the process".
- `internal/mcp/stale.go`: `diskVersion(getenv)` reads `CLAUDE_PLUGIN_ROOT` through it; `withStaleNotice` passes `s.Getenv`, or `os.Getenv` when nil. The cache is unchanged and still keyed by root.
- `internal/cli/mcp.go:16`: `Getenv: env.getenv`, the same read every other cli command uses.
- `internal/mcp/jz_stale_env_test.go`: five tests (rows 1-3, different roots, concurrent roots).
- No in-process answer changes: `mcp` stays binary-only in process; a nil Getenv reads the process as before.

## NOT VERIFIED on a live screen

Nothing here has a screen. Not run: the real `claude` plugin host, and a real `grid mcp` over stdio (no claude or background server allowed).

## Notes for the reviewer

- The package doc of internal/mcp (server.go:2) still says "no process or environment access"; stale.go already used `os.ReadFile` and `os.Getenv` before this lane. Left as is, it is not in the brief.
- `diskVersion` still holds the one mutex across the file read, as before; the single cache slot means servers with different roots re-read on each switch. Correct, not new.

## NOT DONE rows

- 5b `make cross` (windows/amd64 half did not finish, no exit code).
- Full `go test -count=1 ./internal/cli/ ./cmd/grid/ ./lib/` (the importers of internal/mcp); only the `MCP|Mcp` and walk-test slices of internal/cli were seen passing.

Both were started in the background with the lane's code committed and ran with no visible progress while I polled; I did not report them as passing. The diff is one added struct field, one changed call and one added struct literal field, so a failure there is unlikely, but that is a guess, not proof.
