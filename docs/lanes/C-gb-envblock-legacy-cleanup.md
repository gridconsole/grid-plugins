# C-gb-envblock-legacy-cleanup

The session-env hook drops old unmarked Grid export blocks when it rewrites its block.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. N legacy unmarked blocks plus one marked block become exactly one marked block | HOLDS | `TestSessionEnvLegacyUnmarkedBlocksAreDropped` (110 copies + one marked block, also copies with no marked block): `go test ./internal/hook/ -count=1` exit 0 |
| 2. A user line that is not a writeEnv line survives unchanged | HOLDS | `TestSessionEnvLegacyCleanupKeepsUserLines` (keeps order, and a `GRID_HOME` line with a different value) |
| 3. A file with no legacy lines is unchanged byte for byte; five grid-session-env goldens pass unedited | HOLDS | `TestSessionEnvWithoutLegacyLinesIsUnchanged`; `go test ./internal/golden/... ./internal/env/... -count=1` exit 0; no golden file changed |
| 4. `go test ./internal/hook/ -count=1` passes; the new test fails on main | HOLDS | pass: `ok .../internal/hook 10.062s`. On main (base 26c398b7, tests committed first): `--- FAIL: TestSessionEnvLegacyUnmarkedBlocksAreDropped` (`sessionenv_legacy_test.go:53: want exactly one marked block, got 34972 bytes`), `--- FAIL: TestSessionEnvLegacyCleanupKeepsUserLines` (`:86: writeEnv GRID_HOME lines: 3, want 1`, `:89: export PATH lines: 3, want 1`). The no-legacy test passes on main, as it should. Lane head: see report. |
| 5. `make lint` passes | HOLDS | `make lint` (`go vet ./...`) exit 0, no output after it |
| Importers of internal/hook | HOLDS | `go test ./internal/hook/... ./internal/cli/... ./internal/guard/... ./internal/hookio/... ./internal/packaging/... -count=1` all ok |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| Merge of origin/main before the last run | HOLDS | `git fetch origin && git merge origin/main`: Already up to date |
| Seed from the real root session file | NOT DONE | The builder's shell and Read tool are refused outside /srv/gridwork, so `/root/.claude/session-env/618bd718-.../sessionstart-hook-0.sh` could not be read. The test seed is built from the shape the brief describes (blank line, `export GRID_HOME`, `export GRID_PROJECTS`, `unset GRID_TASK`, plugin-dist `export PATH`, repeated 110 times), with paths under /srv/gridwork. |

## What changed

- `internal/hook/hook.go` `replaceEnvBlock`: outside the marked block, a line byte-equal to a line `writeEnv` writes (its own block lines, so the home, projects, task and PATH lines of this run) is dropped, with the blank lines next to the dropped lines. After a dropped run, one blank line is kept before the marked block when other lines precede it. No other line is touched, and a text with no such line is returned as it was. This also applies when the file has no marked block yet.
- `internal/hook/sessionenv_legacy_test.go`: three new tests.

## Notes

- A legacy copy with different values (an older home, an old `GRID_TASK='T-05'`, a PATH line from a different plugin root) is not byte-equal to this run's lines, so it stays, as the brief specifies. The marked block comes later in the file, so its values win.
- When the plugin dir is already on PATH, this run writes no PATH line, so old PATH lines stay.

## NOT VERIFIED on a live screen

Nothing here needs a screen. Not run on a real long-lived root session file (see the NOT DONE row).
