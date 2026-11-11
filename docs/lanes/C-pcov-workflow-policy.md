# C-pcov-workflow-policy: tests for the workflow policy reader

Test commit: 88522270 (one new file, `internal/workflow/policy_read_test.go`). No non-test file changed.

## Proof table

| # | Row / rule | Result |
|---|---|---|
| 1 | ReadPolicy defaults; Read has empty Policy | HOLDS. `TestReadPolicyDefaults` (policy_read_test.go:57), `go test ./internal/workflow/ -run ReadPolicy -v` exit 0. One entry, Project nil, four default keys all Source "default" (refusal_message "plugin"; inbox_act_roles = all 7 acts -> [owner member local agent]; the two *_roles -> [owner member]). `Read` on the same home: Policy empty. |
| 2 | Workspace override, project inherits, project override isolated | HOLDS. `TestReadPolicyWorkspaceBaseAndProjectOverride` (:88). Workspace and alpha/beta/gamma show `task_undo_roles [owner]` and `inbox_act_roles {note:[owner]}` Source "workflow" From "workspace"; beta's `project_document_write_roles: [owner]` is From "project"; gamma (after beta) and alpha still show the default document roles, not beta's. Exit 0. |
| 3 | inbox_act_roles replaced as a whole | HOLDS. `TestReadPolicyInboxActsReplacedAsAWhole` (:122). Value is exactly `{snooze:[owner], wake:[agent local]}`; turn, add, note, dismiss, reopen are absent. Finding for the owner below. Exit 0. |
| 4 | Invalid cases | HOLDS. `TestReadPolicyInvalidFailsWholeAnswer` (:141): 13 cases x {workspace, project} = 26 subtests (inbox not a map, unknown act, role outside set, roles not a list, role not a string; document roles not a list / local / agent; undo roles not a list / guest / local; front matter not closed; front matter bad YAML). Each: `errors.Is(err, ErrPolicy)`, text exactly `invalid <key> in <level> workflow`, zero Answer. Plus `TestReadPolicyUnreadableFileFailsAsFrontMatter` (:189): a directory where workflows.md should be gives `invalid front matter in project workflow`, zero Answer. Exit 0. |
| 5 | CRLF same as LF | HOLDS. `TestReadPolicyCRLFFrontMatter` (:208): the row-2 workspace file with \r\n gives a Policy DeepEqual to LF. Exit 0. |
| 6 | problemError | HOLDS. `TestProblemErrorKeepsCause` (:219): with cause, `errors.Is(err, cause)`, `errors.Unwrap(err) == cause`, text `the workflow cannot be resolved: ` + Problem text; without cause, same text and `errors.Unwrap(err) == nil`. Exit 0. |
| 7a | `go test ./internal/workflow/ -count=1`: 0 fail | HOLDS, exit 0 (`ok ... 3.285s`, run with the importers below). |
| 7b | Coverage before / after | HOLDS. Before (untouched clone, linux): `ok  github.com/gridconsole/grid-plugins/internal/workflow 2.729s coverage: 85.1% of statements`. After: `coverage: 91.5% of statements`. Per function (`go tool cover -func`): policyDefaults 100.0, invalidPolicy 100.0, effectivePolicy 100.0, policyRoles 100.0, appPolicyValue 82.4 (needs >75), ReadPolicy 100.0, policyError.Error 100.0, Unwrap 100.0, problemError 100.0, resolveError.Error 100.0, resolveError.Unwrap 100.0. |
| 7c | Windows and macOS test builds | HOLDS. `GOOS=windows go test -c -o /dev/null ./internal/workflow/` exit 0; `GOOS=darwin go test -c -o /dev/null ./internal/workflow/` exit 0. Compiled only: "compiled, needs Windows run" and "compiled, needs macOS run" with those same commands (`go test ./internal/workflow/ -run 'ReadPolicy|ProblemError'`). |
| 7d | Lint | HOLDS. `make lint` (runs `go vet ./...`) exit 0; `gofmt -l internal/workflow/` empty. |
| R1 | Importers' tests | HOLDS. Importers from `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...`: internal/feed, internal/task, internal/cli. `go test ./internal/workflow/ ./internal/feed/ ./internal/task/ ./internal/cli/ -count=1` exit 0, all four `ok`. |
| R2 | New tests fail on base | HOLDS by mutation (these are tests of existing code, so they pass on unmodified base). Mutation: policy.go `case "project_document_write_roles", "task_undo_roles"` allowed list widened to include local and agent. Failing lines: `--- FAIL: TestReadPolicyInvalidFailsWholeAnswer/workspace/document_role_local ... policy_read_test.go:175: err = <nil>, want ErrPolicy`, same for `workspace/document_role_agent`, `workspace/undo_role_local`, and the three project/ twins (6 subtests). Mutation reverted with `git checkout`; tree clean. |
| R3 | No existing test edited, no deletions | HOLDS. `git status` showed only `?? internal/workflow/policy_read_test.go`; the shared helper putWorkflowFixture is reused and untouched. |
| R4 | No guard, gate, cap or hold added | HOLDS. Tests only. |
| R5 | No personal paths, hosts or emails in the public repo | HOLDS. Test uses t.TempDir() only. |
| R6 | No attribution lines in the commit | HOLDS. One-sentence message, no trailers. |
| R7 | refusal_message only default asserted | HOLDS. No override or invalid-value case for it. |

## What changed

New file `internal/workflow/policy_read_test.go`. It builds its own homes with `putWorkflowFixture` (files_test.go) and calls `ReadPolicy`, `Read`, `problemError` and the `ErrPolicy` chain. Existing fixtures were reused, not edited. I did not use `hardenHome` or `writeTaskPolicy`: `hardenHome` is a fixed alpha-only home and `writeTaskPolicy` writes task-local policy, not the workspace/project files the reader walks.

## Finding for the owner

Overriding one act in `inbox_act_roles` drops every other act instead of leaving them at their default. `appPolicyValue` (policy.go:104-120) builds the map from the named acts only, and `effectivePolicy` stores it as the whole key (policy.go:88). So `inbox_act_roles: {snooze: [owner]}` leaves turn, add, note, dismiss, wake and reopen absent from the effective policy. How a missing act is treated is up to the consumer, which is not in this lane. Pinned by `TestReadPolicyInboxActsReplacedAsAWhole`. Not fixed, as the brief says. If the owner changes it to merge over the defaults, that test is the one to rewrite.

## NOT VERIFIED on a live screen

Nothing in this lane has a UI. `grid workflow show --policy` (internal/cli/workflow.go:57) was not run against a live home; the tests call `ReadPolicy` directly. Windows and macOS runs of the new tests were not done (compiled only).
