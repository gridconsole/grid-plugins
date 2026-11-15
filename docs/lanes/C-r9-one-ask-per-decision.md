# C-r9-one-ask-per-decision: one decision is asked once, and only what the request leaves open is asked

Round 9, F2 and F3 (task T-02 "rename --out to --output", task T-01 "one-line tax fix"). Base: main 6c44c381 (origin/main had nothing new at the merge before the last proof run).

## Cause of J-7 (acceptance 1)

The chain, read from the code at base 6c44c381:

1. `agents/orchestrator.md:24` (step 2) and `:62` tell the first orchestrator Job (J-1) to post a person Job for a decision (J-2 "Decide how the old --out flag is handled"). `agents/orchestrator.md:56` also tells it to "say so in `decisions_needed`" of its result. Nothing told it that a decision it already asked must not be listed as a new one.
2. `internal/orchestrator/decisions_needed.go:30-46` (`abDecisionsNeeded`, pass added by lane C-ab-decisions-orchestrator-job) posts one orchestrator Job for every result whose `decisions_needed` is non-empty. At `:41-42` it reads the decisions and checks only that the same result was not asked before (`abDnPosted`). It never looks for a person Job that already asks the decision. Its title is set at `:133` ("Decisions needed on J-1: ..." is J-7), and its A1 at `:135` says "Ask the person who decides each one with `grid question ask` ... or a person Job".
3. J-7 followed A1. `skills/questions/SKILL.md:18` makes `grid question ask` from the task holder post a person Job, so the same --out decision became Q-1/J-8. J-7 also asked Q-2/J-9 ("Should tally also get -o?"), because neither the orchestrator nor the questions skill said that an option the person did not ask for is not asked (F3; the clamp question of T-01 is the same gap).

Not verified against the round 9 feed: the evidence folders sit outside /srv/gridwork, and Read and `ls` on them were refused in this session. Without them I could not confirm that J-1's result really carried `decisions_needed`. The chain above is derived from the code. Step 2 is the only path in the plugin that posts a Job titled "Decisions needed on <job>" (`grep -rn "Decisions needed on"` finds only decisions_needed.go and its test).

## Proof table

| Row | State | Proof |
|---|---|---|
| Brief A1: the cause of J-7 named with file:line | HOLDS | Section above: decisions_needed.go:30-46, :41-42, :133, :135 at base; orchestrator.md:24, :56, :62; questions SKILL.md:18. Derived from code; r9 feed not readable here. |
| Brief A2: diff of the instruction text and the test; new tests fail on main and pass on the lane | HOLDS | Failing run on base below; passing run `go test -count=1 ./internal/orchestrator ./internal/job/lint -run 'TestR9Oa\|TestAbDn'` exit 0 |
| Brief A3: make lint and the touched package tests pass | HOLDS | `make lint` exit 0 (go vet ./...); `gofmt -l internal/orchestrator internal/job/lint` empty; package run below exit 0 |
| Brief A4: docs/lanes/C-r9-one-ask-per-decision.md | HOLDS | this file |
| Do 3: a person Job and a question on the same decision are caught | HOLDS | `internal/orchestrator/r9_one_ask_test.go` TestR9OaAskedDecisionPostsNoSecondAsk, TestR9OaOnlyOpenDecisionIsAsked (structural rule below) |
| Do 3: text tests for the two instruction changes | HOLDS | `internal/job/lint/r9_one_ask_text_test.go` TestR9OaOrchestratorOneAskPerDecision, TestR9OaQuestionsSkillOneAsk |
| No new guard, gate, cap or limit | HOLDS | The pass posts fewer Jobs and refuses, holds and counts nothing. The Job-count sentence (orchestrator.md:62) is guidance with an example, not a number. |
| Tests of changed packages and of every package that imports them | HOLDS | `go test -count=1 ./internal/orchestrator ./internal/cli ./internal/job/... ./internal/mcp ./internal/migrate ./internal/contract . ./cmd/grid` exit 0 |
| Walk tests: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0 (`ok internal/cli 0.037s`) |
| Walk tests: `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0 (`ok internal/job/lint 0.086s`) |
| `git fetch origin && git merge origin/main` before the last proof run | HOLDS | "Already up to date." |
| Windows build `GOOS=windows go build ./...` | HOLDS | exit 0 |
| macOS build `GOOS=darwin go build ./...` | HOLDS | exit 0 |
| Whole-module test and cross-repo set | not run, not asked | run by the orchestrator at landing |
| RealPlugin tests | not run, not asked | the brief names none |
| No attribution lines, no personal paths or emails | HOLDS | `git log origin/main..HEAD`: one-sentence messages, no trailers. This doc names no host path outside /srv/gridwork. |

Importers of the changed packages, from `go list -f '...{{range .Imports}}...' ./... | grep IMPORTS`:

```
github.com/gridconsole/grid-plugins/internal/cli IMPORTS TESTIMPORTS
github.com/gridconsole/grid-plugins/internal/job/jobcmd TESTIMPORTS
github.com/gridconsole/grid-plugins/internal/mcp IMPORTS
github.com/gridconsole/grid-plugins/internal/migrate IMPORTS TESTIMPORTS
github.com/gridconsole/grid-plugins/internal/orchestrator XTESTIMPORTS
```

internal/job/lint changed only in a `_test.go` file of `lint_test`, so no package imports the change. agents/orchestrator.md and skills/questions/SKILL.md are read by tests in internal/cli, internal/job, internal/job/lint, internal/contract and the root package, and all of them are in the run.

Failing lines on base (new tests, instruction and code not yet changed):

```
--- FAIL: TestR9OaAskedDecisionPostsNoSecondAsk (0.19s)
    r9_one_ask_test.go:32: after the tick: Jobs on the Feed = [J-1 J-2 J-3], want [J-1 J-2]
