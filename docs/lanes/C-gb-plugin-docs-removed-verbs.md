# C-gb-plugin-docs-removed-verbs

| Row | Result |
|---|---|
| 1. `grid ports` gone from README.md | HOLDS. Before: README.md:36. After: `grep -c "grid ports" README.md` gives 0. |
| 2. no `refusal decide` / `cmd-refusal-decide` in contract/README.md | HOLDS. `grep -c` gives 0 for both files. |
| 3. `go test -count=1 -run TestGbDocs ./internal/cli/` | HOLDS, exit 0 (`ok ... internal/cli`). `internal/cli/gb_docs_verbs_test.go` has the verb walk (TestGbDocsVerbsAreImplemented, allow set {help, home, on}, each with a reason comment) and the schema-link walk (TestGbDocsSchemaLinksExist). |
| 4. fails on base | HOLDS. With both READMEs stashed to main: `FAIL: TestGbDocsVerbsAreImplemented ... README.md names `grid ports`, which is not an Implemented command` and `FAIL: TestGbDocsSchemaLinksExist ... links schemas/cmd-refusal-decide.json, which is missing`; exit 1. `git stash pop` restored the files. |
| 5. diff names only the three files | HOLDS. `git diff main --stat`: README.md, contract/README.md, internal/cli/gb_docs_verbs_test.go. `make lint` (go vet ./...) exit 0. |
| Tests of changed package and importers | HOLDS. `go list` importers of internal/cli: cmd/grid, lib. `go test -count=1 ./internal/cli/ ./cmd/grid/ ./lib/`: ok, ok, no test files. |
| Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| New test fails on base, passes on lane | HOLDS (row 4 and row 3). |
| No existing test edited, no production Go file touched, no guard added | HOLDS. |
| RealPlugin tests | Not applicable. |

## What changed
- README.md:36: now "`grid env`: dev environments." (dropped `grid ports` and "and port blocks").
- contract/README.md: removed the cmd-refusal-decide row, "`refusal decide` (also `decide`)" in the task-operands sentence, "`refusal decide`/`decide`" in the receipts sentence, and the example lines `grid refusal decide ...` and `grid decide ...`.
- New test file internal/cli/gb_docs_verbs_test.go.

## Notes
- Beyond the brief's named lines, I also removed the example line `grid decide T-01-renamed R-1 allow --json` (the line after :519). `decide` is unknown (TestRefusalVerbsAreUnknown), and the walk would otherwise fail on it with the allow set held at {help, home, on}.
- On base the verb walk flags only `ports`: the contract's `grid refusal decide` yields the word `refusal`, which is Implemented, so that row is caught by the schema-link walk.

## NOT VERIFIED on a live screen
Nothing here needs a screen.
