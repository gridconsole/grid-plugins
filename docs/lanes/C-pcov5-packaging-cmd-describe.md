# C-pcov5-packaging-cmd-describe

Lane proof table. Repo plugin, branch lane-C-pcov5-packaging-cmd-describe, based on main d0400870.

| Row / rule | Result | Proof |
|---|---|---|
| 1. Coverage before | HOLDS | `go test -count=1 -coverprofile=<own folder>/before.out ./internal/packaging/cmd/` exit 0, `go tool cover -func` total 91.7%. Blocks at 0: main.go:26.2,32.1 (main), main.go:44.4,45.1 (-h), main.go:75.2,76.1 (gitDescribe). The profile folder was `.pcov5cov/` inside the clone (git-ignored, not committed) because `mktemp` and `mkdir` outside the clone were refused in this sandbox. |
| 2. `-h` returns 0, usage on stderr | HOLDS | `TestPdHelpExitsZeroWithUsageOnStderr` (internal/packaging/cmd/describe_help_test.go:11) passes: code 0, stdout empty, stderr starts `Usage of packaging:` and names -describe, -dist, -release, -version, c.build and c.write empty. |
| 3. gitDescribe in a tagged checkout | HOLDS | `TestPdGitDescribeInTaggedCheckout` (describe_help_test.go:34) passes: Isolate, Init, tag v1.2.3, t.Chdir; gitDescribe returns `v1.2.3\n`, nil; checkoutVersion(gitDescribe) equals the first result of version.FromDescribe("v1.2.3"). |
| 4. gitDescribe outside git | HOLDS | `TestPdGitDescribeOutsideGitFallsBack` (describe_help_test.go:57) passes: t.Chdir to a temp folder with GIT_CEILING_DIRECTORIES set to its parent; gitDescribe returns an error; checkoutVersion returns version.Version (the constant). |
| 5. Coverage after and gates | HOLDS | `go test -count=1 -coverprofile=.pcov5cov/after.out ./internal/packaging/cmd/` exit 0, total 97.2% (before 91.7%). Quoted: `main.go:44.4,45.1 1 1`, `main.go:75.2,76.1 1 1`. The only block left at 0 is `main.go:26.2,32.1 1 0` (main()). `make lint` exit 0. `GOOS=windows go vet ./...` exit 0. |
| New tests fail on base, pass on lane | NOT APPLICABLE as a failing run | The lane adds tests only, for code that already works. On base the tests do not exist, and the three blocks they cover sit at count 0 in the before profile (row 1). Nothing was changed to make a test fail. |
| Tests of changed packages and importers | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./... \| grep internal/packaging/cmd` matches only the package's own line, so nothing imports it (it is package main). `go test -count=1 ./internal/packaging/cmd/` exit 0, 97.2%. |
| Windows and macOS builds | HOLDS | `GOOS=windows go vet ./...` exit 0. `GOOS=darwin go vet ./internal/packaging/cmd/` exit 0. |
| gofmt | HOLDS | `gofmt -l internal/packaging/cmd` prints nothing. |
| Test-only; no code change | HOLDS | `git status` showed one new file under internal/packaging/cmd. The only other addition is this report file, which the runbook requires. |
| `pd` prefix on every helper | HOLDS | The new file adds no helpers. The three tests are named `TestPd...` and reuse `calls`, `stubs` and `runArgs` from main_test.go. |
| No Unix-only calls | HOLDS | No chmod, Mkfifo, syscall or symlink in the new file. |
| No guard, gate or cap on agents; no approval.json | HOLDS | None added. |
| No personal paths, hosts or emails; no attribution lines | HOLDS | The commit message is one sentence with no attribution lines. |

## What changed

- New `internal/packaging/cmd/describe_help_test.go` with three tests.
- This report.

## NOT VERIFIED on a live screen

Nothing here is UI. A Windows and macOS test run was not made; only `go vet` for those targets was.
