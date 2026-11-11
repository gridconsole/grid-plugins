Base: plugin main a7809a58 (`git rev-parse --short HEAD` on lane-C-la-land-approval-research before this doc). Every path:line below is read on a7809a58.

# Where the land step reads its fixed gate, and how it would take its approval from the workflow's approve step

Research only; no code changes. Scope T27 brief 8. Owner rule (simplify.md:11, :39): code knows workflow steps, not a plan gate or a ship gate; approvals come from the task's workflow with a default.

The brief named 35a498dc; the lane was cut from a7809a58. Every line the brief cites is the same on a7809a58:

- internal/land/approval.go:75-83 — `value, present := fields["land-approval"]` … `return "", fmt.Errorf("workflow land-approval must be plan, ship or none")`; absent means "plan" (:76-78, comment :48).
- internal/land/land.go:71 — `p.Gate, err = s.gate(t)`.
- internal/land/land.go:86-87 — `if p.Gate == "none" && p.Rule.NeedsApprove { p.Gate = "plan" }`.
- internal/land/land.go:219-220 — `if p.Gate != "none" { approval, err := s.tasks().ApprovalForGate(cwd, t.ID, p.Gate)`.
- internal/migrate/receipt.go:83-84 — `case r.Gate != "": return r, false, ReceiptKept, "approval.json names the gate " + r.Gate + ", not the plan", nil`.

## Key finding

The gate name already decides nothing past "none or not". `ApprovalForGate` ignores its `gate` argument (internal/task/gates.go:140-143: "The gate name is ignored: the plan Job is the only approval"). It reads `planJobResult` (gates.go:255), which takes the newest accepted Person Job of kind `approve` with a `plan_digest` (gates.go:285, :300). So "plan" and "ship" give the same answer today. The only thing `land-approval` really controls is whether land asks for an approval at all ("none").

A second thing: land reads its own workflow file, `projects/<p>/knowledge/workflows/<name>.md` front matter (approval.go:62). That is not the workflow the rest of the plugin resolves: `workflow.TaskWorkflow` (internal/workflow/tasksteps.go:32) reads the inline `workflow:` list in task.md, then the named or default template from `workflows.md` (tasksteps.go:89-107). approval.go:47 says it is a stopgap "until a shared workflow reader exists". That reader exists now.

## 1. Readers and writers of the land gate value

Command: `git grep -n -w -E 'Gate|gate|land-approval|ApprovalForGate' -- internal/land internal/task internal/migrate internal/cli internal/orchestrator ':!*_test.go'`

internal/land:
- internal/land/approval.go:47 — comment on `gate`. Reads (doc).
- internal/land/approval.go:48 — comment: absent `land-approval` keeps the plan gate. Reads (doc).
- internal/land/approval.go:50 — `func (s Store) gate(t task.Task)`. Reads: the only producer of the value.
- internal/land/approval.go:75 — `fields["land-approval"]`. Reads the workflow field.
- internal/land/approval.go:79 — `gate, ok := value.(string)`. Reads.
- internal/land/approval.go:80 — accepts only plan, ship or none. Reads.
- internal/land/approval.go:81 — returns the value. Reads.
- internal/land/approval.go:83 — error on any other value. Reads.
- internal/land/land.go:41 — `Gate string \`json:"approval_gate"\``. Writes: the value goes into the `grid land` result JSON (`plan.approval_gate`), and it is part of the `reflect.DeepEqual` re-check at land.go:216.
- internal/land/land.go:71 — `p.Gate, err = s.gate(t)`. Writes Plan.Gate from the workflow.
- internal/land/land.go:86 — `if p.Gate == "none" && p.Rule.NeedsApprove`. Reads.
- internal/land/land.go:87 — `p.Gate = "plan"`. Writes (deploy rule override).
- internal/land/land.go:183 — comment "The task workflow selects the approval gate". Reads (doc).
- internal/land/land.go:219 — `if p.Gate != "none"`. Reads: the one decision the value makes.
- internal/land/land.go:220 — `ApprovalForGate(cwd, t.ID, p.Gate)`. Reads and passes it on (ignored by the callee).

