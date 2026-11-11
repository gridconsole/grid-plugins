# C-pcov2b-packaging-agent-version

Tests only: four new files, no non-test file changed. Head: see the report.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1 coverage before (Linux, untouched clone) | HOLDS | `go test -count=1 -coverprofile=<file> ./internal/<pkg>/ && go tool cover -func=<file> \| tail -1`: packaging `total: (statements) 67.4%`, agent `94.7%`, version `95.2%`. Exit 0 each. (`mkdir` was refused in the sandbox, so the profiles went to uniquely named files in /tmp, not a `mktemp -d` folder.) |
| 1 coverage after | HOLDS | packaging `total: (statements) 80.0%`, agent `97.6%`, version `98.6%`. Each is above its own before (67.4, 94.7, 95.2). |
| 2 Build refusals | HOLDS | `internal/packaging/dist_refusals_test.go`: TestBuildRefusesDistThatIsAFile (file unchanged), TestBuildRefusesChecksumsThatIsANonEmptyDirectory (inner file still there), TestBuildRefusesMissingSkillsFolder, TestBuildRefusesMissingLicense (errors.Is os.ErrNotExist), contrast TestBuildRefusesInvalidVersionContrast. Each asserts no `dist/plugin/bin/grid-*` afterwards. `go test -count=1 ./internal/packaging/` exit 0. |
| 3 stage, Write, writeChecksums | HOLDS | TestStageRefusesPluginUnderAFile, TestWriteRefusesRootThatIsAFile, TestWriteRefusesLauncherThatIsADirectory, TestWriteChecksumsRefusesMissingPlugin (also asserts no SHA256SUMS). Same run, exit 0. |
| 4 agent lookup | HOLDS | `internal/agent/lookup_test.go`: TestLookupRelativePathEntryIsRefused, TestLookupEmptyPathEntryIsCurrentDirectory, TestLookupResolveInSkipsRelativeStandardDir, TestLookupPathWithoutToolIsNotFound. The first three skip on Windows via `t.Skip` naming PATHEXT (lookupToolDir). `go test -count=1 ./internal/agent/` exit 0. |
| 5 check and helpers | HOLDS | TestLookupCheckWithoutLookupUsesResolve, TestLookupFromBrewRegularFileOutsideCaskroom, TestLookupFixLineClaudeInstallLinux, TestLookupProbeOutputStartFailure. Same run, exit 0. |
| 6 redirects and fetch | HOLDS | `internal/version/compare_more_test.go`: TestManifestClientRedirectTooMany, ...HTTPToHTTP, ...HTTPSToHTTPContrast, TestFetchRefusesUnparseableAddress. No network. `go test -count=1 ./internal/version/` exit 0. |
| 7 Compare | HOLDS | TestCompareMoreNumbersByLength, TestCompareMorePrereleaseBranches (same file). |
| 8 FromDescribe | HOLDS | `internal/version/describe_overflow_test.go`, quotes describeTag (describe.go:10), asserts "" and an error that is `strconv.ErrRange` (the code returns Atoi's error, describe.go:37-40). |
| 9 tests, vet, lint | HOLDS | `go test -count=1 ./internal/packaging/ ./internal/agent/ ./internal/version/`: ok, exit 0. `GOOS=windows go vet` and `GOOS=darwin go vet` on the three packages: exit 0 each. `make lint` (`go vet ./...`): exit 0. |
| Importers' tests | HOLDS | Importers by `go list` imports: agent <- internal/cli; version <- internal/cli, internal/doctor, internal/migrate, internal/packaging/cmd; packaging <- internal/packaging/cmd (it imports internal/packaging; cmd/grid is reached only via cli). `go test -count=1 ./internal/packaging/... ./internal/agent/ ./internal/version/ ./internal/doctor/ ./internal/migrate/ ./internal/cli/`: all ok, exit 0. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0. |
| New tests fail on base | NOT DONE (cannot) | The lane adds tests only and the brief says to assert what the code does today, so every new test passes on base as well; none can fail there. Their value is the covered blocks above, not a red-to-green change. |
| No guard/gate/cap on agents; no non-test file; no existing test edited | HOLDS | `git diff origin/main --stat` lists only the four new test files. |
| No push, no personal paths | HOLDS | Bundle only. |

## What changed

- `internal/packaging/dist_refusals_test.go` (package packaging, reuses `writeTestFile`)
- `internal/agent/lookup_test.go` (helpers and tests start with `lookup`)
- `internal/version/compare_more_test.go`
- `internal/version/describe_overflow_test.go`

Where the code differed from the brief: nothing found; all expected results matched the source.
Not in the lane's scope and left as is: Build's go build calls, which no row reaches.

## NOT VERIFIED on a live screen

No UI. Windows run of the lookup tests: compiled (`GOOS=windows go vet`), needs a Windows run: `go test -count=1 ./internal/agent/ ./internal/packaging/ ./internal/version/`. macOS likewise (`GOOS=darwin go vet` only).
