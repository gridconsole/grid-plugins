# C-gg-cli-person-refusal-code

Base: b87fc36458332a9e2bd680ff9e3ed5a896638900 (lane branch point).

| Row | Result |
|---|---|
| 1 TestGgPersonRefusalCheckHasCode | HOLDS. Fails on base (see below), passes after: `go test -count=1 -run TestGgPersonRefusal ./internal/cli/` exit 0. File internal/cli/gg_person_refusal_code_test.go:68-80. |
| 2 TestGgPersonRefusalRoleHasCode | HOLDS. Fails on base, passes after, same command. Test file :82-92. |
| 3 plain run byte-equal stderr, exit 3, empty stdout | HOLDS before and after: asserted in ggPersonCheckRefusal (test file :23-42); the base failures were only on `code`. |
| 4 diff names only the three files | HOLDS (plus this report, required by the runbook): test file, role_policy.go:16-33, person_check.go:57-68. |
| 4 targeted go test | HOLDS, exit 0: TestGgPersonRefusalCheckHasCode, TestGgPersonRefusalRoleHasCode, TestPersonCheckAnswersPerson, TestPersonCheckRefusesAgent, TestPersonCheckUsage, TestSetupPreviewHonoursWorkflowRoleList, TestPersonAnswerScopeAndWorkflowRoles, TestWorkflowRoleListIgnoresAgentEnvironment, TestRoleRefusalNamesAllowedRoles all PASS. |
| 4 make lint | HOLDS, exit 0 (go vet ./...); gofmt -l internal/cli empty. |
| Packages that import internal/cli (go list) | cmd/grid (no test files), lib: `go test -count=1 ./cmd/grid/ ./lib/` ok. Full `go test -count=1 ./internal/cli/` ok. |
| Windows and macOS builds | HOLDS: `GOOS=windows go build ./...` exit 0, `GOOS=darwin go build ./...` exit 0. |
| No guard/gate/cap added; nothing removed | HOLDS: only two fields added to two structs. |
| No real home | HOLDS: ggPersonIsolate sets HOME, CLAUDE_CONFIG_DIR, CODEX_HOME, GRID_HOME under the temp root and t.Fatal otherwise; no child process is started. |
| *RealPlugin* tests | None apply. |

## Failing lines on base (test only, before the product change)

    --- FAIL: TestGgPersonRefusalCheckHasCode/worker_role
        gg_person_refusal_code_test.go:77: code "", want person_required (object map[error:person_required hint:approvals need a person; this caller is an agent reason:agent_role])
    --- FAIL: TestGgPersonRefusalRoleHasCode
        gg_person_refusal_code_test.go:91: code "", want person_required (object map[error:person_required hint:your role is not in this workflow's list for this command; allowed: member, lead reason:role_not_allowed])

(All five check subtests failed the same way.)

## What changed

Both refusal structs gain `code` ("person_required") and `message` after `hint`. The stderr line is formatted once and used for both stderr and `message` (without final newline). `error`, `reason`, `hint`, their order, stderr bytes and exit 3 are unchanged.

## NOT VERIFIED on a live screen

Nothing here is a UI row. The app's read of these fields (plugincli/person.go) was not run.
