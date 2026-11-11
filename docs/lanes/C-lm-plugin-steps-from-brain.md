# C-lm-plugin-steps-from-brain: task steps and `grid workflow show` take templates from the brain workflow notes

Repo: plugin. Base: origin/main f6383e8c77b17a82718880a54f8330969a4b58c8 (the brief names a7809a58; the code it cites is the same on this base).
Base check: `git grep -n -E 'ErrWorkflowChanged|workflow_digest' -- '*.go'` finds only the internal/task/hold.go:36-37 comment, plus internal/contract/fe_readme_receipts_test.go:26, which lists `workflow_digest` among words that must stay *absent* from the README. No digest over a template's file label exists.

## Proof table

| # | Row / rule | Result | Proof |
|---|---|---|---|
| 1 | A template only in the project note drives steps | HOLDS, one clause differs (see below) | `go test -count=1 -v -run 'TestLmSb' ./internal/workflow/ ./internal/cli/` exit 0: `--- PASS: TestLmSbProjectNoteDrivesSteps`. `TaskWorkflow` returns the note's Quick with File `brain:project/flow` and steps `Do the work:run,Approve it:approve` (lm_steps_brain_test.go:57-75). **Differs:** with `noteText` itself, `TaskSteps` answers the error `workflow "Quick" step "Do the work" is not a usable step name`, not the two steps. `templateSteps` (tasksteps.go:204) refuses step names with spaces, as it does for workflows.md (TestTaskSteps "step name with a space", tasksteps_test.go:101). `templateSteps` is outside Touches, so it is unchanged. The same note with usable names (`do-the-work`, `approve-it`) gives `TaskSteps` = `do-the-work:run,approve-it:approve` (lm_steps_brain_test.go:76-79). |
| 2 | Order, rejected note, note default | HOLDS | `--- PASS: TestLmSbOrder`: project note → home note → project workflows.md → workspace (lm_steps_brain_test.go:84-115). A second project-level note gives `ReadFor` the error `more than one workflow note; keep one`. `Read` drops that problem, and the steps fall through to the home note (:96-107). `--- PASS: TestLmSbNoteDefault`: with no workflow named, the project note's default wins (File `brain:project/pflow`), then the home note's, then the workspace default (:119-137). |
| 3 | Guests and Personal | HOLDS | `--- PASS: TestLmSbGuestAndPersonal` (lm_steps_brain_test.go:141-191), laid out as TestWorkflowProjectSelectionGuestScope (project-roles.json naming guest@example.com, on alpha only). The guest gets the alpha note's Quick. It gets no `Homeonly` (`no workflow is named "Homeonly"`) and no `brain:home/` template in `Read`. For beta it gets no steps, and the helper's own check returns no templates for beta. A Personal task gets the home note's `Homeonly` and `Quick`. `Read` with no project lists the home note's templates first, then the files', and no project note. Source, Note and GatePostRoles stay empty. |
| 4 | `grid workflow show --json` | HOLDS | `--- PASS: TestLmSbCliShowListsNote` (internal/cli/lm_show_brain_test.go:64-94; harness as TestGmWorkflowUnknownProject). `--project alpha --json` and `--project id-alpha --json` both give top-level keys exactly `errors,files,templates`, templates `brain:project/flow:Quick,workflows.md:Standard`, and the output holds `"file":"brain:project/flow"`. Plain `workflow show` stays without the note's template. With no workflow note in the home, `--json` and `--project alpha --json` list exactly `workflows.md:Standard`. |
| 5 | hx pins, package tests, diff scope | HOLDS | `go test -count=1 -v -run 'TestHxTwo' ./internal/workflow/` exit 0: `--- PASS: TestHxTwoSourcesDisagree`, `--- PASS: TestHxTwoNoteOnly`, `--- PASS: TestHxTwoFileOnly`. The first two now assert the note's template and steps through `hxTwoWantNoteSteps` (hx_two_sources_test.go:60-74). `go test -count=1 ./internal/workflow/ ./internal/task/ ./internal/guard/ ./internal/cli/` exit 0. Before this report was added, `git diff main --stat` named only the 6 Touches files: internal/cli/lm_show_brain_test.go, internal/workflow/{hx_two_sources_test.go, lm_brain_templates.go, lm_steps_brain_test.go, read.go, tasksteps.go}. This report, docs/lanes/C-lm-plugin-steps-from-brain.md, is the 7th file, as the builder runbook requires. |
| 6 | lint, cross, walk tests | HOLDS | `make lint` exit 0 (go vet ./...). `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile). `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0. `go test -count=1 ./internal/job/lint/` exit 0. |
| R | New tests fail on base | HOLDS | With read.go and tasksteps.go stashed (the helper file kept so the tests compile): `hx_two_sources_test.go:84: TaskWorkflow = &{Name:Quick ... File:alpha/workflows.md ...}` and `--- FAIL: TestHxTwoSourcesDisagree`; `hx_two_sources_test.go:113: TaskWorkflow = <nil>, no workflow is named "Quick"` and `--- FAIL: TestHxTwoNoteOnly`; `lm_steps_brain_test.go:65: TaskWorkflow template = <nil>, no workflow is named "Quick"` and `--- FAIL: TestLmSbProjectNoteDrivesSteps`; `lm_steps_brain_test.go:95: TaskSteps(alpha) = "file-step:run", <nil>; want "project-note:run"` and `--- FAIL: TestLmSbOrder`; `lm_steps_brain_test.go:130: TaskWorkflow = &{Name:Standard ... File:workflows.md ...}, <nil>; want the project note's default` and `--- FAIL: TestLmSbNoteDefault`; `lm_steps_brain_test.go:151: TaskSteps(alpha) = "ws-step:run", <nil>; want "alpha-note:run"` and `--- FAIL: TestLmSbGuestAndPersonal`; `lm_show_brain_test.go:72: --project alpha: templates workflows.md:Standard, want the note's Quick then the workspace Standard` and `--- FAIL: TestLmSbCliShowListsNote`. The no-note CLI check passed on base, because it pins behaviour that stays the same, so it is folded into TestLmSbCliShowListsNote. |
| R | No existing test edited to pass | HOLDS | Only the two pins the brief names were rewritten, as their own messages asked (TestHxTwoSourcesDisagree, TestHxTwoNoteOnly). TestHxTwoFileOnly and hxTwoHome are unchanged. No test deleted. |
| R | Tests of changed packages and importers | HOLDS | `go list` importers of internal/workflow and internal/cli: cmd/grid, contract/formats, internal/account, internal/braincmd, internal/cli, internal/eval, internal/feed, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, internal/task, internal/workflow, lib. Together with internal/guard (reads task.StepsFor), `go test -count=1` on all of them exit 0. |
| R | No guard, gate, cap, hold or blocking hook added | HOLDS | The diff adds a template source only. Gate roles (gates.go) are unchanged. |
| R | Names | HOLDS | New package-level names start with `lmSb` (lm_brain_templates.go, lm_steps_brain_test.go) or `lmSbCli` (lm_show_brain_test.go). Tests start with `TestLmSb`. Reused helpers: noteText, writeNote, putWorkflowFixture, stepsTable, stepsStandard, stepsWorkspace, stepsTask, stepsString, hxTwoSteps. `hxTwoWantNoteSteps` lives in the hx file and serves only the two rewritten pins. |
| R | gofmt, commits, no attribution, no personal data | HOLDS | `gofmt -l internal/workflow internal/cli` printed nothing. Commits have one-sentence messages and no attribution lines. |
| R | Real-plugin tests | n/a | The brief names no *RealPlugin* row. |

