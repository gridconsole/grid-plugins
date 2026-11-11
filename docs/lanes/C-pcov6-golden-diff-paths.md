# C-pcov6-golden-diff-paths

Test-only lane: two new files, `internal/golden/diff_paths_test.go` and `internal/golden/diff_paths_unix_test.go` (`//go:build unix`). Plus this report.

## Proof table

| Row | Result |
|---|---|
| 1 coverage before | HOLDS. `go test -count=1 -coverprofile=/srv/gridwork/tmp/C-pcov6-golden-diff-paths-before.out ./internal/golden/` exit 0, 82.4%. `go tool cover -func=... \| tail -1` prints `total: (statements) 82.4%`. |
| 2 splitLines U+0085/2028/2029 | HOLDS. `TestGdSplitLinesUnicodeBreaks` (diff_paths_test.go:11), `\u` escapes only, exit 0. |
| 3 matchingBlocks pruning and extension | HOLDS. `TestGdMatchingBlocksPopularLinePruned` (diff_paths_test.go:20) returns `[]block{{0, 99, 3}, {3, 250, 0}}`, exit 0. |
| 4 takeSnapshot walk error | HOLDS. `TestGdTakeSnapshotMissingRoot` (diff_paths_test.go:34), `errors.Is(err, os.ErrNotExist)`. |
| 5 safePath dangling link | HOLDS. `TestGdSafePathDanglingLink` (diff_paths_test.go:42), skips if `os.Symlink` fails. |
| 6a safePath through a file | HOLDS. `TestGdSafePathThroughFile` (diff_paths_unix_test.go:13), error prefix `lstat `. |
| 6b takeSnapshot FIFO | HOLDS. `TestGdTakeSnapshotRefusesFifo` (diff_paths_unix_test.go:25), error prefix `unsupported fixture file: `. |
| 7 coverage after | HOLDS. Full run with `...-after.out`, exit 0, 85.3% (was 82.4%). One `grep -E` of the after profile shows count 1 for: diff.go:22.4,23.1; :41.4,42.1; :111.3,111.35; :112.4,112.30; :113.5,114.1; :140.4,143.1; :145.4,146.1; :192.4,192.12; :194.4,194.12; replay.go:368.5,369.1; :376.4,377.1. |
| 7 `go test -count=1 ./internal/golden/` | HOLDS, exit 0. |
| 7 `make lint` | HOLDS, exit 0 (`go vet ./...`). |
| 7 `GOOS=windows go vet ./internal/golden/` | HOLDS, exit 0. |
| 7 `GOOS=darwin go vet ./internal/golden/` | HOLDS, exit 0. |
| Packages that import golden | `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...` finds none besides golden itself, so there is nothing further to test. |
| New tests fail on base | NOT APPLICABLE. The tests pin existing behaviour: on base they pass (the code is unchanged) and the blocks have count 0, not failures. The proof is the coverage move from count 0 to count 1. |
| No existing test edited, nothing deleted | HOLDS. `git status` shows only the two new test files and this report. |
| No guard, gate, cap or hold added | HOLDS. No non-test file changed. |
| Helpers carry the `gd` prefix | HOLDS. There are no helpers. The test names carry `Gd`. |
| Unix-only code behind a tag | HOLDS. `syscall` and Mkfifo appear only in the unix file. `os.Symlink` is behind a `t.Skip` on error. No error text asserted by the OS's own words. |
| approval.json / Approve | HOLDS. Not touched. |

## What changed
Six tests covering splitLines' Unicode breaks, matchingBlocks' popular-line pruning with block extension, takeSnapshot's walk error and unsupported-file refusal, and safePath's dangling-link and through-a-file errors. No behaviour change, no bug found.

## NOT VERIFIED on a live screen
Nothing here has a screen. The Windows and macOS runs are `go vet` builds only; the tests were not executed on those systems.
