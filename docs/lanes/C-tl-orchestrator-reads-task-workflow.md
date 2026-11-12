# C-tl-orchestrator-reads-task-workflow

Base: plugin main 31c7f1823d362bf56d621536e778c9ed1517ff49. Base check `git grep -n "workflow show --task" -- skills agents` printed nothing, so the lane went ahead.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1 Red on base (only the new test file) | HOLDS | `go test -count=1 -v -run TestTlWorkflowReader ./internal/job/lint/` exit 1. Failing lines: `tl_workflow_reader_test.go:99: skills/plan-jobs/SKILL.md does not name `grid workflow show --task`: a line names `grid workflow show` without --task: - Approvals and gates come from the workflow ...` and `tl_workflow_reader_test.go:102: agents/orchestrator.md does not name `grid workflow show --task`: no line before "2. Decide the Jobs" holds `grid workflow show --task <task> --json``. |
| 2 Green, with (a), (b), (c) | HOLDS | Same command after the two edits: `--- PASS: TestTlWorkflowReader (0.00s)`, exit 0. (a) skills/plan-jobs/SKILL.md:11 holds the command, `task.steps` and `task.rounds` and no line names `grid workflow show` without `--task`; (b) agents/orchestrator.md:18 (sub-item 3 of step 1, before "2. Decide the Jobs" at :19); (c) `tlWorkflowShowHasFlag` reads `contractdata.Commands` for workflow/show/`--task` (contract/commands.json:3845). The test first runs the checks on a copy of the SKILL text, and of the orchestrator text, with `--task` cut out and fails if they pass (`t.Fatal`), so the green is the check's. The command in the orchestrator must be one backticked span (no `;`, no pipe). |
| 3 lint, job, plan, walk tests | HOLDS | `go test -count=1 ./internal/job/lint/` exit 0 (`ok .../internal/job/lint 0.088s`). `go test -count=1 ./internal/job/... ./internal/plan` exit 0 (job, frontmatter, jobcmd, lint, plan ok). `go test -count=1 -v ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` exit 0: `--- PASS: TestHsIpWalksEveryCommand (0.00s)`, `--- PASS: TestIlGuestSiteList (0.02s)`. |
| 4 `go test -count=1 ./internal/cli` | HOLDS | exit 0, `ok .../internal/cli 30.121s`. |
| 5 `make lint`, `make cross` | HOLDS | `make lint` (go vet ./...) exit 0. `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test compile, all `ok`. `gofmt -l internal/job/lint` prints nothing. |
| 6 `git diff main --stat` | HOLDS, plus the report | The three files of the brief, plus this report docs/lanes/C-tl-orchestrator-reads-task-workflow.md that the runbook requires. No Go code outside the test changed. |
| Importers of changed packages | HOLDS | `go list` imports of internal/job/lint: internal/cli, internal/job/jobcmd. Both ran green above (internal/cli full, internal/job/...). |
| Repo-wide walk tests (plugin) | HOLDS | TestHsIpWalksEveryCommand and TestIlGuestSiteList PASS (row 3); `go test ./internal/job/lint/` ok (row 3). |
| New test fails on base, passes on lane | HOLDS | Row 1 and row 2. |
| No existing test edited or deleted | HOLDS | Only the new test file was added. |
| No guard, gate, cap or hold on agents | HOLDS | Text edits only; the new sentence says to read the workflow and never pick a template. |
| RealPlugin tests | Not run | The brief names none; no command behaviour changed. |

## What changed

- skills/plan-jobs/SKILL.md:11: "read it with `grid workflow show`" became "read the workflow this task runs with `grid workflow show --task <task> --json` and use its `task.steps` (kind, agent, approvers) and `task.rounds`", rest of the line kept.
- agents/orchestrator.md:18: new sub-item 3 under step 1 with the one command; steps 1 to 8 keep their numbers.
- internal/job/lint/tl_workflow_reader_test.go: `TestTlWorkflowReader` with helpers `tlWorkflowSkillProblem`, `tlWorkflowOrchestratorProblem`, `tlWorkflowShowHasFlag`.

## NOT VERIFIED on a live screen

Nothing here needs a screen. Not checked: that a live orchestrator agent follows the new line (no claude run, per the brief); the JSON shape of `task.steps` and `task.rounds` was taken from the brief's reading of internal/cli/nh_workflow_show_task.go, not run.

## Open items

None. Step 1's lead-in "Read the task and the plan, one Bash call each" is left as it was; the new sub-item is one more Bash call.

DONE
