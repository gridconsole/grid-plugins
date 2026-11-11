# C-pcov6-golden-replay-setup: proof table

Test-only lane. Two new files, no other file changed:
`internal/golden/replay_setup_test.go` and `internal/golden/replay_setup_unix_test.go` (`//go:build unix`). Helpers all carry the `gr` prefix and tests the `TestGR` prefix.

| Row | Status | Proof |
|---|---|---|
| 1 Coverage before | HOLDS | `go test -count=1 -coverprofile=.../C-pcov6-golden-replay-setup-before.out ./internal/golden/` exit 0, 82.4%; `go tool cover -func=...before.out \| tail -1` = 82.4% (as expected) |
| 2 Cases walk error :84 | HOLDS | TestGRCasesWalkError; profile `replay.go:84.4,85.1 1 1` |
| 3 setup.json missing / `{` / box missing :103 :107 :112 | HOLDS | TestGRSetupMissing, TestGRSetupSyntax, TestGRBoxMissing; profile `103.3,104.1 1 1`, `107.3,108.1 1 1`, `112.3,113.1 1 1` |
| 4 Box folder :134 | HOLDS | TestGRBoxFolderBlocked (prefix `mkdir `); `134.4,135.1 1 1` |
| 5 Fixtures :139 :166 :150 | HOLDS | TestGRFixturesMissing, TestGRFixtureSymlinkUndeclared (exact text; skips if os.Symlink fails); `139.4,140.1 1 1`, `150.4,151.1 1 1`, `166.3,167.1 1 1` |
| 6 dirs :203 :206 safePath :352 | HOLDS | TestGRDirsRelative, TestGRDirsEscape, TestGRDirsOverFile; `203.4,204.1 1 1`, `206.4,207.1 1 1` (safePath :352 runs under the relative-path test) |
| 7 files :173 :178 :181 :190 :210 | HOLDS | TestGRFilesEscape, TestGRFilesRemoveMissing, TestGRFilesRemoveNonEmptyDir, TestGRFilesWriteOverDir; `173.5,174.1 1 1`, `178.6,179.1 1 1`, `181.6,182.1 1 1`, `190.5,191.1 1 1`, `210.3,211.1 1 1` |
| 8 links :215 :219 :225 | HOLDS | TestGRLinksRelative, TestGRLinksTargetEscapes, TestGRLinksOverFile (prefix `symlink `); `215.4,216.1 1 1`, `219.4,220.1 1 1`, `225.4,226.1 1 1`. These tests never call os.Symlink themselves, so no skip is needed. |
| 9 env.txt :230 :235 :239 | HOLDS | TestGREnvMissing, TestGREnvBlankLine, TestGREnvInvalid (NOEQUALS, =v); `230.3,231.1 1 1`, `235.4,235.12 1 1`, `239.4,240.1 1 1` |
| 10 input.json :245, go-diff.md :255 | HOLDS | TestGRInputMissing, TestGRDiffNoteIsFolder; `245.3,246.1 1 1`, `255.3,256.1 1 1` |
| 11 prelude :307, after_pre :311 | HOLDS | TestGRPreludeFails (exact `prelude grid pre failed: exit 3 stdout "" stderr "boom"`), TestGRAfterPreRelative; `307.4,308.1 1 1`, `311.3,312.1 1 1` |
| 12 Snapshots :315 :320 (unix file) | HOLDS | TestGRSnapshotBeforeUnsupported, TestGRSnapshotAfterUnsupported (Mkfifo hook on the prelude Run, then on the main Run only); `315.3,316.1 1 1`, `320.3,321.1 1 1` |
| 13 After coverage and gates | HOLDS | After profile 88.8% (before 82.4%). `go test -count=1 ./internal/golden/` exit 0; `make lint` exit 0 (runs `go vet ./...`); `GOOS=windows go vet ./internal/golden/` exit 0; `GOOS=darwin go vet ./internal/golden/` exit 0. Every block of rows 2-12 is 1 in the after profile (one grep -E call). The only blocks still 0 in the cited range are the ones the brief excludes: :143, :154, :158, :161, :186, :194, :222, :301. |
| Rule: importers tested | HOLDS | Importers of internal/golden, found by grep (all test files): brain, mcp, cli, project, job, plan, task, migrate, question. `go test -count=1` on those and golden: all `ok`, exit 0. replay.go is unchanged, so no behaviour moved. |
| Rule: new tests fail on a break | HOLDS (note) | The lane is test-only, so the tests pass on base as well; they pin existing behaviour. Mutation check: changed the :150 message in replay.go and ran TestGRFixtureSymlinkUndeclared, which failed with `got fixture symlinks are fine: l, want "fixture symlinks must be declared in setup.json: l"`. Reverted with `git checkout internal/golden/replay.go`; `git status` shows only the two new files. |
| Rule: no existing test edited or deleted | HOLDS | `git diff origin/main..HEAD --stat` lists the two new test files and this report only; no existing file is modified. |
| Rule: no guard/gate/cap | HOLDS | Test-only; no agent behaviour touched. |
| Rule: Windows/macOS portability | HOLDS | Mkfifo and syscall only in the `//go:build unix` file; no chmod; outside paths come from a second TempDir; OS error text is asserted by prefix up to the path only; os.Symlink is called only behind a `t.Skip`. Vet for windows and darwin exit 0. |
| Rule: approval.json / Approve | HOLDS | Not used. |
| Rule: commits | HOLDS | One commit, one sentence, no attribution lines. |

## What changed

- `internal/golden/replay_setup_test.go`: 24 test functions plus helpers `grDriver`, `grSetup`, `grRun`, `grOutside`, `grWantErr`, `grWantPrefix`, `grWantNotExist`.
- `internal/golden/replay_setup_unix_test.go`: 2 snapshot tests with a Mkfifo hook (`grFifoDriver`).
- Deviation from the brief: none. All rows behaved as the brief says.

## NOT VERIFIED

- The tests were not run on a real Windows or macOS machine, only vetted for those targets. Windows behaviour of the `mkdir `, `remove `, `open ` and `symlink ` prefixes and of EvalSymlinks on a missing box follows from the code, not from a run.
