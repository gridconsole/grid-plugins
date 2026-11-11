# C-pcov-migrate-version-ceiling: report

Repo: plugin. Branch: lane-C-pcov-migrate-version-ceiling. Tests only: two new files, no other file changed.

- `internal/migrate/version_ceiling_test.go`
- `internal/migrate/version_ceiling_unix_test.go` (`//go:build unix`)

## Proof table

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | Scan on an old home with VERSION "abc", "-1", FormatVersion+1, a folder, FormatVersion (separate home), none | HOLDS | `TestSourceVersionChecks` version_ceiling_test.go:18. "abc", "" and "-1" give exactly `invalid Grid VERSION`; FormatVersion+1 gives exactly `gridhome.FormatProblem(FormatVersion+1)`; a folder gives `VERSION must be a regular file`; FormatVersion (with and without a newline) gives `source is already a current Grid home`; no VERSION gives a plan (Current false, actions present, no error). Extra: a VERSION over maxRecord bytes gives readSmall's refusal (:68). `go test ./internal/migrate/ -run TestSourceVersionChecks -count=1 -v` exit 0 |
| 2 | WithCeiling | HOLDS | `TestCeilingStopsTheRepoWalk` version_ceiling_test.go:80. Repo R, project folder R/sub/proj. No ceiling: the project record has one repo, `Path` `../..` resolving to R, remote `example.com/team/ceil` (:118). `WithCeiling(R/sub)` and `WithCeiling(R/sub/proj)`: no repo and no keys (:119-120). Controls: ceiling at R and above R still find R (:121-122). Symlinked spelling of R/sub (:123, skipped on Windows): no repo, so gridhome.Real is applied. Exit 0 |
| 3 | RestoredFormat | HOLDS | `TestRestoredFormat` :136. Empty home gives ("unversioned", nil); a missing home the same; "2\n" gives "2"; " draft \r\n" gives "draft"; VERSION as a folder gives an error and "". Exit 0 |
| 4 | ProcessStart | HOLDS | `TestProcessStartOfThisProcess` :180: non-empty, stable over two calls, on Linux starts with `proc:` and has exactly one `:`. `TestProcessStartOfAFinishedProcess` :200: a pid spawned (`true`) and waited for gives `""` on Linux (the /proc/<pid>/stat read fails; on macOS `ps` exits non-zero, same answer). It is not a different token; the test asserts "". Exit 0 |
| 5 | specialFiles (unix-tagged file) | HOLDS | version_ceiling_unix_test.go:35, :59, :72. A named pipe in the source gives exactly one Unmapped line `alpha/projects/widget/events.pipe: socket, pipe or device file cannot be copied; remove it or move it away`; `WriteText` shows `unmapped:` and `0 conflicts, 1 unmapped`; `Apply` refuses with `migration blocked: resolve 0 conflicts and 1 unmapped`; the home is not made. A pipe in the destination gives one line. An in-place scan (one folder as source and destination) lists each pipe once. Exit 0. Not run on Windows (tagged `unix`; Windows compile in row 7) |
| 6 | sameEntry / sum via Scan | HOLDS | `TestSameEntryOfTwoSources` :240. Identical bytes (three homes, so the cached hash is used): no conflict. Same size, different bytes: exactly `knowledge/notes.md: destination already has different data`. Different size: the same conflict. File and folder, both orders: conflict. Two folders: none. Symlinks to one target: none; to two targets: conflict; symlink and file: conflict. Exit 0 |
| 7a | `go test ./internal/migrate/ -count=1` 0 fail | HOLDS | `ok ... internal/migrate 3.060s coverage: 87.4%` exit 0 (the package takes about 3 s on this VM, not 3 minutes) |
| 7b | Coverage before and after | HOLDS | Before (untouched clone, Linux, `go test -cover ./internal/migrate/ -count=1`): `coverage: 85.3% of statements`. After: `coverage: 87.4% of statements`. Per function after (`go tool cover -func`): sourceVersion 100.0%, WithCeiling 100.0%, RestoredFormat 100.0%, ProcessStart 100.0%, pidStart 100.0%, specialFiles 100.0%, checkVersion 100.0%, sameEntry 85.7%, sum 85.7%. `under` left at 0.0% as the brief says |
| 7c | Windows and macOS compile | HOLDS | `GOOS=windows go test -c -o /dev/null ./internal/migrate/` exit 0; `GOOS=darwin go test -c -o /dev/null ./internal/migrate/` exit 0. Compiled only; the Windows and macOS test runs need those hosts: `GOOS=windows go test ./internal/migrate/ -run 'TestSourceVersionChecks|TestCeilingStopsTheRepoWalk|TestRestoredFormat|TestProcessStart|TestSameEntryOfTwoSources'` on a Windows host, and the same with `-run 'TestNamedPipe'` added on macOS |
| 7d | `make lint` | HOLDS | prints `go vet ./...`, exit 0. `gofmt -l internal/migrate/` prints nothing |
| R1 | Packages that import the changed package, tested | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...` filtered on internal/migrate gives `internal/cli` (and migrate itself). `go test ./internal/cli/ -count=1`: `ok ... internal/cli 27.452s` exit 0 |
| R2 | New tests fail on base, pass on the lane | HOLDS, by mutation | The lane adds tests only, so the base code passes them. To show they can fail I broke `plan.go` twice, ran them, and reverted with `git checkout -- internal/migrate/plan.go` (`git status --short` empty; nothing of it committed). Mutation 1, the ceiling left unresolved (`filepath.Clean` instead of `gridhome.Real`): `--- FAIL: TestCeilingStopsTheRepoWalk/ceiling_spelled_through_a_symlink ... the project was mapped to a repo above the ceiling: repos [{Path:../.. Remote:example.com/team/ceil}]`. Mutation 2, `sameEntry` returns true when either side is a folder: `--- FAIL: TestSameEntryOfTwoSources/a_file_and_a_folder/alpha_has_the_file ... a file and a folder at one path did not conflict: []`. Not mutated: the pidStart cut and the pipe text |
| R3 | No existing test edited or deleted | HOLDS | `git diff --stat origin/main..HEAD` before the report commit: only `internal/migrate/version_ceiling_test.go` (348 lines) and `internal/migrate/version_ceiling_unix_test.go` (98 lines), 446 insertions, 0 deletions. The report commit adds this file |
| R4 | No guard, gate, cap or hold added; no non-test change | HOLDS | tests only |
| R5 | No attribution lines in commits; no personal paths, hosts or emails | HOLDS | two commits, one sentence each; the tests use `t.TempDir()` and `example.com` |

## What changed

- `version_ceiling_test.go`: `TestSourceVersionChecks`, `TestCeilingStopsTheRepoWalk`, `TestRestoredFormat`, `TestProcessStartOfThisProcess`, `TestProcessStartOfAFinishedProcess`, `TestSameEntryOfTwoSources`, and helpers `namedHomes`, `link`, `hasConflict`, `wantOneConflict`. It reuses `fixture`, `put`, `scan`, `absent`, `read`, `parseProject` and `internal/testgit`; no existing helper file was edited.
- `version_ceiling_unix_test.go` (`//go:build unix`): three named-pipe tests with `syscall.Mkfifo`, so the Windows compile has no Mkfifo.
- The pipe tests never put a pipe at a path that two sources claim, because `sum` would open it and a read-open of a pipe with no writer blocks.

## Things worth knowing

- No bug found; no row left out.
- The WIP bundle at call 35 was not written; the work was finished in one pass and the final bundle is written instead.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: the new tests on Windows or macOS (compiled only, commands above).

## Open items

None.

DONE
