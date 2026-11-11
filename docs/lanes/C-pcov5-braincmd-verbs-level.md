# C-pcov5-braincmd-verbs-level

Test-only lane: one new file, `internal/braincmd/verbs_level_test.go`, helpers prefixed `vl`, tests prefixed `TestVL`. No code outside the test file changed.

## Proof table

| Row | Result |
|---|---|
| 1 Coverage before | HOLDS. `go test -count=1 -coverprofile=<own>.out ./internal/braincmd/` exit 0, `coverage: 88.6% of statements`. (`mktemp -d` and `mkdir` outside the clone are refused in this sandbox, so the profile went to a gitignored `*.out` file in the clone.) |
| 2 levelStore changes.go:37 (changes :143, revert :227, supersede :66) | HOLDS. `TestVLNoGridHome` (3 subtests) exit 0; blocks `changes.go:37.3,38.1`, `:143.3,144.1`, `:227.3,228.1`, `supersede.go:66.3,67.1` count 1. |
| 3 changes.go:42 unknown project | HOLDS. `TestVLUnknownProject`; `changes.go:42.4,43.1` count 1. |
| 4 changes.go:47 no Grid record | HOLDS. `TestVLRevertFromAFolderWithNoGridRecord`; `changes.go:47.5,48.1` count 1. |
| 5 changes.go:58 project level without project | HOLDS. `TestVLProjectLevelWithoutAProject`; `changes.go:58.3,59.1` count 1. |
| 6 help for changes and supersede | HOLDS. `TestVLHelpPrintsUsageOnStdoutAndExitsZero`; exit 0, stdout starts with the usage constant; `changes.go:84.4,85.1`, `supersede.go:25.3,27.1` count 1. |
| 7 supersede bogus level, exit 2 | HOLDS. `TestVLSupersedeRefusesAnUnknownLevel`; `supersede.go:40.3,41.1` count 1. |
| 8 log not a file | HOLDS. `TestVLChangesRefusesALogThatIsNotAFile`; `changes.go:147.3,148.1` count 1. |
| 9 closed stdout on changes (json, plain) and revert | HOLDS. `TestVLClosedStdoutOnChangesAndRevert` (3 subtests); `changes.go:155.4,156.1`, `:161.4,162.1`, `:274.3,275.1` count 1. Uses the package's `failingWriter`. |
| 10 closed stdout on supersede (json, plain) | HOLDS. `TestVLClosedStdoutOnSupersede`; `supersede.go:83.4,84.1`, `:96.3,97.1` count 1. |
| 11 index rebuild fails after the supersede write | HOLDS. `TestVLSupersedeReportsTheWriteWhenTheIndexRebuildFails` (json: exit 1, stdout `wrote: true`, stderr error has `is superseded by` and `but rebuilding the index failed`; text: stdout `brain:home/c1 superseded by brain:home/c2 (v2, v2)`, stderr `brain: ` prefix); `supersede.go:86.4,88.1`, `:99.3,100.1` count 1. |
| 12 Coverage after and gates | HOLDS. Same command: `coverage: 92.1% of statements` (before 88.6%, after 92.1%). `go test -count=1 ./internal/braincmd/ ./internal/cli/` exit 0. `make lint` exit 0 (runs `go vet ./...`). `GOOS=windows go vet ./...` exit 0. `GOOS=darwin go build ./...` exit 0. |

Rules:

| Rule | Result |
|---|---|
| Packages importing braincmd | `go list` of imports: only `internal/cli`; its tests pass (exit 0, 64s). |
| New tests fail on base | NOT APPLICABLE. This is a coverage-only lane: the tests pin existing behaviour and pass on base code (there is nothing to fail against). Before the file the blocks above had count 0 (88.6%); after, count 1. |
| No existing test edited or deleted | HOLDS. `git status`: only the new test file. |
| No guard, gate, cap or hold added | HOLDS. Test-only. |
| No Unix-only calls; OS error text not asserted | HOLDS. No chmod, mkfifo, syscall or symlink; every asserted message is the package's own text. `GOOS=windows go vet ./...` exit 0. |
| No approval.json, no jobtest Approve | HOLDS. |
| No second failing writer | HOLDS. Reuses `failingWriter` (changes_test.go:505), `newHarness`, `threeVersions`, `svNotes`, `svReason`. |
| Not in lane (changes.go:231, supersede.go:70, decide.go) | HOLDS. Not touched. |

## What changed

New `internal/braincmd/verbs_level_test.go` with 10 tests and 2 helpers (`vlFail`, `vlBreakIndex`).

## NOT VERIFIED on a live screen

Nothing here needs a screen.
