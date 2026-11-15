# C-r16-orch-body-quoting

The orchestrator, the plan-jobs skill and the `--body` flag help now say the Job body goes in single quotes, because the shell runs backticks and `$` inside double quotes (round 16: the backticked `grid deliver ...` of J-3's acceptance was lost).

## Proof table

| Row | Result |
|---|---|
| 1. agents/orchestrator.md step 5 has one sentence: single quotes, why, `'` for a single quote, or `--body -` with a `<` redirect from a Write-made file | HOLDS: agents/orchestrator.md:27; the `--body -` route is proved by `TestR16JobPostBodyFromStdinKeepsBackticksAndQuote` (a body with backticks and `'` posted through stdin arrives intact), and a `<` redirect is not among the shapes line 13 bans |
| 2. Same sentence in skills/plan-jobs/SKILL.md after the post-command area | HOLDS: skills/plan-jobs/SKILL.md:48 (new bullet; the deliver line at :47 is untouched) |
| 2. `--body` flag help | HOLDS: internal/cli/orchestrator.go:64; `TestR16BodyFlagHelpSaysSingleQuotes` |
| 3. New tests fail on ff0a1034, pass on the lane | HOLDS: see below |
| 3. Added text holds no "approve", "gate", "ask the person" in the checked lines | HOLDS: `go test ./internal/cli -run TestEr` exit 0 (the sentence sits before "In your first Job", outside the scanned span; SKILL bullet is a new line) |
| Diff touches only the three files and their tests | HOLDS: `git diff --stat origin/main..HEAD` (plus this lane doc) |
| `make test` | HOLDS: exit 0 |
| `make lint` | HOLDS: exit 0 (`go vet ./...`) |
| Packages that import internal/cli (go list) | cmd/grid (no tests), lib: `make test` ok for both |
| Windows build | HOLDS: `GOOS=windows go build ./...` exit 0 |
| macOS build | HOLDS: `GOOS=darwin go build ./...` exit 0 |
| Walk tests: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS: ok |
| Walk tests: `go test -count=1 ./internal/job/lint/` | HOLDS: ok |
| `git fetch origin && git merge origin/main` | HOLDS: "Already up to date" |
| Brief did not name webui, app walk tests, `make realplugin` | not run, not asked (plugin repo) |
| Never push, curl, sudo, cp; no guard/cap/limit | HOLDS: none used or added |

### New tests on ff0a1034 (source edits stashed, test file kept)

```
--- FAIL: TestR16OrchestratorTextSaysSingleQuotesForTheBody
    r16_body_quoting_test.go:31: step 5 lacks "single quotes"
    r16_body_quoting_test.go:31: step 5 lacks "backticks"
    r16_body_quoting_test.go:31: step 5 lacks "--body -"
--- FAIL: TestR16PlanJobsSkillSaysSingleQuotesForTheBody
    r16_body_quoting_test.go:43: skills/plan-jobs/SKILL.md lacks "single quotes"
    r16_body_quoting_test.go:43: skills/plan-jobs/SKILL.md lacks "backticks"
--- FAIL: TestR16BodyFlagHelpSaysSingleQuotes
    r16_body_quoting_test.go:56: job post --help lacks "single quotes": ...
    r16_body_quoting_test.go:56: job post --help lacks "backticks": ...
```

On the lane all four `TestR16*` pass. `TestR16JobPostBodyFromStdinKeepsBackticksAndQuote` passes on base too: it pins the behaviour the new text names.

## What changed

- agents/orchestrator.md step 5: the post command stays literal (`--body <json>`, which `TestQdJobPostLinesPostJobs` runs as written); the sentence after it says to write the body in single quotes and why, `'` for a single quote, and `--body -` with a `<` redirect from a file written with Write.
- skills/plan-jobs/SKILL.md: a new bullet after the deliver-Job bullet, same advice.
- internal/cli/orchestrator.go: `--body` help.
- internal/cli/r16_body_quoting_test.go: four tests.
- docs/lanes/C-r16-orch-body-quoting.md: this doc.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: a live orchestrator agent posting a deliver Job with a backticked acceptance through a real shell; the tests prove the text and the stdin route, not the model's behaviour.
