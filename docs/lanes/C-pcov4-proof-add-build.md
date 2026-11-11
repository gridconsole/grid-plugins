# C-pcov4-proof-add-build: proof refusals in Add, readRecords and Build

Head: see the report (the commit that adds this file follows 58130524).

## Proof table

| Row | Result | Command / place |
|---|---|---|
| 1 Coverage before and after | HOLDS | Before, `go test -count=1 -coverprofile=<own file> ./internal/proof/` and `go tool cover -func=...`: `total: (statements) 78.4%`. After, same command: `total` 85.4% (test output `coverage: 85.4% of statements`). 85.4 > 78.4. The brief's `d=$(mktemp -d) && ...` form and `mkdir` were refused by the shell here, so the profile went to a file under tmp/hand. |
| 2 Add extension and size | HOLDS | `TestProofAddBuildExtensionAndSize`: x.dat gives a `.bin` File and identical stored bytes; empty x.log gives exactly "proof must contain 1 byte to 20 MiB" and no proof.json; asserts `MaxBytes == 20<<20`. Code proof.go:127-137. |
| 3 Add kind and type | HOLDS | `TestProofAddBuildKindAndType`: screenshot+shot.txt and video+clip.png give the exact messages (proof.go:138-143); proof/i-1 holds no entry afterwards. |
| 4 Now in a non-UTC zone | HOLDS | `TestProofAddBuildNow`: `time.FixedZone` +05:30 with 123456789 ns; Record.At and proof.json At both equal `2026-03-04T09:34:05.123456789Z` (proof.go:164-167). |
| 5 readRecords through Add and Build | HOLDS | `TestProofAddBuildReadRecords`, four subtests: `{` gives an error starting "invalid proof.json"; `null` gives "proof.json must be an array" (proof.go:220-225). |
| 6 Build refusals | HOLDS | `TestProofAddBuildRefusals`, seven subtests (kind photo, empty By, newline Note, At "yesterday", File of i-2, File proof.json, File ../a.txt) with the exact messages for i-1 (proof.go:262-271), plus a valid record that is accepted. |
| 7 Build sort | HOLDS | `TestProofAddBuildSortByFile`: same At, written b.txt then a.txt, comes back a then b (proof.go:286-292). |
| 8 Test, vet, build, lint | HOLDS | `go test -count=1 ./internal/proof/` ok (exit 0); `GOOS=windows go vet ./...` exit 0; `GOOS=darwin go vet ./internal/proof/` exit 0; `GOOS=windows go test -c -o /dev/null ./internal/proof/` exit 0; `make lint` (runs `go vet ./...`) exit 0; `gofmt -l internal/proof` empty. |
| Importers tested | HOLDS | Direct importers of internal/proof, from `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...`: internal/task and internal/cli. `go test -count=1 ./internal/task/ ./internal/cli/`: both ok. Everything else reaches proof only through those two; no non-test file changed. |
| New tests fail on base | NOT DONE | The brief asks only for tests of code that already exists, so they pass on base too (base has no such test, and the production code is untouched). There is nothing to fail on. |
| No existing test edited, none deleted | HOLDS | `git diff --stat main..HEAD`: one added file. |
| No guard, gate, cap or hold added | HOLDS | No non-test file changed; the 20 MiB number is pinned as `MaxBytes` and not changed. |
| No personal paths, hosts or emails | HOLDS | The new file uses only `t.TempDir()` paths. |
| Windows-safe new file | HOLDS | No build tag, no unix-only calls, no chmod, no Mkfifo; vet and test compile for windows exit 0. |
| Report-level rules (60-call budget, one command per call) | HOLDS | About 25 calls used. |

## What changed

One new file: `internal/proof/proof_add_build_test.go` (`package proof`). It reuses `fixture` and adds helpers prefixed `proofAddBuild`. No production code, no existing test file touched.

Where the code differs from the brief: nothing. Each row's expected message matched the source as read.

## NOT VERIFIED on a live screen

None. This is a tests-only lane with no UI. The macOS and Windows builds are checked by vet and compile only (`GOOS=darwin go vet ./internal/proof/`, `GOOS=windows go test -c -o /dev/null ./internal/proof/`); the tests themselves have not run on a Windows or macOS host: compiled, needs Windows run (`go test ./internal/proof/`).

## Open items

None.
