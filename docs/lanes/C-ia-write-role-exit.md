# C-ia-write-role-exit: `grid workflow write --json` tells a role refusal from a changed file

Base: origin/main ee8a96c1 (contains lane-C-hf-cli-failure-shapes 161396e7: `git merge-base --is-ancestor origin/lane-C-hf-cli-failure-shapes origin/main` exit 0).
Base check: on base (with only the `CodeWorkflowFileChanged` declaration added so the new file compiles) `workflow write --rev 000000000000 --json` and `--rev none --json` over an existing workflows.md print nothing on stdout (TestIaWrChanged failing lines below); a refused role prints nothing on stdout (TestIaWrRoleRefusal failing lines below).

## Proof table

| Row | Status | Proof |
| --- | --- | --- |
| 1 TestIaWrRoleRefusal | HOLDS | `go test ./internal/cli -count=1 -run 'TestWorkflowWrite\|TestGuestProjectParity\|TestStableErrorCodes\|TestErrorCodeIgnoresWording\|TestGmWorkflow\|TestHfFs\|TestIaWr' -v` exit 0: `--- PASS: TestIaWrRoleRefusal`. Test at internal/cli/ia_workflow_write_test.go:39. Roles guest, agent, unknown; exit 3; stderr == main's line + `\n` == plain stderr; stdout has exactly the five keys and values; plain stdout empty; Grid home empty. On base: `ia_workflow_write_test.go:56: json stdout is not one object: unexpected end of JSON input: ""` (x3) |
| 2 TestIaWrChanged | HOLDS | same run: `--- PASS: TestIaWrChanged`. Test at ia_workflow_write_test.go:80. `--rev 000000000000` and `--rev none` over an existing workflows.md: exit 3, stderr contains `workflow changed since read` and equals plain stderr, `assertErrorOutput(t, out, errb, CodeWorkflowFileChanged)`, plain stdout empty, file unchanged. On base: `ia_workflow_write_test.go:94: error JSON: unexpected end of JSON input:` (x2) |
| 3 TestIaWrReadme | HOLDS | same run: `--- PASS: TestIaWrReadme`. Test at ia_workflow_write_test.go:119; finds the table by `\| Stable code \| Meaning \|` and the section by its `### \`grid workflow write ` heading up to the next `### `. On base: `ia_workflow_write_test.go:136: stable-code table has 0 workflow_file_changed rows, want 1` |
| 4 pins move | HOLDS | guest_parity_test.go:436-438 json-mode stdout is the person_required object; workflow_write_test.go:95-97 exit-3 rows expect CodeWorkflowFileChanged; workflow_write_test.go:250-253 the loser's stdout is the envelope (assertErrorOutput). Row-4 cli run above exit 0 (all of TestWorkflowWrite*, TestGuestProjectParity, TestStableErrorCodes, TestErrorCodeIgnoresWording, TestGmWorkflowUnknownProject, TestHfFs*, TestIaWr* PASS); `go test ./internal/contract -count=1 -run 'TestReceipts' -v` exit 0 (3 PASS) |
| 5 lint, cross, scope | HOLDS | `make lint && make cross` exit 0 (go vet; darwin/arm64 and windows/amd64 vet + test compile all ok). `git diff origin/main --stat`: contract/README.md, internal/cli/error_codes.go, guest_parity_test.go, ia_workflow_write_test.go, workflow.go, workflow_write_test.go, plus this report. commands.json untouched; `make generate && git status --short` exit 0 with empty status (no generated file differs). As seen: `git diff origin/lane-C-hf-cli-failure-shapes --stat` gives 151 files because main merged other lanes after hf; the base origin/main already contains 161396e7, so the diff from the base is the scope proof |
| Rule: new tests fail on base, pass on lane | HOLDS | failing lines in rows 1-3 (base behaviour plus the constant declaration only, so the file compiles); PASS lines above |
| Rule: no existing test edited to pass except the pins the brief names | HOLDS | only workflow_write_test.go:95-97, :250-253 and guest_parity_test.go:436-438, all named in Touches |
| Rule: no guard, gate, cap, hold | HOLDS | which roles may write is unchanged (workflow.WriteDefaultAllowed untouched); only stdout with `--json` changes |
| Rule: tests of changed and importing packages | HOLDS | importers via `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep "grid-plugins/internal/cli\b"`: cmd/grid, contract/formats, internal/account, internal/cli, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test ./internal/cli/... -count=1` exit 0; `go test -count=1 ./cmd/grid ./contract/formats ./internal/account ./internal/golden ./internal/jobgraph ./internal/mcp ./internal/packaging ./internal/question ./internal/settings ./lib ./internal/contract` exit 0 (all ok) |
| Rule: gofmt | HOLDS | `gofmt -l internal/cli` printed nothing |
| Rule: no attribution, no personal paths | HOLDS | commit message is one sentence, no trailer |
| Rule: RealPlugin tests | n/a | no row names a RealPlugin test |
| Rule: UI rows | n/a | no UI change |

## What changed

- internal/cli/workflow.go:133-147: the host-policy role refusal keeps main's stderr line and exit 3; with `--json` it prints `{"error":"person_required","reason":"role_not_allowed","hint":"caller role is not allowed to write workflows","code":"person_required","message":"grid workflow write: caller role is not allowed to write workflows"}`. roleRefusal is not called.
- internal/cli/error_codes.go: `CodeWorkflowFileChanged = "workflow_file_changed"`; errorCode returns it for `errors.Is(err, workflow.ErrChanged)` (only grid workflow write returns ErrChanged), so finish writes `{"error":{"code":"workflow_file_changed","message":"grid workflow write: workflow changed since read"}}` with `--json`. Exit 3, stderr unchanged.
- contract/README.md: table row `workflow_file_changed` after `invalid_policy`; "The first six constants"; the `grid workflow write` section names the envelope and the person gate object. No `` `workflow_changed` ``.
- Invalid input (exit 2, `invalid_argument` envelope), plain output, successful JSON and the allowed roles are unchanged.

## Open items

- contract/README.md:84-86 lists the files that write `person_required` as a literal (role_policy.go, person_check.go, task_write.go); workflow.go now also writes it. The sentence does not say "only", the code is already in the table, and TestHfFsCodeTable scans only those three files, so the line was left as the brief scoped it. Backlog note.
- The app maps every exit 3 of this verb to 409 `workflow_changed` (app internal/slices/workflows/write.go:126-128); telling the two apart in the app is a later app lane, as the brief states.

## NOT VERIFIED

Nothing needs a live screen.

DONE
