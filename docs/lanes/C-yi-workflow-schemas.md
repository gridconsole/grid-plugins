# C-yi-workflow-schemas: output schemas for `grid workflow show` and `grid workflow write --json`

Base: plugin main `103b9157`. Lane branch `lane-C-yi-workflow-schemas`.

## Base check

Neither command had an `output:` annotation on base: `internal/cli/contract_inventory_test.go:582-583` read
`"workflow show": {}` and `"workflow write": {required: ...}` with no `output`, `contract/commands.json` had
`"output": ""` for both, and no `cmd-workflow-*` schema existed. Show's answer is written at
`internal/cli/workflow.go:74` (`writeJSON(env.Stdout, answer)`, answer from `read(...)` at :59) and write's at :176.
(The brief's :60-76 and :170-178 are two lines off; the writers are the lines above.)

## Shape choice

One schema, `cmd-workflow-show`, for show with and without `--policy`: one object whose `policy` array is optional.
`policy` is present exactly when `--policy` is given (one entry per `files` entry). A `oneOf` would repeat the whole
template object, because the schema subset has no local `$defs` and a second `cmd-` schema file is not in Touches.
`TestYiWorkflowShowSchema` asserts that only the `--policy` answers carry `policy`. `source`, `note` and
`gate_post_roles` (set only by `workflow.ReadFor`, never by `workflow show`) are not in the schema.
`steps[].approvers` is `array|null` because the plugin writes `null` for a step that is not an approve step.

## Proof table

| # | Row / rule | Result | Proof |
|---|---|---|---|
| 1 | Red on base, then green | HOLDS | Base (schemas absent): `yi_workflow_schema_test.go:236: unknown schema "cmd-workflow-show"`, `:279: unknown schema "cmd-workflow-write"`, `:351: the unrenamed stdout must pass: unknown schema "cmd-workflow-show"`, then `--- FAIL: TestYiWorkflowShowSchema`, `--- FAIL: TestYiWorkflowWriteSchema`. Lane: `go test -count=1 -v -run 'TestYi' ./internal/cli/` exit 0 prints `--- PASS: TestYiWorkflowShowSchema/no-workflows`, `/project`, `/project-policy`, `/project-errors`, `/project-policy-front-matter`, `--- PASS: TestYiWorkflowWriteSchema/project-new` (`--rev none`), `/project-replace`, `/workspace`, `--- PASS: TestYiWorkflowRequiredFields`. |
| 2 | Required holds the app's fields | HOLDS | `--- PASS: TestYiWorkflowRequiredFields/cmd-workflow-show/templates.steps`, `.../errors`, `.../errors.section`, `.../errors.message`, `.../templates.file`, `.../templates.steps.name\|kind\|agent\|sizes`, `cmd-workflow-write/rev`, `cmd-workflow-write/file` (each case first validates the unrenamed stdout, then the renamed copy must fail). Also renamed: templates.name, default, rounds, overrides, openTasks, applies, policy, policy.keys and its value/source. |
| 3 | Fixtures, unknown fields, inventory | HOLDS | `go test -count=1 -v -run 'TestFixtures\|TestContractCommandsRejectUnknownFields\|TestContractCommandsParserInventory' ./internal/contract/ ./internal/cli/`: `--- PASS: TestFixtures` (5 valid + 2 invalid show, 3 valid + 2 invalid write), `--- PASS: TestContractCommandsRejectUnknownFields` (`/cmd-workflow-show`, `/cmd-workflow-write`), `--- PASS: TestContractCommandsParserInventory`. |
| 4 | No product change | HOLDS | `git diff main --stat`: 17 files, all in Touches (two schemas, two fixture folders, `contract_inventory_test.go`, `commands.json`, `yi_workflow_schema_test.go`). `git diff main -- contract/commands.json` adds only `"output": "cmd-workflow-show.json"` and `"output": "cmd-workflow-write.json"` (`"output": ""` before). |
| 5a | `go test -count=1 ./internal/cli/ ./internal/contract/` 0 fail | NOT DONE for `internal/contract` | `internal/cli`: `ok ... 195.428s`. `internal/contract`: **FAIL `TestIySchemaRows`** (`iy_schema_rows_test.go:47: schema cmd-workflow-show.json has no row in the contract docs`, same for `cmd-workflow-write.json`). The test wants a link `](schemas/<name>)` in `contract/README.md`, a file the brief's Touches does not list; I did not edit it. Fix: add after the `grid task show output` row (README.md:197) these two rows, then the test passes: `\| grid workflow show output \| [cmd-workflow-show.json](schemas/cmd-workflow-show.json) \| internal/cli/workflow.go:74 \|` and `\| grid workflow write output \| [cmd-workflow-write.json](schemas/cmd-workflow-write.json) \| internal/cli/workflow.go:176 \|`. The brief said a new schema file needs no registration; this README rule (lane iy) is the exception. |
| 5b | `make lint` | HOLDS | `make lint > /dev/null 2>&1 && echo lint-exit-0` printed `lint-exit-0`. |
| 5c | `make cross` (Windows, macOS) | HOLDS | `make cross` exit 0 (background task reported completed, exit code 0). |
| 6 | Importers of the changed packages | see 7 | `go list -f '{{.ImportPath}}: {{join .Deps " "}}' ./...` lists the packages that depend on the embedded `contract` data (cmd/grid, internal/{account,approvefix,artifact,brain,braincmd,cli,contract,doctor,env,feed,guard,hook,intent,job,job/jobcmd,job/jobtest,job/lint,jobgraph,land,lease,mcp,migrate,orchestrator,packaging,packaging/cmd,people,ports,projectenv,projectmd,...} first 30 of the list). No production Go file changed; only embedded data was added, and `commands.json` gained two `output` values. |
| 7 | Module-wide run in this clone | see report | `go test -count=1 ./...` was run once in the background; its result is in the report. |
| 8 | Plugin walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` `ok`; `go test -count=1 ./internal/job/lint/` `ok`. (App walk tests do not apply to the plugin repo.) |
| 9 | `*RealPlugin*` tests | n/a | No row names one. |
| 10 | No guard, gate, cap or hold | HOLDS | Only schemas, fixtures and tests; nothing on agents. |
| 11 | No deleted or edited existing test | HOLDS | Only `contract_inventory_test.go:582-583` changed (the `output:` values the brief names). |

## App fields checked against the plugin (app 8f9b806d, read with `git show`)

- Judge (`internal/kernel/worker/judge.go:430-447`): `templates[].name|default|rounds|file|overrides|steps[]` and `errors[]` are all written by `workflow show`. `workflow` and `owner` there are read from `grid task show`, not `workflow show`, and are covered by `cmd-task-show`.
- Task page (`internal/slices/taskview/view_steps.go:51-70`): `templates[].name|default|steps|openTasks|applies|file|overrides`, `errors[].section|message`, `steps[].name|kind|sizes|agent` are all present.
- Write policy (`cmd/grid-server/write_policy.go:139-143`, `workflowPolicyEntry` :275): needs exactly `files`, `templates`, `errors`, `policy` with `policy[].project` and `policy[].keys` holding `refusal_message` and `inbox_act_roles`; the plugin writes all four keys, each `{value, source[, from]}`.
- Workflows page (`internal/slices/workflows/write.go:138-148`): `file` and `rev` of write's answer. Its `scope`/`files` at :170-181 are the app's own request body, not plugin output.

## What changed

- `contract/schemas/cmd-workflow-show.json`, `cmd-workflow-write.json` (new).
- `contract/fixtures/cmd-workflow-show/{valid,invalid}`, `cmd-workflow-write/{valid,invalid}`: valid ones are captured stdout (project made with `testgit.Init` + `project add`, file stored by `workflow write --rev none`, replaced with the stored rev, a hand-appended broken section for `errors[]`, a hand-written front-matter file for `policy` with `source: workflow`), invalid ones are those with one required field renamed or a bad `rev`.
- `internal/cli/contract_inventory_test.go`: the two `output:` values; `contract/commands.json` regenerated by `make generate`.
- `internal/cli/yi_workflow_schema_test.go` (new): `TestYiWorkflowShowSchema`, `TestYiWorkflowWriteSchema`, `TestYiWorkflowRequiredFields`. Each valid fixture must equal the live answer; `UPDATE_YI_WORKFLOW_FIXTURES=1` rewrites them (pass it as a make variable: `make test UPDATE_YI_WORKFLOW_FIXTURES=1 "GOFLAGS=-count=1 -run=TestYiWorkflow(Show|Write)Schema"`).

## NOT VERIFIED on a live screen

Nothing here has a screen. The app decoders were checked by reading their source at app 8f9b806d, not by running the app.
