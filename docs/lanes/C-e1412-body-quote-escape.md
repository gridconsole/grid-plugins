# C-e1412-body-quote-escape

Brief: the Job body quoting rule named a literal apostrophe where the JSON escape `\u0027` (backslash, u, 0, 0, 2, 7) was meant (end review 1412Z M1, S3). Base 8103dd82.

## Proof table

| Row | Result |
|---|---|
| 1. agents/orchestrator.md:27 and skills/plan-jobs/SKILL.md:47 hold the six-character escape | HOLDS, `grep -n 'u0027' agents/orchestrator.md skills/plan-jobs/SKILL.md` prints both lines: `orchestrator.md:27: ... write it as the JSON escape `\u0027` inside the single-quoted body ...` and `SKILL.md:47: ... A single quote in the text is the JSON escape `\u0027`; or write the body ...`, exit 0 |
| 2. step 5 and SKILL bullet tests require the six characters (Go raw string const `r16Escape`, internal/cli/r16_body_quoting_test.go:22); `TestR16EscapeIsSixCharacters` guards the constant; stdin row feeds a body with the escape and checks the decoded text | HOLDS. On base prose (`git checkout 8103dd82 -- agents/orchestrator.md skills/plan-jobs/SKILL.md`, then `go test -count=1 -run TestR16 ./internal/cli/`, exit 1): `--- FAIL: TestR16OrchestratorTextSaysSingleQuotesForTheBody`: `step 5 lacks "\\u0027"`; `--- FAIL: TestR16PlanJobsSkillSaysSingleQuotesForTheBody`: `skills/plan-jobs/SKILL.md lacks "\\u0027"`. After restoring the lane files: `go test -count=1 -v -run 'TestR16|TestQeShowLine' ./internal/cli/` all PASS, exit 0 |
| 3. bash single-quoted body with the escape, piped to `grid job post ... --body -` | HOLDS, `TestR16SingleQuotedBodyWithEscapeSurvivesTheShellAndPosts`: runs `bash -c "printf '%s' '<body>'"`, exit 0 and output equal to the body; a control with a literal apostrophe makes bash fail; the output is fed as stdin to `job post J-3 --type job --body -`, exit 0, and the posted text reads "the owner's text". It is a Go test that builds the shell line and runs real bash (skips when bash is missing). PASS |
| 4. rename TestQeOldShowLineFailsOnCancelledJob | HOLDS, now `TestQeShowLineExitsZeroOnCancelledJobFromItsFeedRecord` (qe_job_read_lines_test.go:113); body unchanged; PASS. Only C-r15-job-show-unsaved.md still names the old name, as history |
| 5. lane doc | HOLDS, this file |
| `go test ./internal/cli/...` | HOLDS, `go test -count=1 . ./internal/cli/... ./cmd/grid/... ./lib/... ./internal/job/...` exit 0 (root ok, internal/cli ok 38s, lib ok, internal/job, frontmatter, jobcmd, lint ok; cmd/grid and jobtest have no test files). Importers from `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...`: the root package (embeds the prose) is imported by internal/cli, internal/job, internal/job/jobcmd; internal/cli by cmd/grid and lib |
| `make dist` builds | HOLDS, exit 0: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64 all built; this is the Windows and macOS build check |
| `make lint` | HOLDS, `go vet ./...` exit 0 |
| Walk tests | HOLDS, `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok; `internal/job/lint` ok (in the run above). `make webui-test`, `TestFvArgvCollects` and the guest walk tables are app-repo checks: not run, not asked |
| `git fetch origin && git merge origin/main` before last run | HOLDS, "Already up to date" |
| No other file changes | HOLDS, `git diff --stat origin/main..HEAD`: the two prose files, r16_body_quoting_test.go, qe_job_read_lines_test.go, and this doc |
| Whole-module test, cross-repo set | not run, not asked (orchestrator runs them at landing) |

## What changed

- The two prose files now say the escape as `\u0027`, the six characters, so an agent that copies it keeps its single-quoted body intact.
- r16_body_quoting_test.go: the step 5 and SKILL checks want the six characters; the stdin row replaces "owner's" with the escape (the old `ReplaceAll` was a no-op) and checks the posted text is decoded; a new bash test runs the real shell line.
- One test renamed, no assertion change.

Note: the stdin row and the bash test pass on base prose too, since they test the CLI and the shell, not the prose; the two prose tests are the ones that fail on base.

## NOT VERIFIED on a live screen

Nothing here is UI. Not checked: that an agent run by Claude copies the six characters from the prose rather than normalising them to an apostrophe (my own edit tool did that until I wrote `\` for the backslash).
