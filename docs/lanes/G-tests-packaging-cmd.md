# G-tests-packaging-cmd: proof table

| # | Row / rule | Result |
|---|---|---|
| 1 | main() only calls run and os.Exit; flags and exit paths unchanged | HOLDS. Before (base main.go:15-44): `flag.Parse()` on the default set, `fmt.Println(checkoutVersion())` then `return` for -describe, `os.Exit(1)` after `fmt.Fprintln(os.Stderr, err)`. After (internal/packaging/cmd/main.go:25-32 main, :35-72 run): `os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, deps{...}))`; run has the same order: -describe first (return 0), getwd, -release (print, return 0; on error fall through to the error print), then -dist -> Build(root, version, stdout) else Write(root, version), error -> stderr, return 1. Flags, names, defaults and help text are the same. |
| 2 | Tests: -describe, -release, default, -dist | HOLDS. internal/packaging/cmd/main_test.go: TestDescribePrintsSemverFromStub; TestDescribeFallsBackToSourceVersion (stub fails, junk, empty -> version.Version); TestReleasePrintsCommitCountOfGitRepo (testgit repo, 2 commits -> 0.1.2); TestReleaseOutsideGitExitsOneWithError (exit 1, error on stderr, no build or write); TestDefaultWritesWithVersionFlag, TestDefaultWritesSourceVersionWithoutFlag, TestDistBuildsWithVersionFlagAndStdoutLog (stubbed builders get the -version value). Also covered: builder error, getwd error, bad flag. `go test ./internal/packaging/cmd/ -v`: exit 0, all PASS. |
| 2a | New tests fail on base | HOLDS. With base main.go restored, `go test ./internal/packaging/cmd/`: `main_test.go:20:35: undefined: deps`, `main_test.go:40:9: undefined: run`, `main_test.go:148:10: undefined: run`, `FAIL ... [build failed]`. |
| 3 | `GOOS=windows go vet ./internal/packaging/...` | HOLDS, exit 0. Also `GOOS=darwin go vet ./internal/packaging/...` exit 0; `GOOS=windows CGO_ENABLED=0 go build ./internal/packaging/...` and the darwin build exit 0. |
| 4 | `go test ./internal/packaging/...` and `make lint` | HOLDS. Test: `ok .../internal/packaging`, `ok .../internal/packaging/cmd`, exit 0. `make lint` (go vet ./...) exit 0. `gofmt -l internal/packaging` prints nothing. |
| R1 | Importing packages tested | HOLDS. Only internal/packaging/layout_test.go (same package tree, covered by `go test ./internal/packaging/...`) imports internal/packaging; nothing imports the cmd main package. |
| R2 | Small commits, one-sentence messages, no attribution | HOLDS, see git log. |
| R3 | No guard, gate, cap, hold or blocking hook added | HOLDS. None added. |
| R4 | No existing test edited or deleted | HOLDS. Only main_test.go is new. |

## What changed
- internal/packaging/cmd/main.go: `main` now calls `run(args, stdout, stderr, deps)` and exits with its code. `deps` holds getwd, describe, build and write so a test can stub them. `checkoutVersion` takes the describe func; `gitDescribe` runs the same git command as before.
- internal/packaging/cmd/main_test.go: new, 10 tests.

Two small differences, neither reachable from the make targets: the flag set is named "packaging" instead of os.Args[0] in the usage line, and a bad flag now returns 2 from run (as flag.ExitOnError did) rather than exiting inside flag.Parse; `-h` returns 0 as before. The -release path calls packaging.ReleaseVersion directly (tested against a real git repo), not a stub.

## NOT VERIFIED on a live screen
Nothing here has a UI. `make dist` was not run (it builds every platform); its flags (`-dist -version`) are the ones the tests cover.
