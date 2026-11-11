# A-tests-gitx: unit tests for every gitx function

Only file added besides this report: `internal/gitx/gitx_lane_test.go`. `gitx.go` and `testgit` are unchanged.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 Version: empty PATH gives "git not found"; real PATH gives "git version…" | HOLDS | `go test ./internal/gitx/ -count=1` exit 0; internal/gitx/gitx_lane_test.go:39-47 (t.Setenv PATH to empty temp folder), :49-54 |
| 2 FindRoot: nested a/b/c; ceiling a gives ("", false); plain x/y with ceiling x; ceiling "" ; worktree subfolder | HOLDS | internal/gitx/gitx_lane_test.go:63, :69, :76, :84, :90; test exit 0 |
| 2b FindRoot "" ceiling reaches the volume root without looping | HOLDS | gitx_lane_test.go:104-120 (starts at the volume root itself, so nothing above t.TempDir matters) |
| 3 CommonDir: repo and worktree both equal gridhome.Real(<repo>/.git), absolute, filepath form, no "/" on Windows; non-repo errors | HOLDS (compiled, file:line) for the Windows part | gitx_lane_test.go:122-157; Windows assertion at :145; `GOOS=windows go test -c -o /dev/null ./internal/gitx/` exit 0. Linux run of the rest: exit 0 |
| 4 Remotes: origin+upstream in order; none gives nil; non-repo gives nil; `C:\repos\x` as written | HOLDS | gitx_lane_test.go:161, :182, :189, :195 (also :171 empty value skipped); exit 0 |
| 5 RootCommit: empty gives ""; one commit gives its sha; two roots (orphan + --allow-unrelated-histories merge) give the smaller | HOLDS | gitx_lane_test.go:208, :215, :223 (second root has its own message, :229; asserts 2 roots at :236) |
| 6a `go test ./internal/gitx/ -count=1` | HOLDS | exit 0, 0 fail |
| 6b `go test -cover ./internal/gitx/` above 90% | HOLDS | `ok ... coverage: 97.4% of statements` |
| 6c `GOOS=windows go test -c -o /dev/null ./internal/gitx/` | HOLDS | exit 0 |
| 6d `GOOS=darwin go test -c -o /dev/null ./internal/gitx/` | HOLDS | exit 0 |
| 6e `make lint` | HOLDS | prints `go vet ./...`, exit 0 |
| New tests fail on a broken base | HOLDS | These are tests of unchanged code, so they pass on base. Mutation check: with gitx.go temporarily changed to `return roots[len(roots)-1]`, `go test -run TestRootCommit` exit 1: `RootCommit = "bee68baa…", want the smaller of "bee68baa…" and "29fab234…"`. gitx.go restored with `git checkout`; `git status` showed only the new test file |
| Importers of gitx pass | HOLDS | importers (from `go list`): cli, doctor, migrate, projects, task, where. `go test -count=1` on gitx where projects migrate task doctor cli: all `ok`, exit 0 |
| No existing test edited or deleted | HOLDS | diff is one new test file and this report |
| No guard, gate, cap or hold added | HOLDS | tests only |
| Commits: small, one sentence, no attribution | HOLDS | `Add unit tests for every gitx function.` |
| No dependence on what lies above t.TempDir | HOLDS | FindRoot rows use ceilings; CommonDir and Remotes non-repo rows set GIT_CEILING_DIRECTORIES to EvalSymlinks of the folder's parent (plainFolder, gitx_lane_test.go:20-31) |
| No bug found in gitx.go | HOLDS | no NOT DONE assertion left out |

Windows host run: for the orchestrator: `go test ./internal/gitx/ -count=1 -v`

## What changed

New `internal/gitx/gitx_lane_test.go` with TestVersion, TestFindRoot, TestFindRootWalksToVolumeRoot, TestCommonDir, TestRemotes, TestRootCommit. Coverage of gitx goes from 25.6% to 97.4%.

## NOT VERIFIED

- Windows forward-slash to backslash conversion in CommonDir, and the Windows `C:\repos\x` remote: compiled only, needs a Windows host.
- A run with TMPDIR inside a checkout (not run: the shell allow list does not take an env prefix on go). The tests avoid depending on it by construction.
- The macOS run.
