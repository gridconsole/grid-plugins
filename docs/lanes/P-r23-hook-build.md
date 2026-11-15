# P-r23-hook-build: no go build inside a hook call

Brief: row 6 of `U-chk-plugin-rev-skills.md` (BUG): `bin/grid` ran `go build` inside a hook call whose SessionStart timeout is 10s.

## Proof table

| Row | Result | Proof |
|-----|--------|-------|
| 1. Row 6 confirmed or NOT A FAULT | CONFIRMED | `bin/grid:53-69` on main: in a source checkout, when `dist/plugin/bin/grid-<platform>` and `dist/grid` are missing or older than any `*.go`, `go.mod` or `go.sum`, the launcher runs `go build -C $root ... ./cmd/grid` whatever `$1` is. The new tests on main: `TestCheckoutLauncherHookDoesNotBuildStaleBinary`: `launcher_test.go:202: hook ran "freshly-built\n", want the existing build`; `TestCheckoutLauncherHookDoesNotBuildMissingBinary`: `launcher_test.go:213: hook printed "freshly-built\n", want a silent skip`. (Running the real binary for timing is not part of this lane; the fake `go` in the tests records that a build ran.) |
| 2. Fix | HOLDS | `internal/packaging/grid.sh:61-68`, regenerated to `bin/grid` with `go run ./internal/packaging/cmd` (exit 0). In a hook call the launcher runs the existing `dist/plugin/bin/grid-<platform>`, else the existing `dist/grid`, else exits 0 silently; it never calls `go`. Other commands still build as before. A release install (`bin/grid-<platform>` present) was never affected. |
| 3. Test per fixed row, fails on main, passes on head | HOLDS | `TestCheckoutLauncherHookDoesNotBuildStaleBinary` and `TestCheckoutLauncherHookDoesNotBuildMissingBinary` (fail lines above, on main's `grid.sh`); both pass on head. `TestCheckoutLauncherCommandStillBuilds` passes on both (guards the dev flow). `go test -count=1 ./internal/packaging/...`: ok, exit 0. |
| 4. go test of changed packages and importers | HOLDS | Importers by `go list`: `internal/packaging/cmd` only. `go test -count=1 ./internal/packaging/...`: `ok .../internal/packaging 9.763s`, `ok .../internal/packaging/cmd 0.365s`. Also `./internal/doctor/` ok (reads the launcher). |
| 5. make lint | HOLDS | `make lint` exit 0 (`go vet ./...`). |
| 6. Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| 7. Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `ok .../internal/cli 0.646s`. `go test -count=1 ./internal/job/lint/`: `ok 3.104s`. |
| 8. Merge origin/main before last run | HOLDS | `git fetch origin && git merge origin/main`: Already up to date. |
| 9. Rules: no guard, no attribution, no edited existing test | HOLDS | The change removes a wait from a hook; it adds no guard or cap. Existing tests are unchanged (`TestCheckoutLauncherHookFailsOpenWithoutGo` still passes). Commit has no attribution line. |
| Whole-module test, cross-repo set | not run, the orchestrator runs it at landing | |

## What changed

- `internal/packaging/grid.sh` and the generated `bin/grid`: a hook call never compiles.
- `internal/packaging/launcher_test.go`: three tests.

## NOT VERIFIED

- Real wall-clock time of a SessionStart hook in a stale source checkout (no live run). The tests prove only that `go` is not called.
- A stale build runs inside a hook until the developer runs `make build`; that is the intended trade.
