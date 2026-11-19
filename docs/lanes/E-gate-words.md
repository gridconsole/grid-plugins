# E-gate-words: user-facing text says person Job / approve step, not gate

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. List every user-facing string and doc line that says gate | HOLDS | Table below (old -> new, file:line). |
| 2. Reworded; identifiers and keys unchanged | HOLDS | `git diff --stat`: 9 files, none a rename. `git diff -U0 -- internal contract testdata \| grep -E 'json:"\|"gate\|gate_post_roles'` shows only the removed test line "gated by classify" (a display string, not a key). `gate:` tokens, `gate_post_roles`, `--gate`, `gate: ship` in task.md are untouched. |
| 3. Golden and help tests updated, none asserts old words | HOLDS | Before the golden edits: `go test ./internal/cli` FAILED `TestContractCommandsInventory` (contract_commands_test.go:91) and `TestContractCommandsParserInventory` (contract_inventory_test.go:379, "approve a gate" vs new summary). After editing `testdata/golden/help.golden:20`, `contract/commands.json:2506`: `go test ./internal/cli` ok. The one existing test edited, `internal/cli/job_graph_test.go:97`, asserts the new "waits for classify" because the string it pinned was reworded. |
| 4. go test, make lint | HOLDS (scoped) | `go test ./cmd/grid ./internal/braincmd ./internal/cli ./internal/guard ./internal/hook ./internal/job/... ./internal/land ./internal/signal ./internal/task ./contract/...` exit 0 after the fix (cli, task re-run ok). Importers of cli/task from `go list`: cmd/grid, braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, land, signal, task. `make lint` (go vet ./...) exit 0. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. Whole-module test is the orchestrator's. |
| No guards, caps or holds added | HOLDS | Strings and docs only. |
| No personal paths, hosts or emails | HOLDS | None added. |
| Small commits, no attribution | HOLDS | One commit, one sentence. |

## Strings changed (row 1)

| Where | Old | New |
|---|---|---|
| internal/cli/cli.go:116 (also help.golden:20, contract/commands.json:2506) | approve a gate (a person or the app only) | approve the plan or a later approve step (a person, the app or an agent) |
| internal/cli/job_graph.go:113 | gated by classify | waits for classify |
| internal/task/gates.go:155 | no approval for gate %q: only the plan is approved | no approval for the %q approve step: only the plan is approved |
| internal/task/gates.go:164 | approval is for another gate | approval is for another approve step |
| docs/job-contract.md:36-41 | gate ids / Gate ids / Gate state | person-question ids / A person question id / Whether the person has answered |
| docs/plan.md:150,160,166 | task and gate checks / plan gate / satisfy the gate | task and approval checks / approve-the-plan step / satisfy the check |
| docs/delivery.md:112,122,136,147,169-177,197 | ship gate raised / gate is not raised / gate stays / raises that gate / The gate a person saw / ship gate in-process, person gate | ship approve step waits / does not start waiting / The `gate` key stays / makes the preview approve step wait / The approve step a person saw / moves the task to the ship approve step, does not require a person |

Kept on purpose (identifiers, keys, tokens): `gate:Q-7`, `--gate`, `gate: ship|preview` task.md key, `gate_post_roles`, Feed `gate` body field.

## Open items (outside the brief's Touches)

- Strings in other packages still say gate and need a follow-up lane: internal/feed/store.go:200,240 ("may not post %s gates", "gate id %s already exists"; pinned by tests in feed, mcp, cli gate_test.go and gate_squat_test.go), internal/jobgraph/graph.go:65,219 ("invalid gate edge", "waiting on gate"), internal/job/lint/lint.go:245, internal/question/files.go:82 and feed.go:78, question.go:181, internal/orchestrator/gates.go:47 and ready.go:186, internal/workflow/gates.go:43,49, internal/guard/host.go:14 gateReason (hook/guard area).
- docs/delivery.md still quotes a "Changed since you approved: the task is at ... approve step" message that no code currently emits (stale doc; not changed beyond the wording).
- Code comments that say gate were left alone.

## NOT VERIFIED on a live screen

The help text in a terminal and any UI that shows the approve summary.