internal/task:
- internal/task/gates.go:119 — comment on keptApproval: a receipt that "names a gate" is not kept. Reads old receipts.
- internal/task/gates.go:126 — `e.Gate != ""` rejects a gate-named approval.json. Reads old receipts.
- internal/task/gates.go:137 — `Approval` calls `ApprovalForGate(cwd, id, "plan")`. Writes a fixed "plan" literal (ignored).
- internal/task/gates.go:140 — comment: the gate name is ignored. Unrelated to the value (doc).
- internal/task/gates.go:143 — `func (s Store) ApprovalForGate(cwd, id, gate string)`. Reads the argument and drops it.
- internal/task/write.go:80-82 — `Event.Gate`, "read from old history lines only: Grid writes none". Reads old history.
- internal/task/write.go:89 — "supplied by the CLI gate" (CLI entry point). Unrelated.
- internal/task/write.go:325 — comment on the plan gate (step position). Unrelated to the land value; it is the PlanApprovalIndex idea.
- internal/task/write.go:524, :588 — legacy per-gate receipt files (`approval-preview.json`, `approval-ship.json`). Reads old receipts (migration).
- internal/task/deliver.go:42, :152-153, :191, :258 — comments on step/plan-gate state moves. Unrelated to the land value.
- internal/task/hold.go:34, :87-88, :110, :116, :122-123, :130, :160 — the plan approval step index (`taskfile.PlanApprovalIndex`). Unrelated to the land value; this is the step-based reader point 3 builds on.

internal/migrate:
- internal/migrate/receipt.go:40 — comment: a receipt with a gate is kept, not moved. Reads old receipts.
- internal/migrate/receipt.go:54 — `Gate string \`json:"gate"\`` on the old receipt. Reads old receipts.
- internal/migrate/receipt.go:79 — comment: readers accepted only a plan receipt with no gate. Reads old receipts.
- internal/migrate/receipt.go:83 — `case r.Gate != ""`. Reads old receipts.
- internal/migrate/receipt.go:84 — keeps the receipt: "approval.json names the gate …, not the plan". Reads old receipts.

internal/cli:
- internal/cli/cli.go:29 — "the early guest gate". Unrelated (guest write refusal).

internal/orchestrator (all unrelated: Feed question gates, `gate:` edges, the question default from internal/question `ReserveGate`):
- internal/orchestrator/events.go:89, :91, :93, :109, :113, :366-367, :379-380.
- internal/orchestrator/gates.go:27-28, :47.
- internal/orchestrator/ready.go:55, :111, :161-162, :166-167, :172, :174, :178, :180-181, :206, :208, :233.
- internal/orchestrator/write.go:105, :116, :281, :283.

## 2. Tests that pin a gate value

Command: `git grep -l -E 'land-approval|ApprovalForGate|Gate: *"(plan|ship|none)"' -- '*_test.go'`

- internal/land/land_test.go:325 — writes `land-approval: <gate>` into `knowledge/workflows/release.md`; :412-414 table rows for wrong type, empty and duplicate `land-approval`. Pins the field and its three values. Retired by brief LA2 below.
- internal/task/approval_read_test.go:216, :224, :252, :267, :314 — `ApprovalForGate` with "plan" and a loop over gate names; pins that every name gives the same answer. Retired by LA3 (renamed reader).
- internal/task/gates_digest_test.go:35 — loop over gate names; same answer for each. LA3.
- internal/task/gates_test.go:152 — `ApprovalForGate(..., "ship")` gives ErrNoPlanJob. LA3.
- internal/cli/approve_reader_test.go:86 — `ApprovalForGate(path, "T-07", gate)` in a loop. LA3.

No test has `Gate: "plan|ship|none"` as a struct field literal.

## 3. How a workflow's approve step is read today, and the smallest change

Readers today:
- internal/taskfile/workflow.go:15 `WorkflowSteps(text)` — the inline `workflow:` list in task.md, `[]WorkflowStep{Step, Kind}`; nil on anything malformed.
- internal/guard/helpers.go:13 `WorkflowSteps` — a thin forward to taskfile.WorkflowSteps for the hooks.
- internal/workflow/tasksteps.go:23 `TaskSteps` / :32 `TaskWorkflow` — the full resolver: inline list, else named template (project before workspace), else the workspace default; every failure is an error.
- internal/task/hold.go:43 `StepsFor(r, dir)` — the lookup the guard and Deliver share; adds the folder/project check and the workflow_digest check (hold.go:34-41).
- internal/taskfile/workflow.go:45 `PlanApprovalIndex(steps)` — the first `kind: approve` step that names "plan" or comes before any run/work step.
- internal/task/hold.go:91 `PlanStageWith` — places the task's current step against that approve step ("before", "gate", "past").
- The approval itself: internal/task/gates.go:255 `planJobResult` — the newest accepted Person Job of kind `approve` with a `plan_digest`; "by whom" is `JobApproval.Who` (gates.go:305).

