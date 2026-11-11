# C-ft-job-record-fix-hint

Base: 9dba6dc6a3c42b88f1526c01981b126ae83bd2eb. Base check: `internal/job/lint/json.go:27` on that sha still passed `"Fix the field the error names so the record matches the job-record schema."` for every `job.record` refusal.
Code head: 8a802d020a1eb1267696a51d9fccf0b55b143e25 (this report is a second commit on top).

## Proof table

| Row | Result |
|---|---|
| 1. `go test ./internal/contract/ -run TestFtHint -count=1` passes: enum hint lists every schema value (read from `contract/schemas/job-record.json`), unknown property names the known properties, missing required names the field | HOLDS. `go test ./internal/contract/ -run TestFtHint -count=1 -v` exit 0, 6 tests PASS (`TestFtHintEnumListsEverySchemaValue` top-level `$.visible` and nested `$.links[0].rel`, `...UnknownPropertyNamesKnownProperties`, `...MissingRequiredNamesField`, `...EmptyWhenItCannotTell`, `...MessagesUnchanged`, `...CheckJSONFixOverInvalidFixtures`) |
| 2. Loop over every file in `contract/fixtures/job-record/invalid` through CheckJSON: Fix is the hint where Hint has text, the generic line otherwise | HOLDS. `TestFtHintCheckJSONFixOverInvalidFixtures` (`ft_hint_test.go:177`) PASS. Also asserts the loop sees both a hinted and a generic finding: priority-retired and visible-not-enum are hinted, the other four stay generic |
| 3. `go test ./internal/job/lint/ ./internal/contract/ -count=1` passes, no existing test changed | HOLDS. exit 0: `ok .../internal/job/lint 0.257s`, `ok .../internal/contract 0.934s`. `git diff --stat HEAD~1` of the code commit lists only `internal/contract/ft_hint_test.go`, `internal/contract/hint.go`, `internal/job/lint/json.go` |
| 4. `git grep -n 'value outside enum' -- internal/contract/schema.go` | HOLDS. exit 0: `internal/contract/schema.go:212:  return fail("value outside enum")`. schema.go not touched |
| New tests fail on base | HOLDS. With the code stashed, the test file alone: `internal/contract/ft_hint_test.go:95:21: undefined: contract.Hint` (8 such lines), `FAIL .../internal/contract [build failed]` |
| Packages importing the changed ones | `go list` direct importers of `internal/job/lint`: `internal/cli`, `internal/job/jobcmd`. `go test ./internal/cli/ ./internal/job/jobcmd/ -count=1` exit 0 (`ok cli 38.899s`, `ok jobcmd 1.785s`). Direct importers of `internal/contract` (26 packages: artifact brain braincmd cli feed guard hook intent job jobcmd lint jobgraph land lease mcp migrate orchestrator question session settings signal task usage where workflow) get only an added function; the whole-module run is the orchestrator's |
| Lint | HOLDS. `make lint` (go vet ./...) exit 0; `gofmt -l internal/contract internal/job/lint` prints nothing |
| Windows and macOS builds | HOLDS. `GOOS=windows GOARCH=amd64 go build ./...` exit 0; `GOOS=darwin GOARCH=arm64 go build ./...` exit 0 |
| *RealPlugin* tests | Not needed: no test of this lane is named RealPlugin and the lane changes no plugin behaviour outside lint/contract |
| No guard/gate/cap on agents, nothing removed, no existing test edited | HOLDS. The change only replaces a fix-line string; the generic line stays as the fallback |
| No attribution lines in commits | HOLDS. The code commit message is one sentence with no trailer |

## What changed

- `internal/contract/hint.go`: `Hint(name string, err error) string`. It reads the last `$path: rule` in the Validate message (a oneOf/anyOf message nests the furthest branch), walks the named schema along the path (properties, additionalProperties, items, through `$ref`, allOf/anyOf/oneOf and `then`), and returns:
  - enum: `Set $.visible to one of: big, small.` (values in schema file order, unioned across branches)
  - unknown property: `Remove "priority" from the record; the known properties are: ...` (sorted; branches that set `additionalProperties:false` decide when there are any)
  - missing required: `Add the required field "goal" to $.intent.` (`the record` at the top)
  - `""` for any other rule, an unknown schema, a nil error or a path the schema does not reach.
- `internal/job/lint/json.go:26-31`: the hint is the fix line when not empty, else the old generic line.
- `internal/contract/ft_hint_test.go`: the six `TestFtHint*` tests, external package `contract_test` so it can call lint without an import cycle.
- The Validate messages are unchanged.

## NOT VERIFIED on a live screen

- None: no UI. The hint text is checked in tests only; an agent reading `grid job lint` output on a live run is not exercised here.
- A `oneOf` over bodies with different enums would list the union of the branch values; no job-record path does this today.
