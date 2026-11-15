# C-r10-plan-verify-preexisting-fail

A plan's verify names the tests in scope when the base suite is already red (real run r10 P2).

## Where the verify is written (step 1)

- `agents/orchestrator.md:26`: step 4 sends the orchestrator to the `plan-jobs` skill checks before a Job is posted ("verify command").
- `skills/plan-jobs/SKILL.md:24`: check 7 "Verify" in the list of checks run on each Job. This is where the rule now stands.
- `skills/plan-jobs/SKILL.md:56`: the `## Verify` section of the plan.md example (the plan's verify line that `intent.goal` points at).

## Rule wording (step 2, `skills/plan-jobs/SKILL.md:24`)

> Before you name a whole-suite verify (`go test ./...`), run it once on the base; when it is already red, name the tests the change touches in verify and write the pre-existing failure in the Job's out of scope, so no builder is rejected for it.

It is one sentence appended to an existing check. It adds no gate, guard, cap or hook.

## Proof table

| Row | Result |
|---|---|
| 1. Find where the verify is written | HOLDS: `agents/orchestrator.md:26`, `skills/plan-jobs/SKILL.md:24` and `:56` |
| 2. Rule added, one or two lines, no gate | HOLDS: `skills/plan-jobs/SKILL.md:24`, one sentence |
| 3. Rule wording shown, test that holds it | HOLDS: `TestR10PlanJobsVerifyPreexistingFail`, `internal/job/lint/r10_verify_preexisting_fail_test.go`. No plan-jobs skill eval in `evals/suite` covers a verify line, so the test is the holder, in the style of `TestR6PlanJobsShipPostedByDeliver` |
| New test fails on base | HOLDS: with `skills/plan-jobs/SKILL.md` reverted, `go test -count=1 ./internal/job/lint/ -run TestR10PlanJobsVerifyPreexistingFail` exit 1: `--- FAIL: TestR10PlanJobsVerifyPreexistingFail` / `skills/plan-jobs/SKILL.md: the Verify check does not hold: Before you name a whole-suite verify ...` |
| New test passes on the lane | HOLDS: `go test -count=1 ./internal/job/lint/` exit 0 |
| `make lint` | HOLDS: exit 0 (`go vet ./...`) |
| Skill tests that exist pass | HOLDS: `go test -count=1 ./internal/plan/ ./internal/cli/ ./internal/job/...` exit 0 (includes `TestFfAnchorFormsSkillExampleResolves`, `TestR6PlanJobsShipPostedByDeliver`, skill_example_test) |
| Packages that import the changed ones | HOLDS: changed are the root package (embeds `skills/`) and `internal/job/lint`; importers from `go list`: `internal/cli`, `internal/job`, `internal/job/jobcmd`, `cmd/grid`, `lib`. `go test -count=1 . ./internal/job/... ./cmd/grid ./lib ./internal/orchestrator ./internal/jobgraph ./internal/plan ./internal/cli` all ok, exit 0 |
| Walk test `./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS: ok, exit 0 |
| Walk test `go test -count=1 ./internal/job/lint/` | HOLDS: ok, exit 0 |
| `git fetch origin && git merge origin/main` before the last run | HOLDS: "Already up to date" |
| Windows build `GOOS=windows CGO_ENABLED=0 go build ./...` | HOLDS: exit 0 |
| macOS build `GOOS=darwin CGO_ENABLED=0 go build ./...` | HOLDS: exit 0 |
| Whole-module test, cross-repo set | not run, the orchestrator runs them at landing |
| `make webui-test`, app guest walk tables | not run, not asked (app-repo checks; this is the plugin repo) |
| RealPlugin tests | not run, not asked |
| No personal paths, hosts or emails in the diff | HOLDS: the diff holds the skill sentence, the test and this doc |

## What changed

- `skills/plan-jobs/SKILL.md`: check 7 gains the base-red rule.
- `internal/job/lint/r10_verify_preexisting_fail_test.go`: new test that holds the sentence on the `7. Verify:` line and proves it bites (the text with the sentence cut out fails).
- `docs/lanes/C-r10-plan-verify-preexisting-fail.md`: this table.

## Not verified

- The rule changes what an orchestrator model writes. Whether it now names the touched tests on an r10-style seed (failing `TestTotalWithDiscount`) needs a live eval run (`evals/suite/fix-total` is the nearest seed); not run here.