## What changed

- New internal/workflow/lm_brain_templates.go:
  - `lmSbBrainTemplates(r, folder)` calls `fromBrain` (and through it `take`) on a scratch Answer. It goes to the project level first, under ReadFor's check `PersonalProjectAllowed && GuestProjectAllowed`, then the home level, under `!IsGuest && PersonalOK`.
  - A rejected note adds nothing. A taken note adds its templates and only the problems labelled with its own `brain:<level>/<name>`. Problems that `workflowNotes` adds for other files at the same level stay out.
  - `lmSbNoteFolder` takes the project folder from `Files[i].Project` that readFiles resolved, so a name or an ID both work. It returns nothing with no selector or with a Personal selector.
  - `lmSbNoteDefault` finds the project note's default template, else the home note's.
- read.go `read`: the note templates and problems come first in the answer, then the workflows.md ones. Source, Note and GatePostRoles are not set.
- tasksteps.go:
  - `scopeTemplates` Personal branch: home note, then `<home>/workflows.md`. The file parse result is appended after the note's, where it used to overwrite.
  - `defaultTemplate`: after the error and rule checks, a note's default (project, then home) wins, then today's workspace rule.
- `namedTemplate` is unchanged; answer order already makes the project note win, then the home note, then the project file, then the workspace file.

**Design point for review (data format):** notes are parsed with project `""`, the way the workspace file is parsed, at both levels. `Parse` with a project refuses a default template (parse.go:278, "a project's file holds no default workflow"), so row 2's "default template in the project note" needs this. As a result:
- a note's body must open with `# Workflows` (`# alpha workflows` is not taken);
- a note cannot copy a workspace workflow (`applies when: as in the workspace X`) or say `only for project`.

ReadFor and brainGatePostRoles still pass the project; gate roles do not depend on it.

## Open items

- Row 1's clause "`TaskSteps` returns the note's two steps" with `noteText` differs, as stated in row 1. The step-name rule in `templateSteps` would need an owner call to change; it is outside this lane's Touches.

## NOT VERIFIED (live screen)

- Screenshot pair at 1440x900 of `#/project/<p>/workflow` against pages/grid-design/project.html (the Workflow tab's template cards). The UI track takes it after landing.

DONE