--- FAIL: TestR9OaOnlyOpenDecisionIsAsked (0.18s)
    r9_one_ask_test.go:50: title = Decisions needed on J-1: Asked in J-2: how the old --out flag is handled, want "Decisions needed on J-1: Which export format: csv or json?"
FAIL	github.com/gridconsole/grid-plugins/internal/orchestrator	0.546s
--- FAIL: TestR9OaOrchestratorOneAskPerDecision (0.00s)
    r9_one_ask_text_test.go:26: agents/orchestrator.md lacks "One decision has one ask"
    r9_one_ask_text_test.go:26: agents/orchestrator.md lacks "a person Job or a question, not both"
    r9_one_ask_text_test.go:26: agents/orchestrator.md lacks "what the person's words leave open and the work cannot go on without"
    r9_one_ask_text_test.go:26: agents/orchestrator.md lacks "left out and named in the result"
    r9_one_ask_text_test.go:26: agents/orchestrator.md lacks "begins with the id of that person Job"
    r9_one_ask_text_test.go:26: agents/orchestrator.md lacks "the size of the change"
--- FAIL: TestR9OaQuestionsSkillOneAsk (0.00s)
    r9_one_ask_text_test.go:43: skills/questions/SKILL.md lacks "One decision has one ask"
    r9_one_ask_text_test.go:43: skills/questions/SKILL.md lacks "what the person's words leave open and the work cannot go on without"
    r9_one_ask_text_test.go:43: skills/questions/SKILL.md lacks "already asks"
    r9_one_ask_text_test.go:43: skills/questions/SKILL.md lacks "left out and named in the result"
FAIL	github.com/gridconsole/grid-plugins/internal/job/lint	0.005s
```

TestR9OaNonPersonJobIdStillAsks passes on base and on the lane. It guards the new rule: a decision that names a build Job or an id no Job has is still asked.

## What changed

- `internal/orchestrator/decisions_needed.go`: `r9OaOpen` (:83-106) drops a decision line that names, as a word, the id of a Job on the Feed whose role is Person. The pass at :42 posts a decisions Job only for the decisions still open; when none are open, it posts nothing. A1 of the decisions Job (:163) now says one decision has one ask: ask nothing a person Job or question on the task already asks (`grid job tree`, `grid question list`), ask only what the person's words leave open and the work cannot go on without, and leave out a new option and name it in the result. It still names `grid question ask` and `grid job result` (pinned by ab_decisions_test.go:127).
  - The structural rule the code checks is "the decision line names the id of a person Job". Code does not judge whether two texts mean the same decision. Text that does not name the person Job is covered by the instruction in A1, which tells the decisions Job's agent to read `grid job tree` and `grid question list` before asking.
- `agents/orchestrator.md:24`: one decision has one ask (a person Job or a question, not both). Ask only what the person's words leave open and the work cannot go on without; a new option (a short flag, a clamp, a new behaviour) is not asked, it is left out and named in the result.
- `agents/orchestrator.md:56`: a decision already asked is listed in `decisions_needed` beginning with the id of its person Job, so the pass above asks it no second time.
- `agents/orchestrator.md:62`: the number of Jobs follows the size of the change (a one-line fix is one build and one review). No number, no cap.
- `skills/questions/SKILL.md:8`: the same one-ask and only-what-is-open rule for anyone who asks a question.
- Why internal/orchestrator is touched: brief step 1 is to find where J-7 comes from. Its list (orchestrator.md, the questions skill, the plan-jobs lint, the review of J-1) is where to look, and the poster turned out to be this pass. A fix in the text alone would leave the pass posting a second ask of a decision the orchestrator listed. The change only removes posts. No gate, hold or refusal is added.
- Not changed: the plan-jobs lint. A Job file holds no link between a decision and a question, so a lint could only guess from wording. The catch sits at the one place that posts the second ask.

## Open items / NOT VERIFIED

- Not checked against a live run: whether an orchestrator agent now names the person Job id in `decisions_needed`, asks no unrequested option, and keeps a one-line fix to one build and one review. That needs a new real run (round 10).
- The cause in acceptance 1 is derived from code. The round 9 feed and question files were not readable in this session.
