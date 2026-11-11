# C-gm-workflow-unknown-project-envelope

`workflow show` and `workflow write` give the `no_such_project` envelope for an unknown project.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. New test FAILs on base, PASSes on the lane; exit 1 plain and `--json`, stderr byte-equal, stdout with `--json` is the `no_such_project` envelope | HOLDS | Base (new test file only): `go test -count=1 -v -run TestGmWorkflowUnknownProject ./internal/cli/` printed `gm_workflow_envelope_test.go:64: error JSON: unexpected end of JSON input:` for `show/json`, `show-policy/json` and `write/json`, then `--- FAIL: TestGmWorkflowUnknownProject`, `FAIL ... internal/cli`. Lane: same command, `--- PASS` for `show/plain`, `show/json`, `show-policy/json`, `write/plain`, `write/json`, `ok ... internal/cli`. The check is `assertErrorOutput` (gm_workflow_envelope_test.go:64) plus the exit and stderr compare at :60-62. One sub-case cannot be run: `show --policy` without `--json` is exit 2 at `internal/cli/workflow.go:41`, before the project lookup, so `show-policy` runs `--json` only (`jsonOnly`, gm_workflow_envelope_test.go:36 and :43). Row 1 holds for the five cases that reach the lookup. |
| 2. HOME, GRID_HOME, CLAUDE_CONFIG_DIR, CODEX_HOME set with t.Setenv under t.TempDir(); t.Fatal when HOME is not under it | HOLDS | gm_workflow_envelope_test.go:15-26 (`t.Setenv` at :21, the HOME check at :23). |
| 3. `ggProjectKnownOpen` holds no `workflow show` or `workflow write`; `TestGgUnknownProject` PASSes | HOLDS | Both lines and the blank line above them removed (gg_unknown_project_walk_test.go, was :73-75). `go test -count=1 -v -run 'TestGmWorkflowUnknownProject\|TestGgUnknownProject' ./internal/cli/`: `--- PASS: TestGgUnknownProjectWalkCoversTheContract`, `--- PASS: TestGgUnknownProjectWalk`, log `known open: 34 of 81` (was 36). |
| 4. `go test -count=1 ./internal/workflow/ ./internal/projects/ ./internal/cli/` exit 0 | HOLDS | `ok .../internal/workflow 4.611s`, `ok .../internal/projects 2.244s`, `ok .../internal/cli 62.124s`. The message tests named in the brief pass unchanged. `TestGuestProjectParity`: `--- PASS` (`-run TestGuestProjectParity ./internal/cli/`, `ok`). This needed two more edited assertions than the brief names (row 5a). |
| 4a. Tests of every package that imports workflow or cli | HOLDS | `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./...` filtered on `internal/workflow` and `internal/cli` gave: cmd/grid, approvefix, braincmd, cli, env, feed, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, packaging, packaging/cmd, ports, question, signal, task, usage, lib. With the test-only importers (contract/formats, account, eval, golden, settings) all `ok` or `[no test files]`, run with the production change in place. The only failures were in cli and are the two assertions of row 5a. |
| 5a. `git diff main --stat` names only the Touches files | NOT DONE | The brief's Touches names two of the four assertions that pin empty stdout with `--json` on this failure. The other two, `workflow_personal_test.go:33` and `workflow_write_test.go:148`, fail with the production change in place, so row 4 and row 5a cannot both hold. I kept row 4 and changed those two assertions the same way as the named two (see below). The diff stat therefore also names those two files and this report. This is a gap in the brief's list, not extra scope. |
| 5b. `make lint` exit 0 | HOLDS | `make lint >/dev/null && echo lint-exit-0` printed `lint-exit-0`; `gofmt -l internal/` printed nothing. |
| 5c. Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Rule: no guard, gate, cap or hold on agents | HOLDS | The change gives one error a code and records it. No check, limit or hook was added. |
| Rule: messages stay byte-equal | HOLDS | `projectSelectionError` (internal/workflow/files.go:92-100) builds the same two texts as before; the existing message tests pass unchanged. |
| Rule: no *RealPlugin* test, no UI row | HOLDS | The brief names none, so none applies. |
| Rule: small commits, no attribution lines | HOLDS | `3f3c4636`, `19c012a7`, and the report commit. |

## What changed

- `internal/workflow/files.go`: `projectSelectionError` returns `*where.LookupError{Kind: where.ErrProjectUnregistered, Message: <same text>}`. The guest and hidden-project branch keeps the text `project is missing or ambiguous`, so a guest, a private name and an unknown name still read the same diagnostic and now the same code.
- `internal/cli/workflow.go`: `env.recordError(err)` before the print in `show` and in `write`.
- `internal/cli/gm_workflow_envelope_test.go`: `TestGmWorkflowUnknownProject`.
- `internal/cli/gg_unknown_project_walk_test.go`: two map entries and the blank line above them removed.
- `workflow_project_test.go:48` and `workflow_test.go:109` expect the envelope (`assertErrorOutput`).

## Outside Touches (row 5a)

Two more existing assertions pinned "stdout empty with `--json`" on this same failure. I changed them the same way and nothing else in those tests:

- `internal/cli/workflow_personal_test.go:33` (`TestWorkflowShowPersonalVisibility`): `--json` expects the envelope, plain still expects empty stdout. The comparison of private name, private ID and unknown stays, so the member and guest still read the same answer as for an unknown project.
- `internal/cli/workflow_write_test.go:148` (`TestWorkflowWriteUsageAndPaths`): the envelope is expected, `no_such_project` for exit 1 and `invalid_argument` for exit 2 (existing `finish` rule). The check that stderr does not contain the root stays.

## NOT VERIFIED on a live screen

Nothing here is a screen. The guest answer was checked by `TestGuestProjectParity` (uninvited, no-roles and unknown guests read the same output).

NOT DONE: row 5a is the only open row; the brief's Touches list is short of two test files that pin the old empty stdout, so either row 4 or row 5a had to give, and I kept row 4.
