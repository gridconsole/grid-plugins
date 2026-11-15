# C-r6-one-ship-ask: proof table

Base: 3adc0590 (main; `git merge origin/main` after fetch: "Already up to date").
Base check: at base, `internal/cli/nh_workflow_show_task.go:196-203` (`nhWfBuiltIn`) gave Ship as `{Name: "Ship", Kind: "approve", Approvers: []string{}}` with no posted-by field, and `skills/plan-jobs/SKILL.md:11` ended "A step only a person can do is a Job with role Person (`grid job post --person`)." with nothing about deliver. The base answer did not mark Ship, so the lane went ahead.

The field: **`posted_by`** on a `task.steps` entry of `grid workflow show --task <t> --json`. Its value is `"deliver"` on an approve step whose person Job `grid deliver` posts (postShipPersonJob). The key is absent on every other step (`omitempty`). Only the built-in flow sets it today. The schema allows any approve step to carry it, so a brain workflow may set it later.

Round 7 must show: exactly one approve person Job for task E1, posted by deliver at `waiting`/`step: ship`, and no person Job posted by the orchestrator.

| Row | Verdict | Proof |
|---|---|---|
| A1 new tests red on base | HOLDS | Base (`go test -count=1 -run 'TestR6BuiltInShipPostedByDeliver\|TestR6PlanJobsShipPostedByDeliver' ./internal/cli/ ./internal/job/lint/`, exit 1): `r6_ship_posted_by_test.go:46: answer: step map[agent:<nil> approvers:[] kind:approve name:Ship], want Ship (approve) with posted_by deliver`; `r6_ship_posted_by_test.go:46: fixture task-built-in: step map[... name:Ship], want Ship (approve) with posted_by deliver`; `r6_ship_posted_by_skill_test.go:34: skills/plan-jobs/SKILL.md: the workflow line does not hold: An approve step whose ...`; `FAIL internal/cli`, `FAIL internal/job/lint` |
| A1 new tests green on lane | HOLDS | Same command plus `TestNhWf\|TestBuiltInFlowShowsShipPersonStep\|TestTlWorkflowReader`, exit 0: `ok internal/cli 0.375s`, `ok internal/job/lint 0.007s` |
| A2 TestApproveAfterDeliveryRecordsPlanApproval and approve_no_gate_test.go unedited and passing | HOLDS | `git diff --stat origin/main..HEAD -- internal/cli/delivery_test.go internal/cli/approve_no_gate_test.go` prints nothing; `go test -count=1 -v -run 'TestApproveAfterDeliveryRecordsPlanApproval\|TestApproveGateFlagIsUnknown\|TestRefusalVerbsAreUnknown' ./internal/cli/`, exit 0, three `--- PASS` |
| A3 `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| A3 `go test ./internal/cli/` and the skill-test package | HOLDS | Unpiped, exit 0, in the run below: `ok internal/cli 38.379s`, `ok internal/job/lint 0.268s` |
| A4 round-7 line | HOLDS | The "Round 7 must show" line above |
| Do 1 workflow answer carries the field | HOLDS | `internal/cli/nh_workflow_show_task.go:53` (field), `:57` (const), `:208` (Ship sets it); `contract/schemas/cmd-workflow-show.json:309` |
| Do 2 SKILL.md:11 sentence | HOLDS | `skills/plan-jobs/SKILL.md:11`. skills/deliver is unchanged: deliver/SKILL.md:10 already says deliver sets `step: ship` for the person's review, which does not contradict |
| Do 3 fixture and test carry the field | HOLDS | `contract/fixtures/cmd-workflow-show/valid/task-built-in.json:40`; `TestR6BuiltInShipPostedByDeliver` (internal/cli/r6_ship_posted_by_test.go) checks the live answer and the fixture. Unedited `TestNhWfShowTaskSources` validates the answer against the schema and compares it with the fixture (exit 0) |
| Do 3 skill text test | HOLDS | `TestR6PlanJobsShipPostedByDeliver` (internal/job/lint/r6_ship_posted_by_skill_test.go). It follows the `tl_workflow_reader_test.go` pattern: a problem function plus a cut text that must fail |
| Tests of changed packages and importers | HOLDS | Importers of internal/cli and contract, from `go list` Imports/TestImports/XTestImports: cmd/grid, contract, contract/formats, internal/account, internal/braincmd, internal/cli, internal/contract, internal/golden, internal/hook, internal/job/lint, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. Readers of the skill and agent text added: ., internal/job/..., internal/plan, internal/brain. `go test -count=1 . ./cmd/grid ./contract/... ./internal/account ./internal/braincmd ./internal/cli ./internal/contract ./internal/golden ./internal/hook ./internal/job/... ./internal/jobgraph ./internal/mcp ./internal/packaging ./internal/question ./internal/settings ./internal/plan ./internal/brain ./lib/...`, exit 0, every package ok |
| Walk tests (plugin) | HOLDS | After the merge: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`, exit 0; `go test -count=1 ./internal/job/lint/`, exit 0 |
| Windows build | HOLDS | `GOOS=windows go build ./...`, exit 0 |
| macOS build | HOLDS | `GOOS=darwin go build ./...`, exit 0 |
| No guard, gate, cap, hold or lint that blocks a Job | HOLDS | The diff adds one output field, schema text and skill/agent sentences. Nothing refuses or holds a Job |
| No existing test edited | HOLDS | `git diff --stat origin/main..HEAD` shows only new `_test.go` files |
| App guest walk tables (`make realplugin ...`) | not run, not asked | Plugin lane; that rule names the app repo |
| Whole-module test and cross-repo set | not run, not asked | The orchestrator runs these at landing |

## What changed

- `internal/cli/nh_workflow_show_task.go`: `nhWfStep.PostedBy` (`posted_by,omitempty`). The built-in Ship step sets it to `deliver`. The text answer is unchanged.
- `contract/schemas/cmd-workflow-show.json` (further file: the code forces it, since step items are `additionalProperties: false`): optional `posted_by`, enum `["deliver"]`, not required.
- `contract/fixtures/cmd-workflow-show/valid/task-built-in.json`: Ship carries `"posted_by": "deliver"`.
- `skills/plan-jobs/SKILL.md:11`: an approve step whose `posted_by` is `deliver` is not a Job the orchestrator posts, and its Jobs end at the step before it.
- `agents/orchestrator.md:18` (further file, outside the brief's Touches list). Reason: at base this line said `task.steps` "are the steps to post Jobs for", which contradicts the new skill sentence. That line is what the orchestrator follows when it posts per step, and it is the cause of J-4. One sentence was added: a step whose `posted_by` is `deliver` gets no Job from the orchestrator.
- New tests: `internal/cli/r6_ship_posted_by_test.go` and `internal/job/lint/r6_ship_posted_by_skill_test.go`.

## Open items

- The app may decode `cmd-workflow-show` task steps with a strict struct. The new key is optional and additive, but the app-side read is NOT VERIFIED in this lane; the cross-repo set at landing checks it.
- NOT VERIFIED live: whether a real orchestrator run (round 7) posts no Ship person Job. Only an eval run can show that.

DONE
