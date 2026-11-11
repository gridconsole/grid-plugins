# C-ef-gate-roles-brain-workflow

Base: bb3c134081ecb6ec65f02a5ad567337a8e86ec74. Base check: `internal/workflow/gates.go:18-27` read only `readTaskPolicy` and returned `defaultGatePostRoles()` on ErrNotExist, so no brain fallback existed.

## Proof table

| Row | Result |
|---|---|
| 1 project note, task without workflow.md: person=[agent], rest default | HOLDS. `TestGatePostRolesProjectNote` (internal/workflow/gates_brain_test.go). `go test ./internal/workflow/...` exit 0. |
| 2 task workflow.md wins; home note answers project and personal tasks | HOLDS. `TestGatePostRolesTaskWorkflowWins`, `TestGatePostRolesHomeNote`, `TestGatePostRolesProjectBeatsHome`. exit 0. |
| 3 no note / taskDir outside layout = defaultGatePostRoles(); gates_test.go unchanged | HOLDS. `TestGatePostRolesDefaultsWithoutNote`, `TestGatePostRolesOutsideLayoutIgnoresNotes`; existing `TestGatePostRoles` passes, gates_test.go not edited (`git diff main --stat`). exit 0. |
| 4 broken front matter, bad roles, two notes in project fall through, nil error | HOLDS. `TestGatePostRolesBrokenOrDoubledProjectNoteFallsThrough` (3 subtests: default with no home note, home note with one). exit 0. A broken task workflow.md still errors: `TestGatePostRolesBrokenTaskWorkflowStillErrors`. |
| 5 feed Store refuses owner person gate, takes agent | HOLDS. `TestGatePostRolesFromBrainNote` (internal/feed/gate_roles_brain_test.go). `go test ./internal/feed/...` exit 0. |
| 6 `go test ./internal/workflow/... ./internal/feed/...`, `make lint`, diff only Touches | HOLDS. Tests exit 0; `make lint` (go vet ./...) exit 0; `git diff main --stat` names gates.go, gates_brain_test.go, gate_roles_brain_test.go, plus this report under docs/lanes/. |
| New tests fail on base, pass on lane | HOLDS. With gates.go stashed to base: `--- FAIL: TestGatePostRolesProjectNote ... roles = map[... person:[cli owner member lead] ...], want ... person:[agent]`; also `--- FAIL: TestGatePostRolesHomeNote`, `...ProjectBeatsHome`, `...BrokenOrDoubledProjectNoteFallsThrough`; feed: `gate_roles_brain_test.go:35: owner person gate was not refused: <nil>`. Pass on lane. |
| No existing test edited or deleted | HOLDS. Only new files and gates.go changed. |
| Importer tests | HOLDS, exit 0: workflow, feed, task, cli, hook, lease, mcp, orchestrator, question, usage, migrate, approvefix, job/..., ports. These are the packages `go list` shows importing workflow, plus the ones that import feed. |
| Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. gofmt -l clean. |
| No guard/gate/cap on agents; no personal paths; no attribution lines | HOLDS. Change only widens where a policy can come from; commit message has no attribution. |

## What changed

`GatePostRoles` (internal/workflow/gates.go): when the task's workflow.md does not exist, `brainGatePostRoles` finds home and project from the task folder (`projects/<p>/tasks/T-NN...` or `personal/tasks/T-NN...`; taskHome mirrors where.taskParts, which stays unexported). It then calls `Answer.fromBrain` for the project level, then the home level, the same workflowNotes and gatePostRoles code `ReadFor` uses. Broken, unreadable or doubled notes are skipped and their Problems dropped; with no usable note the default answers. A broken workflow.md still returns its error. The doc comment names the order.

Note for the reviewer: `ReadFor` also checks the caller (guest, personal-project access) through a Resolver. `GatePostRoles(taskDir)` has no caller, so it reads the notes without that check; the feed callers (store.go:189, questions.go:253) are unchanged.

## NOT VERIFIED on a live screen

Nothing here is UI. No live run of the grid CLI was made; all proof is go test.

Result: DONE
