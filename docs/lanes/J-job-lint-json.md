# J-job-lint-json: `grid job lint` checks a JSON job record

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. A `.json` file, or content starting with `{`, goes to CheckJSON: job-record schema first, then the `c.validate` rules | HOLDS | `internal/job/lint/json.go:13` (IsJSON: suffix or first non-space byte), `json.go:24-39` (contract.Validate("job-record") then `c.validate`), `internal/job/jobcmd/command.go:203-204` (routing). `TestIsJSON`, and `TestLintJobRecordJSON` covers a `.json` file and a `{` file with no suffix. `go test ./internal/job/lint/ ./internal/job/jobcmd/` exit 0 |
| 2. A valid record lints clean, exit 0; a record missing a required field gives the schema error with the field path, exit non-zero | HOLDS | `TestCheckJSONValidRecordIsClean`, `TestCheckJSONMissingRequiredFieldNamesThePath`, and the CLI test `TestLintJobRecordJSON` (exit 0 for the good record, exit 1 for the record without `title`, rule `job.record`, message names `title`). Exit 0 |
| 3. A markdown brief lints as before | HOLDS | `TestLintJobRecordJSON` lints the rendered brief at the end (exit 0, no findings). The markdown path is untouched; `TestRenderLintShow`, `TestGoldenBriefPasses` and the broken-brief test still pass unedited |
| 4. `go test ./internal/job/lint/ ./internal/job/jobcmd/`: 0 fail | HOLDS | `go test ./internal/job/lint/ ./internal/job/jobcmd/` exit 0 (`ok` for both) |
| New tests fail on base | HOLDS | With `command.go` reverted: `--- FAIL: TestLintJobRecordJSON ... lint .../job.json --task T-01 --json: exit 1 want 0; stdout {"ok":false,...,"rule":"brief.frontmatter","message":"line 1: missing frontmatter block"}`. The lint tests (`json_test.go`) do not compile on base because `CheckJSON` and `IsJSON` do not exist |
| Existing tests not edited, none deleted | HOLDS | `git diff origin/main..HEAD --stat` shows only added files plus the lint case in `command.go` |
| Packages that import the changed ones | HOLDS | `go list` importers of `internal/job/lint` and `internal/job/jobcmd`: `internal/cli`, `internal/job/jobcmd`, `internal/job/lint`. `go test ./internal/cli/ ./internal/job/jobcmd/ ./internal/job/lint/` exit 0 |
| gofmt, vet | HOLDS | `gofmt -l internal/job` empty; `go vet ./internal/job/... ./internal/cli/` exit 0 |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| No guard, gate, cap or hold on agents | HOLDS | The change only adds a check path in a lint command; nothing blocks an agent |
| Only the named files edited; no personal paths, hosts or emails | HOLDS | Files: `internal/job/jobcmd/command.go` (lint case), `internal/job/lint/json.go`, `internal/job/lint/json_test.go`, `internal/job/jobcmd/lint_json_test.go`, this report. `lint.go` untouched |
| Whole-module test, cross-repo set | NOT RUN | The orchestrator runs these at landing |

## What changed

- `lint.IsJSON(name, text)` and `lint.CheckJSON(data, opts)` in the new `internal/job/lint/json.go`. CheckJSON runs `contract.Validate("job-record")` and, when that fails, returns one `job.record` finding whose message is the schema error with its field path. A record that passes is decoded into `job.Job`, given its defaults, and checked by the same `validate` rules the markdown path runs.
- The `lint` case in `jobcmd/command.go` picks CheckJSON when the input is JSON and `lint.Check` otherwise.
- Tests: `internal/job/lint/json_test.go` and `internal/job/jobcmd/lint_json_test.go`. The record fixture is the golden brief or the rendered brief, projected with `Job.Record()`.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: a record posted with `grid job post --body`, then linted from the real orchestrator output.
