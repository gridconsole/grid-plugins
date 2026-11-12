# C-se4-render-fixtures-with-agent: the render fixtures recaptured for the brain agent block

Stacked on C-se2-step-who-fixtures (docs/lanes/C-se2-step-who-fixtures.md, docs/lanes/C-se-step-who-role.md). Merged origin/main (holds C-sa2).

## Why

C-sa-brain-agent-render renders the role's brain agent into the brief. C-se2 adds knowledge/agents/grid-writer.md to the jobtest home (internal/job/jobtest/jobtest.go), so the `writer` fixtures now show a `### Instructions` block. Orchestrator decision 02:03 UTC: that block is C-sa's intended behaviour, so the fixtures are recaptured.

## Merge

`git merge origin/main`: one conflict, `agents/orchestrator.md` (step 4 and step 5 of the orchestrator list). Resolved by keeping this lane's step 4 (with the role-agent sentence) and main's step 5 (`grid job post --task <task> --type job --body <json>`). `skills/plan-jobs/SKILL.md` and `internal/job/jobcmd/command.go` merged automatically.

## Failure before the recapture

```
--- FAIL: TestFormatSavedRenderWriter (0.04s)
    --- FAIL: TestFormatSavedRenderWriter/full (0.02s)
        formats_test.go:219: ../fixtures/fmt-job-render/valid/full.json differs from the writer capture; inspect before recapturing
    --- FAIL: TestFormatSavedRenderWriter/elided (0.02s)
        formats_test.go:219: ../fixtures/fmt-job-render/valid/elided.json differs from the writer capture; inspect before recapturing
FAIL	github.com/gridconsole/grid-plugins/contract/formats
```

## What changed

`go test ./contract/formats/ -run TestFormatSavedRenderWriter -count=1 -update-format-fixtures`. `git diff main -- contract/fixtures/fmt-job-render` shows two files, 4 lines each, and only:

- full.json and elided.json `brief`: under `## Job`, after `Document the export format`, the block `### Instructions\n\nWrites documentation.\n\n` before `- Write docs/export.md`.
- `brief_bytes` 2108 -> 2149 (full), 1875 -> 1916 (elided).

No other line changed (`inputs.generation`, sources, `brain_tokens`, `pasted_chars` are unchanged).

## Proof table

| Row / rule | Result |
|---|---|
| Merge, files resolved | HOLDS: agents/orchestrator.md (above); commit bc103638 |
| TestFormatSavedRenderWriter red before | HOLDS: failure quoted above, exit 1 |
| Recapture diff only the Instructions block and brief_bytes | HOLDS: `git diff main --stat -- contract/fixtures/fmt-job-render` 2 files, 4+ 4-; diff read line by line |
| TestFormatSavedRenderWriter green after | HOLDS: `go test ./contract/formats/ -run TestFormatSavedRenderWriter -count=1 -update-format-fixtures` ok; later `go test -count=1 ./contract/...` ok |
| `go test ./contract/... ./internal/job/... -count=1` | HOLDS, exit 0: contract ok, contract/formats ok, internal/job ok, frontmatter ok, jobcmd ok, jobtest (no tests), lint ok |
| `make lint` | HOLDS, exit 0 (`go vet ./...`) |
| `make cross` | HOLDS, exit 0 (windows/amd64 and darwin/arm64 vet and test compile; ends `ok github.com/gridconsole/grid-plugins`) |
| C-se2 row 1 `go test -count=1 ./internal/job/... ./internal/orchestrator/...` | HOLDS, exit 0: orchestrator `ok 179.379s` (job packages above) |
| C-se rows 1-3 `go test -count=1 -v -run TestSe ./internal/job/...` | HOLDS: `--- PASS:` TestSeKnownRoleNoProject, TestSeLintBrainAgent, TestSeRoleWithoutAgentWarns, TestSeRolesKnown |
| C-se row 4 `go test ./internal/job/... ./internal/workflow/` | HOLDS: workflow ok, job packages ok; lint_test.go and skill_example_test.go unchanged |
| C-se row 5 git grep | HOLDS: agents/orchestrator.md:25 and skills/plan-jobs/SKILL.md:60, one line each |
| C-se row 6 diff names only Touches | HOLDS: `git diff main --stat` lists agents/orchestrator.md, the two fixtures (this brief's added Touches), skills/plan-jobs/SKILL.md, command.go, lint.go, jobtest.go, the two se test files and the lane reports |
| Importers of changed packages | HOLDS, exit 0: contract/formats, internal/cli (57.7s), internal/job/..., internal/plan, internal/workflow, internal/orchestrator (go list: importers of jobtest/lint/jobcmd) |
| Walk tests | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| New tests fail on base, pass here | No new test in this lane; the fixture test TestFormatSavedRenderWriter was red before the recapture (quoted) and is green after |
| Existing tests edited or deleted | HOLDS: none; only the two fixture files changed |
| No guard, gate, cap or hold on agents | HOLDS: a warning and fixtures only |
| RealPlugin tests | Not applicable: no row names one |

## Not verified on a live screen

Nothing here is UI.
