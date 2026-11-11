# O-render-role-instructions

Base: ba5ddfa3 (main with lane-I-realrun-first-job merged in). Head: see the report.

## Proof table

| Row | Result |
|---|---|
| 1. Test fails on base | HOLDS. `go test -run TestBuilderBriefCarriesItsResultStep ./internal/job/` on base ba5ddfa3 (new test file, no code change): exit 1, `render_role_test.go:44: a builder brief does not tell the agent to post its result with 'grid job result'`; also `TestRoleInstructionsSection`: `builder: 0 Instructions sections`. |
| 2. builder/reviewer/orchestrator/curator get one `### Instructions` with the agent body; `grid-builder` and `Builder` resolve the same; no-file roles get nothing and a byte-identical brief | HOLDS. internal/job/render.go:255 (`AgentFor(contract.NormalizeRole(j.Role))`), embed.go `AgentFor`. Tests `TestRoleInstructionsSection` (three spellings per role, count 1, body match, placed after `## Job`) and `TestRoleWithoutAgentFileGetsNoSection` (person, researcher, scout, checker, operator, designer: no section, brief equals the base `writer` render apart from the role line); existing `TestBriefGolden` (role writer) still passes. `go test -count=1 ./internal/job/...`: exit 0. |
| 3. agents/curator.md | HOLDS. agents/curator.md: front matter name/description/model/effort/tools/skills `[brain-keeper]`, body 14 lines (limit 25), names keep, show --proposal, decide, index, job result, never asks a person. `TestCuratorAgentFileAndItsVerbs` reads contract/commands.json and asserts brain keep/show/decide/index and job result exist: pass. |
| 4. inputs.json and brain_tokens unchanged | HOLDS. `TestInstructionsAreNotASourceOrBrainText`: inputs JSON and BrainTokens equal the base render for all four roles: pass. |
| 5. Package set | HOLDS for job, packaging and every importer; 2 cli failures are on base too. See below. |
| Lint / vet | HOLDS. `gofmt -l .` empty; `go vet . ./internal/job/...` exit 0. |
| Windows / macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No guard/gate/cap | HOLDS. Nothing added on agents; the render only adds text. |
| Existing tests untouched, none deleted | HOLDS. Only the new file internal/job/render_role_test.go; builder.md, reviewer.md, orchestrator.md and skills/ untouched. |
| Public repo hygiene | HOLDS. No personal paths, hosts or emails added; commits carry no attribution lines. |

## Test runs

Packages changed: root (`embed.go`), `internal/job`. `go list` importers of `internal/job`: `internal/cli`, `internal/job/jobcmd`, `internal/job/lint`, `internal/jobgraph`, `internal/lease`, `internal/orchestrator`, `internal/ports`; root is imported by `internal/job` only.

`go test -count=1 . ./internal/job/... ./internal/packaging/ ./internal/jobgraph/ ./internal/lease/ ./internal/orchestrator/ ./internal/ports/`: all ok, exit 0.

`go test -count=1 ./internal/cli/`: exit 1, two failures, both also failing on base ba5ddfa3 (checked with the change stashed):
- `TestTaskNewThenQuestionAdd` (task_identity_test.go:35)
- `TestTaskNewFeedCommands` (task_test.go:73: `next: 3 first record of a tenure must be handover`)
They come from the stacked lane-I merge, not from this lane. The orchestrator should look at them when landing I. Row 5 asks for `./internal/cli/` 0 fail: NOT MET on this stack because of those two base failures; every other cli test passes.

## What changed

- agents/curator.md (new): the curator agent.
- embed.go: embeds agents/*.md; new `AgentFor(role)`; `OrchestratorAgent` kept (no caller left in internal/job; kept as the brief allows).
- internal/job/render.go: the Instructions section now uses `AgentFor(contract.NormalizeRole(j.Role))`.
- internal/job/render_role_test.go (new): the tests above.

## NOT VERIFIED on a live screen

Nothing in this lane has a UI. Not run: a real worker starting `claude -p` with the longer brief (orchestrator's live check).

Note for the orchestrator: M-jobskills edits builder.md/reviewer.md/orchestrator.md; the render reads those files at build time, so no conflict, but the new test compares against whatever the files hold.
