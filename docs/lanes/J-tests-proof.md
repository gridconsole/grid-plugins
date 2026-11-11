# J-tests-proof: unit tests for proof Level, ReadEvidence, Rebuild and Index.Check

Repo: plugin. Branch lane-J-tests-proof. Only new file under internal/proof: `proof_units_test.go`. `proof.go` is unchanged.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Level maps each kind; unknown errors | HOLDS | `proof_units_test.go:14` (TestLevelMapsEachProofKind: "", check, log, screenshot, video) and `:29` (TestLevelUnknownKindErrors: magic, Screenshot, " log", audio; error names the item). `go test ./internal/proof/` exit 0. |
| 2. ReadEvidence: regular file reads; missing and directory error; symlink outside task | HOLDS | `:42` regular file; `:53` missing is not-exist, directory gives "regular file" error; `:63` over 20 MiB errors; `:82` TestReadEvidenceSymlinkOutsideTask: ReadEvidence and Add both refuse a symlink source, no record is left, and the index stays unproven (same refusal as TestSymlinkProof). Exit 0. |
| 3. Rebuild after deleting index equals Build; Check passes on full and names missing item | HOLDS | `:110` TestRebuildAfterDeletingIndexMatchesBuild (DeepEqual of Rebuild, Build and the first Rebuild; index.json bytes equal; Build does not write the file). `:154` TestIndexCheck (full passes, empty errors "no items", missing names "i-2 lacks screenshot proof" and not i-1). `:169` TestIndexCheckOnBuiltIndex on a real built index. Exit 0. |
| 4. `go test ./internal/proof/` 0 fail | HOLDS | `go test ./internal/proof/` -> `ok ... 0.076s`, exit 0. Existing tests unchanged and pass. |
| New tests fail on base | HOLDS (by mutation) | The tests cover code already on main, so they pass on base. To show they bite I changed proof.go temporarily (reverted, `git status` shows only the new test file): `if !item.Proven` -> `if false` and `if !kindOK(level)` -> `if false`. Result: `FAIL TestLevelUnknownKindErrors: Level("magic") = "magic", <nil>; want an error and no level`, `FAIL TestIndexCheck: missing proof: want i-2 named, got <nil>`, `FAIL TestIndexCheckOnBuiltIndex: want i-2 named, got <nil>`. |
| Tests of changed package and importers | HOLDS | Importers by `go list`: internal/cli, internal/task. `go test ./internal/proof/ ./internal/cli/ ./internal/task/` -> all ok, exit 0. |
| Lint | HOLDS | `go vet ./internal/proof/` exit 0; `gofmt -l internal/proof` empty. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 and `go vet ./internal/proof/` (compiles the tests) exit 0; same for `GOOS=darwin`. |
| No edit of existing tests; nothing deleted | HOLDS | `git status`: one new file plus this report. |
| No guard, gate, cap or hold added | HOLDS | Tests only. |
| New file named for the subject, not a shared table | HOLDS | `internal/proof/proof_units_test.go`. |
| Bug found in proof.go | None | No bug found, no assertion left out. |

## What changed

- New `internal/proof/proof_units_test.go`: 9 tests for Level, ReadEvidence (incl. the 20 MiB limit, which the brief did not name), Rebuild and Index.Check. The symlink test skips where symlinks cannot be created, like TestSymlinkProof.
- This report.

## Notes

- `ReadEvidence` on a symlink is refused at the `Lstat` regular-file check, so the `OpenNoFollow` call after it is not reached by a symlink in these tests.

## NOT VERIFIED on a live screen

Nothing here touches a UI.
