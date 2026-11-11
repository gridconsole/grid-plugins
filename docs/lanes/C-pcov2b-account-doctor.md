# C-pcov2b-account-doctor: tests for account and doctor refusals

Tests only. Two new files: `internal/account/refusals_test.go`, `internal/doctor/hints_bundle_test.go`. No non-test file and no existing test file changed. Host: linux/amd64, go1.27.3.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 coverage before/after | HOLDS | Before (untouched clone, `go test -count=1 -coverprofile=.cov/<pkg>.out ./internal/<pkg>/ && go tool cover -func=... \| tail -1`, exit 0): account `coverage: 86.5% of statements`, doctor `85.3% of statements`. After (exit 0): account **95.1%**, doctor **90.6%**. Each above its own before. (Linux figures; they match the planner's darwin figures.) |
| 2 identity and env | HOLDS | `TestCodexIdentityRefusesBadAuthJSON` (refusals_test.go:23): not json, `a.b`, `a.!!!.c`, base64url of `not json` -> "", ""; also `{"tokens":{}}`, four parts, empty payload; `idToken("no-at-sign")` -> email "" plan "plus". `TestEnvGetenvConfigDirAndNow` (:47): Getenv set, ConfigDir `<dir>/y`; unset -> `<home>/.claude`; `Env{}.now()` within a minute. |
| 3 MakeFolder | HOLDS | `TestMakeFolderRefusals` (:68): `Env{}` -> "cannot find the home folder"; `~/.claude-x` a regular file -> `errors.Is ErrBadFolder`, bytes unchanged. |
| 4 paths and names | HOLDS | `TestRegistryPathAndNameHelpers` (:84): tilde with Home "", outside Home, Home's parent (`..`), Home -> "~"; expand("~") -> Home, expand("/a/b/../c") -> Clean; slugOf("+++@example.com") -> "account"; nameFor over 40 bytes -> local part, short -> email. |
| 5 broken registry | HOLDS | `TestStatusAndWindowsRefuseABrokenRegistry` (:118): `Status(Env{}, false)` -> "cannot find the Grid home; set GRID_HOME"; `not json` -> Status and Windows errors contain "is not readable JSON" and "it was left as it is", file unchanged; `{}` -> no error, empty list; registry.json a directory -> error; `accounts` a regular file -> `Status(env, true)` error, file unchanged. Every Scan-reaching env is `machine(t)`. |
| 6 Note and snapshots | HOLDS | `TestNoteAndSnapshotRefusals` (:164): `Note(Env{}, "{}", dir)` -> "cannot find the Grid home; set GRID_HOME"; `readSnapshot(Env{}, dir)` false; Note on `not json` -> error, no snapshot file; snapshot holding `not json` and holding `{}` -> Windows source "none", Windows non-nil and empty, Note not empty (with `~/.claude/keep` written). |
| 7 doctor hints and checks | HOLDS | `TestFixHintForAllowBlockAndUnknownNames`, `TestRunWithNoSettingsPathFailsTheAllowBlock` (SettingsPath "" -> not OK, detail "cannot locate the Claude settings file", Fix "Fix or move the Claude settings file, then run grid setup."), `TestValidateHooksTooLargeAndEmptyEvent` (goodHooks padded past 1 MiB with a top-level `"pad"` string -> "plugin hooks file is too large"; `"Stop":[]` -> "plugin hooks file has an empty event"), in hints_bundle_test.go. |
| 8 doctor bundle | HOLDS | `TestWriteReportReturnsTheWritersError` (first write, and the second write = the fix line of a failed check); `TestBundleRefusals` (`BundleOptions{Out: <tmp>/x.zip}` with no GridHome -> "cannot find the Grid home; set GRID_HOME" and no file; `logs` a regular file -> "logs must be a directory, not a symlink", no archive); `TestBundleLeavesOutUnsafeNamesAndOrdersEqualTimesByName` (skips on Windows, "fslock support is Unix-only": `a\b.log` left out, entries `doctor.txt,versions.txt,logs/ok.log`; 15 equal-time logs in zip order = name order, read with archive/zip); `TestReadBundleTailRedactsAKeyCutByTheLimit` (5 MiB + 50 bytes, cut inside `-----BEGIN KEY-----` -> starts `-----BEGIN REDACTED-----\n`, ends with the tail lines, no "KEY"). `Bundle(BundleOptions{})` itself was not called: with Out "" a failure to refuse would write under `support/` of the cwd; Out is set to a temp path so "no file" is checkable. |
| 9 checks | HOLDS | `go test -count=1 ./internal/account/ ./internal/doctor/`: ok, ok (exit 0). `GOOS=windows go vet ./internal/account/ ./internal/doctor/` exit 0. `GOOS=darwin go vet ...` exit 0. `go vet` (linux) exit 0. `make lint` (runs `go vet ./...`) exit 0. `gofmt -l internal/account internal/doctor` prints nothing. |
| Importers | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...`: `internal/account` and `internal/doctor` are imported only by `internal/cli` (and `cmd/grid` imports `cli`). `go test -count=1 ./internal/cli/ ./cmd/grid/`: `ok internal/cli 36.975s`, `cmd/grid [no test files]`, exit 0. |
| New tests fail on base | NOT APPLICABLE as stated | A tests-only lane: on base the test files do not exist, so the named tests do not run there. The proof is the flipped blocks: account functions `codexIdentity`, `tilde`, `expand`, `loadRecords`, `slugOf`, `nameFor`, `Windows`, `ConfigDir`, `readSnapshot`, `getenv`, `now`, `normEmail` go to 100%; doctor `fixHint`, `checkAllowBlock`, `ValidateHooks`, `WriteReport` go to 100% (all were below 100% before; `ConfigDir` was 0%). No code was changed to make a test pass. |
| No guard/gate/cap added | HOLDS | Diff is two test files and this report. |
| No existing test edited or deleted | HOLDS | `git diff --stat origin/main..HEAD` lists only the three new files. |
| No syscall.Mkfifo, chmod, installFakeClaude | HOLDS | Triggers are files and directories only; no provider is run or installed. |
| Bugs found | None | Every assertion matched the code as read; no row was dropped. |

## What changed

- `internal/account/refusals_test.go` (new): 6 tests for the refusal paths of codexIdentity, Env helpers, MakeFolder, registry path/name helpers, Status/Windows on a broken registry, Note and bad snapshots.
- `internal/doctor/hints_bundle_test.go` (new): 7 tests for fixHint, the allow-block check with no settings path, ValidateHooks limits, WriteReport writer errors, Bundle refusals, unsafe log names and equal-time order, readBundleTail PEM redaction.

## Outcome notes

- Row 8's equal-time test uses 15 logs written newest-name-first. With two logs `sort.Slice` is stable in practice and the tie-break could not be told apart; the block is covered either way.
- The `.cov/` folder holds coverprofiles (the shared `/tmp` was avoided; `mktemp` and `mkdir` outside the clone are refused here). It is untracked and not committed.

## NOT VERIFIED on a live screen

Nothing in this lane has a screen. Windows and macOS: compiled only (`go vet`), "compiled, needs Windows run": `GOOS=windows go test -c ./internal/account/ ./internal/doctor/` then run on a Windows host; "needs macOS run": `go test -count=1 ./internal/account/ ./internal/doctor/` on darwin.
