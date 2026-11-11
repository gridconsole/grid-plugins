# C-iy-readme-schema-walk

Base: 31824184686c6f0028cd6524815394a5eb9878d9 (lane-Pint103 merge tip; README table was at :137-154, not :136-182, so the row sits after `Question and gate` at :147). On base, `question-file.json` was the only schema linked from neither README.md nor formats.md.

| Row | Result |
| --- | --- |
| 1 base fails naming only question-file.json | HOLDS. Before the row: `iy_schema_rows_test.go:47: schema question-file.json has no row in the contract docs` / `--- FAIL: TestIySchemaRows`, exit 1. |
| 1 passes after the row | HOLDS. `go test ./internal/contract -count=1 -run 'TestIySchema' -v` exit 0: `--- PASS: TestIySchemaRows`, `--- PASS: TestIySchemaRowsNegative`, no SKIP. |
| 2 negative case | HOLDS. TestIySchemaRowsNegative (t.TempDir): reports `unlinked.json` (in neither doc) and `fmt-x.json` (linked only from README text); `fmt-y.json` linked from formats text is not reported. |
| 3 TestGbDocs / TestHfFs | HOLDS. `go test ./internal/cli -count=1 -run 'TestGbDocs\|TestHfFs'` exit 0, `ok internal/cli`. |
| 4 make lint | HOLDS. `make lint` (go vet ./...) exit 0. Diff names contract/README.md, the new test and this report only. |
| Tests of changed packages and importers | HOLDS. Only a `contract_test` file and a markdown row changed, no Go package code. `go test ./internal/contract -count=1` exit 0. Importers need no rerun; no code changed. |
| Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No guard/gate/cap | HOLDS. None added. |
| Attribution | HOLDS. No attribution lines in the commit. |
| RealPlugin tests | Not applicable. |

## What changed
- contract/README.md: one row `Task question file | question-file.json | internal/question/question.go:27; internal/question/files.go:145`.
- internal/contract/iy_schema_rows_test.go: TestIySchemaRows (reads contract/schemas with os.ReadDir; `fmt-` files must be linked from formats.md, others from README.md) and TestIySchemaRowsNegative; helper `iySchemaUnlinked`.

## NOT VERIFIED on a live screen
Nothing; no UI.
