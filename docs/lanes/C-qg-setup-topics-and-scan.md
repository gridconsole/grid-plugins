# C-qg-setup-topics-and-scan

Base: 44c7de5bacbac31560de11e84afd4ff15980c05d (plugin main).

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| Base check: `project scan` with no operand | HOLDS (seen on base) | `TestQgSetupLinesWalk` on base: `` `grid project scan` exit 2: grid project scan: wrong number of arguments `` and the same for `grid project scan --apply` |
| Base check: `check` names `grid deploy` | HOLDS (seen on base) | `brain check` on base printed `"message":"config: run grid deploy"` (also `grid environments`, `grid agents`, `grid people`, `grid routines`) |
| 1 TestQgConfigFindingNamesAVerb red on base | HOLDS | Base: `qg_config_step_test.go:71: environments: finding "config: run grid environments" names `grid environments`, which is not a verb in contract/commands.json` (same for agents, deploy, machines, people, routines, workflows); `--- FAIL`. Lane: `go test -count=1 -v -run 'TestQg\|TestCheckConfigTopic' ./internal/brain/ ./internal/cli/` exit 0 prints `--- PASS: TestQgConfigFindingNamesAVerb`, `--- PASS: TestCheckConfigTopic`, `--- PASS: TestQgSetupLinesWalk`. The test walks the keys of `configProposalTopics` (internal/brain/qg_config_step_test.go:50). |
| 2 TestQgSetupLinesWalk red on base | HOLDS | Base: `exit 2 ... wrong number of arguments` for both scan lines, and `brain check exit 3 ... config_topic` for agents, deploy, environments, people, routines; `--- FAIL: TestQgSetupLinesWalk`. Lane: `--- PASS: TestQgSetupLinesWalk`. It reads the skill through `gridplugins.SkillFor("setup")`, runs every `grid project scan` span with a temp folder holding one git repo (exit 0), runs the propose line once per topic it names (`decisions`, `conventions`; exit 0), then `brain check --project example --json` (exit 0, no `config_topic`). |
| 3 braincmd + contract pins | HOLDS | `go test -count=1 -v -run 'TestCheckProposalByID\|TestHeMvMessagesNameRealSubs\|TestHeMvProseListBothWays' ./internal/braincmd/ ./internal/contract/` exit 0, `--- PASS: TestHeMvMessagesNameRealSubs`; `git diff main -- internal/contract/` is empty (no new `heMvProse` entry). |
| 4 greps | HOLDS | `git grep -n 'config: run grid "+topic' -- internal` prints nothing; ``git grep -n -E 'grid project scan( --apply)?`' -- skills/setup/SKILL.md`` prints nothing. |
| 5 package tests | HOLDS | `go test -count=1 ./internal/brain/ ./internal/braincmd/ ./internal/cli/ ./internal/packaging/` exit 0 (all ok). |
| Importers of `internal/brain` (from `go list`): brain, braincmd, cli, hook, job, job/jobcmd, job/jobtest, people, projectenv, routine, task, workflow | HOLDS | `go test -count=1 ./internal/hook/... ./internal/job/... ./internal/people/... ./internal/projectenv/... ./internal/routine/... ./internal/task/... ./internal/workflow/...` exit 0 |
| `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| `make cross` (darwin/arm64, windows/amd64 vet + test compile) | HOLDS | exit 0 |
| Walk: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | ok |
| Walk: `go test -count=1 ./internal/job/lint/` | HOLDS | ok |
| New tests fail on base, pass on lane | HOLDS | pasted above |
| No guard, gate or cap added; config-topic refusal kept | HOLDS | only the finding text changed; internal/brain/store.go untouched |
| No attribution lines, no personal paths | HOLDS | commits are one sentence each |
| `git diff main --stat` names only Touches files | HOLDS, plus this report | files: contract/README.md, internal/brain/check.go, internal/brain/check_test.go, internal/braincmd/check_test.go, internal/brain/qg_config_step_test.go, internal/cli/qg_setup_lines_test.go, skills/setup/SKILL.md, skills/brain-keeper/SKILL.md, docs/lanes/C-qg-setup-topics-and-scan.md (runbook-required) |

## What changed

- skills/setup/SKILL.md:19 now says `grid project scan <folder>` and `grid project scan <folder> --apply`. Line 34 lists `decisions` and `conventions` as the topics, sends environment and deploy settings to the `--answers` file, and makes any other environment fact a `conventions` note. The Git-authors sentence stays at :36, unchanged (it already says the list goes in the last message).
- internal/brain/check.go: new `configTopicSteps` map next to `configProposalTopics`; the finding reads `config: <step>`: environments and deploy `set it with grid setup <project> --answers <file>`, agents `run grid agent set`, routines `run grid routine add`, workflows `run grid workflow write`, people `people join through the Grid app; grid person show lists them`, machines `machines join from the Grid app's Devices page`.
- The two pins (internal/brain/check_test.go TestCheckConfigTopic, internal/braincmd/check_test.go TestCheckProposalByID) carry the new want strings. TestCheckConfigTopic now holds a topic-to-step table of its own.
- skills/brain-keeper/SKILL.md:29 and contract/README.md:631 say the finding names the step to take.

## Notes

- `people` and `machines` have no write verb in the plugin, so their finding points at the Grid app.
- A topic added to `configProposalTopics` without a step gives `config: ` with nothing after it; TestQgConfigFindingNamesAVerb fails on that.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Whether Claude, following the new setup text, proposes sensible `conventions` notes needs a live setup run.