Gap: a Feed Job record does not carry the workflow step it belongs to (contract.Job has no step field; the only `Step` is on Checkpoint, internal/contract/types.go:112). So today the approval reader cannot tell which approve step a Job answers; it takes any approve Person Job.

Smallest change (land asks "is the approve step before land done, and by whom"):
1. In `land.prepare`, replace `s.gate(t)` (land.go:71) with the steps from `task.StepsFor(s.Resolver, t.Path)` on task.md, the same reader as the guard and Deliver. Drop approval.go:47-84 and its `knowledge/workflows/<name>.md` read.
2. Decide "needs approval" from the steps: true when the workflow has a `kind: approve` step at or before the land/ship step (or anywhere, while steps carry no land step kind); false when it has none. Replace `Plan.Gate string` with `Plan.Approve string`, the name of that approve step ("" when none). The JSON field `approval_gate` becomes `approve_step` (data-format change on the `grid land` result).
3. Keep the deploy rule: when the step list has no approve step and `Rule.NeedsApprove` is true, land still needs the approval (land.go:86-87 stays, written as `if p.Approve == "" && p.Rule.NeedsApprove`).
4. At land.go:219-220 ask `s.tasks().Approval(cwd, t.ID)` (gates.go:136) when an approve step is named; report `approval.Who` and `When` in the result and the "land started" note so the "by whom" is on record. `ApprovalForGate` loses its unused argument (or is removed for `Approval`).
5. Default when the workflow has no approve step: no approval asked, unless the deploy rule says needs-approve. When the steps cannot be resolved (StepsFor error): refuse with that error, as approval.go:49 does now ("never an approval bypass"). This matches the shared default in `approvalRequired` (hold.go:162): unresolved steps need approval.

Optional follow-up, only if one workflow carries two approve steps (plan and a later one before land): the approve person Job would need a `step` field naming its workflow step, so the reader can pick the approve step before land. That is a Feed record format change and is its own brief (LA4).

## 4. What stays

- The deploy rule's `NeedsApprove` (land.go:33, :86, :155): a workflow cannot waive it. Unchanged in meaning.
- The plan/lands hash check at land.go:224 (`approval.PlanHash != p.Hash || approval.LandsHash != ruleHash(p.Rule)`) and the approval binding in approval.go:30-45 (`Approve`).
- migrate's reading of old receipts: internal/migrate/receipt.go:40, :54, :79, :83-84 keep a gate-named approval.json in place; internal/task/gates.go:119-126 and write.go:80-82, :524, :588 read old receipts and history only. These are old-data readers, not gates, and stay until the owner retires old-receipt migration.
- The orchestrator's question gates (`gate:` edges, ReserveGate) are a different thing and are untouched.

## Briefs that follow

- LA1 land-reads-task-steps: land takes "needs approval" from `task.StepsFor` steps (approve step present) instead of `land-approval`; files internal/land/land.go, internal/land/approval.go, new internal/land test file; rows: no `land-approval` read in internal/land, a workflow with an approve step asks the approval, one without asks none, needs-approve still asks, unresolved steps refuse; Review: yes (the `land-approval` workflow field stops being read).
- LA2 land-result-approve-step: `Plan.Gate`/`approval_gate` becomes `Plan.Approve`/`approve_step`, and the result and land note carry the approver (`Who`, `When`); files internal/land/land.go, the app's land result reader if any; rows: JSON field renamed, who/when shown, land_test.go:325 and :412-414 retired with the test that now covers them named; Review: yes (data-format change on the `grid land` output).
- LA3 approval-reader-no-gate-arg: drop the ignored `gate` argument, `ApprovalForGate` folds into `Approval`; files internal/task/gates.go, internal/land/land.go, and the tests at approval_read_test.go, gates_digest_test.go, gates_test.go:152, approve_reader_test.go:86 re-pointed; rows: no `ApprovalForGate` left in production, same behaviour pinned by a new test file; Review: no (API only, no data format).
- LA4 approve-job-step (only if needed): the approve person Job records the workflow step it answers, so land can pick the approve step before land when a workflow has two; files internal/contract, internal/feed, internal/task/gates.go, grid approve; rows: Job carries `step`, reader picks the step before land, old Jobs without `step` read as today; Review: yes (Feed record format).
