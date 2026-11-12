# F-0445-sessionenv-replace: SessionStart replaces its block in the session env file

Head: see the report line; branch lane-F-0445-sessionenv-replace, from main.

## Proof table

| Row | Result |
| --- | --- |
| 1. Five SessionStart runs on one env file give one GRID_HOME line and one PATH line with the plugin bin dir | HOLDS: `TestSessionEnvFiveRunsKeepOneBlock` (internal/hook/sessionenv_replace_test.go), `go test -count=1 ./internal/hook/` exit 0. On base (old hook.go, marker consts added only so the test compiles): `sessionenv_replace_test.go:65: GRID_HOME lines: 5, want 1` and `:71: export PATH lines: 5, want 1` |
| 2. GRID_TASK changed between two runs leaves the new value only | HOLDS: `TestSessionEnvTaskChangeKeepsNewValueOnly`. On base: `sessionenv_replace_test.go:99: old task value is still in the file:` and `:102: GRID_TASK lines: 2, want 1` |
| 3. A foreign line before and after the block survives | HOLDS: `TestSessionEnvForeignLinesBeforeAndAfterSurvive` (also a last line with no trailing newline stays whole). On base: `:123: line after the block is gone or moved:` |
| Extra: start marker with no end marker is an ordinary line | HOLDS: `TestSessionEnvUnclosedStartMarkerIsAForeignLine`; base fails `:151: GRID_HOME lines: 2, want 1` |
| 4. `go test -count=1 ./internal/hook/` | HOLDS: `ok .../internal/hook 9.998s`, exit 0 |
| 4. `make lint` | HOLDS: `go vet ./...`, exit 0 |
| 4. `make cross` | HOLDS: darwin/arm64 and windows/amd64 vet and test-compile, exit 0 |
| Importers of internal/hook (`go list`: internal/cli, internal/packaging) | HOLDS: `go test -count=1 ./internal/cli/ ./internal/packaging/` both `ok` |
| Golden corpus | HOLDS: `go test -count=1 ./internal/golden/` ok after re-recording 5 cases (below) |
| Walk tests: `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` | HOLDS: `ok .../internal/cli 0.260s` |
| Walk tests: `go test -count=1 ./internal/job/lint/` | HOLDS: `ok .../internal/job/lint 0.179s` |
| App-only walk tests (webui, plugincli, grid-server, slices) | n/a: plugin repo |
| New tests fail on base, pass on lane | HOLDS: lines above; run with hook.go stashed and a temporary consts file (deleted, `git status` clean of it) |
| No guard, gate, cap or hold added | HOLDS: no hook decision changed; only the env file write |
| `*RealPlugin*` tests | n/a: none touched |
| UI rows | n/a: no UI |

## What changed

- `internal/hook/hook.go`: `writeEnv` writes the Grid lines (GRID_HOME, GRID_PROJECTS, GRID_TASK or `unset GRID_TASK`, and the PATH line) between `# >>> grid session env >>>` and `# <<< grid session env <<<`. If the file already holds a marked block it is replaced in place; a later duplicate block is dropped; every other line is kept. The result goes to a temp file in the same folder (mode of the old file, else 0600) and is renamed over the file; a symlinked path is resolved first. A start marker with no end marker after it is an ordinary line. First write keeps the old layout: a blank line, then the block.
- `internal/hook/sessionenv_replace_test.go`: four new tests.
- `internal/hook/ex_outside_home_test.go`: `TestExHomeEnvFileGetsItsLinesAtEveryLevel` asserted the exact appended bytes; its expected string now includes the two marker lines (the brief's format change forces this; the assertion on the earlier bytes is unchanged).
- Goldens re-recorded with the built-in `-xs-rerecord` flag: `golden/agent-routing/{empty-object,session}`, `golden/grid-env/{discovered,explicit,no-config}` (`go-expected/files.diff`: +2 marker lines each).

## Notes for the backlog

- `docs/privacy.md` (rows at line 8 and 23-25) still says SessionStart appends a blank line and the export lines; it is outside this brief's Touches. It should say the lines sit in a marked block that later runs replace.
- Copies written by older versions (outside a marked block) are foreign lines and stay; the new block comes after them, so its values win when the file is sourced.
- `PATH` check in `pluginPathLine` is unchanged: when the hook's own PATH already holds the bin dir, no PATH line is written (and a replaced block then has none).

## NOT VERIFIED on a live screen

What a real Claude Code does with the file (it sources it before each Bash call) is not run here; the tests source the file with `sh` only in the existing quoting tests, which still pass. Windows and macOS builds were vetted and test-compiled, not run.
