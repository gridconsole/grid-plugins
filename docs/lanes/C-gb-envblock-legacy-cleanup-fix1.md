# C-gb-envblock-legacy-cleanup-fix1

Seed the legacy env-block test from the real root session file, and drop old Grid PATH lines that name a different plugin-dist or Grid bin folder.

Code commit: `06761f5fb835b06e939897a999ee23c26beb7472` (test commit `69a181a0`, on lane head `985d71c9`). The head SHA is in `/srv/gridwork/tmp/hand/C-gb-envblock-legacy-cleanup-fix1-report.md`.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. The real-seed test passes: one marked block remains, the legacy PATH lines are gone | HOLDS | `TestSessionEnvRealSeedKeepsOnlyTheMarkedBlock` reads `internal/hook/testdata/sessionstart-hook-0.sh` (the 562-line seed, copied as is; 110 plugin-dist PATH lines asserted) and requires the result to equal the one marked block byte for byte. `go test ./internal/hook/ -count=1` exit 0: `ok .../internal/hook 10.033s`. |
| Real-seed test fails before the code change | HOLDS | On `69a181a0` (tests only, code of `985d71c9`): `--- FAIL: TestSessionEnvRealSeedKeepsOnlyTheMarkedBlock` / `sessionenv_realseed_test.go:30: want exactly one marked block, got 117 lines (111 PATH lines)`. This is the reviewer's must-fix 1: 110 legacy PATH lines kept. |
| 2. An unrelated `export PATH='/opt/x/bin':"$PATH"` line survives | HOLDS | `TestSessionEnvLegacyPathCleanupKeepsOtherPathLines` keeps `/opt/x/bin`, `/opt/plugin-dist-x/bin`, `/srv/gridwork/plugin-dist/sbin` and `"$HOME/go/bin:$PATH"`, and drops plugin-dist / `.grid` bin lines. Before the change: `sessionenv_realseed_test.go:62: plugin-dist PATH lines left: 1`, `:65: Grid bin PATH lines left: 1`. After: passes (same run as row 1). |
| 3. The earlier three tests and the five goldens pass unedited | HOLDS | `git diff --stat origin/main..HEAD` shows `sessionenv_legacy_test.go`, `sessionenv_replace_test.go` and all golden files untouched; `go test ./internal/hook/ ./internal/golden/... -count=1` exit 0: `ok .../internal/hook 10.033s`, `ok .../internal/golden 0.231s` |
| 4. `go test ./internal/hook/ ./internal/golden/... -count=1` and `make lint` pass; head SHA in the report | HOLDS | both exit 0 (`make lint` = `go vet ./...`, exit 0); head SHA in the report file |
| Importers of internal/hook | HOLDS | `go list` importers: `internal/cli`, `internal/packaging`. `go test ./internal/cli ./internal/packaging -count=1`: both ok |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| Merge of origin/main before the last run | HOLDS | `git fetch origin && git merge origin/main`: Already up to date |
| New tests fail on base, pass on lane | HOLDS | see rows 1 and 2 |

## What changed

- `internal/hook/hook.go`: new `isGridPathLine(line, home)`. `replaceEnvBlock` takes the Grid home and also treats as an old Grid line any `export PATH='<dir>/bin':"$PATH"` where `<dir>` is the Grid home, or its last folder is `plugin-dist` or `.grid`. Any other PATH line (other folders, `sbin`, double-quoted forms) is kept.
- `internal/hook/testdata/sessionstart-hook-0.sh`: the real seed, as is. It holds only Grid export lines and root-home Grid paths; no emails or hosts.
- `internal/hook/sessionenv_realseed_test.go`: two new tests.

## Notes

- An old PATH line whose folder is not this run's home, a `plugin-dist` folder or a `.grid` folder (for example a plugin cache path) stays, unless it is byte-equal to this run's PATH line.
- The existing three tests and `replaceEnvBlock`'s behaviour for GRID_HOME, GRID_PROJECTS and GRID_TASK lines are unchanged.

## NOT VERIFIED on a live screen

Nothing here needs a screen.
