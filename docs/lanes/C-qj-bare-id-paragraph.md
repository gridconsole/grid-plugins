# C-qj-bare-id-paragraph

Base: plugin main 44c7de5bacbac31560de11e84afd4ff15980c05d. No Go product file changed.

## Base check (production code is unchanged, so the lane runs show the base behaviour)

Harness: `newHarness` project `example` with a registered folder and task T-01 made by `task new`; the test runs from the project folder and from a folder in no project.

- `question list --task T-01` from the project folder: exit 0, stdout `No questions.` (it does not print "needs --project", so the lane goes on).
- `plan items --task T-01` from the project folder: exit 1, stderr `plan: lstat .../tasks/T-01-add-search/plan.md: no such file or directory`. The task was found. The fixture has no plan.md.
- From a folder in no project, role owner or unset: `question: task T-01 needs --project: this folder is in no project with a record`.
- From a folder in no project, role member: `question: task not found`. `where.go:268-272` prints "needs --project" only when `PersonalOK()` holds (`where.go:269-270` returns ErrTaskNotFound otherwise). The new sentence says so.
- `job result --task <task> --body {}` with GRID_MEMBER set, no job: `Job id is required` (TestQjJobResultErrors reads it, orchestrator.go:323-324).
- With `--job J-1` and GRID_MEMBER unset: `GRID_MEMBER is required` (orchestrator.go:613-616).

## Proof table

| Row | Result |
|---|---|
| 1. TestQjBareIdClaims red on base | HOLDS. On the base skill text (new test, old skills) `go test -count=1 -v -run TestQj ./internal/cli/` exit 1: `deliver: \`plan items\` bare id from the project folder printed needs --project=false; the sentence says it refuses=true`, `deliver: \`question list\` bare id from the project folder printed needs --project=false; ...`, `questions: \`question\` ...`, `\`answer\``, `\`person\``, `\`plan\``, `research: \`where\``, `\`job show\``, `\`job result\`` (same message), then `--- FAIL: TestQjBareIdClaims`. On the lane: `--- PASS: TestQjBareIdClaims`, `--- PASS: TestQjJobResultErrors`, exit 0. |
| 2. TestQjJobResultErrors red on base | HOLDS. Base: `research quotes "specify a Job id", which \`job result\` was not seen to print (no Job id gave "Job id is required\n"; no GRID_MEMBER gave "GRID_MEMBER is required\n")`, `--- FAIL: TestQjJobResultErrors`. Lane: `--- PASS: TestQjJobResultErrors`. |
| 3. grep and packaging tests | HOLDS. `git grep -n 'specify a Job id' -- skills` prints nothing (exit 1, no match). `go test -count=1 -v -run 'TestSkillCommandsImplemented\|TestErDeliver' ./internal/packaging/ ./internal/cli/` exit 0, includes `--- PASS: TestErDeliverSkillNamesTheDeliverJob` and `--- PASS: TestSkillCommandsImplemented`. |
| 4. lint, cross, diff | HOLDS. `make lint` (go vet ./...) exit 0. `make cross` (darwin/arm64 and windows/amd64 vet and test compile) exit 0. `gofmt -l` on the new file prints nothing. `git diff main --stat`: the three SKILL.md files and `internal/cli/qj_bare_id_claims_test.go`, plus this report (see Open items). |
| Tests of changed packages and importers | HOLDS. `go test -count=1 . ./internal/cli/ ./cmd/grid/ ./lib/ ./internal/packaging/... ./internal/job/... ./internal/jobgraph/ ./internal/lease/ ./internal/orchestrator/ ./internal/hook/ ./internal/migrate/ ./internal/ports/ ./internal/mcp/` exit 0, all ok. The list comes from `go list`: the root package (it embeds skills) is imported by internal/job, and internal/cli by cmd/grid and lib. |
| Walk tests (plugin) | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`walked 39 commands, 39 table entries`, both PASS). `go test -count=1 ./internal/job/lint/` exit 0. |
| New tests fail on base, pass on lane | HOLDS (rows 1 and 2). |
| No existing test edited or deleted | HOLDS. Only the new test file was added. |
| No guard, gate, cap or hold added | HOLDS. Text and one test only. |
| RealPlugin tests | Not needed: no row names one. |
| No personal path, host or email | HOLDS. The new text holds none. |

## What changed

- skills/deliver/SKILL.md:15, skills/questions/SKILL.md:14, skills/research/SKILL.md:16: the bare-id sentence now says a bare id works for every verb below inside the task's project folder; from a folder in no project `proof`, `brain`, `task history` and `deliver` still find it and the named verbs answer "needs --project" (a role with no Personal scope gets "task not found"); the folder path works from anywhere. The `person` note in questions stays.
- skills/research/SKILL.md:23: `grid job result` needs `GRID_MEMBER` and a Job id (`--job <job>` or GRID_JOB); the errors are "GRID_MEMBER is required" and "Job id is required".
- internal/cli/qj_bare_id_claims_test.go: `TestQjBareIdClaims` cuts the sentence from each skill through `gridplugins.SkillFor`, reads which verbs refuse and which find a bare id, runs each verb read-only from the project folder and from a folder in no project (role owner), and checks "needs --project" appears exactly where the sentence says. It also runs the folder path from the no-project folder when the sentence claims it, checks a member role gets "task not found", and checks the three sentences name all 13 verbs. `TestQjJobResultErrors` cuts the quoted errors from research and runs `job result`.

## Notes

- `answer` and `task history` take the task as an operand, not `--task`; the test uses their real shape (`answer <task> Q-1 x`, `task history <task>`). The skills say "all use `--task`"; I left that text as it is (outside the sentence this brief touches).
- `brain` has no `--task` read for `brain topics`; the test runs `brain search x --task T-01`, as research step 2 does.
- Which line fails first on base is `plan items` (the deliver loop runs first); `question list` fails right after it.

## Open items

- docs/lanes/C-qj-bare-id-paragraph.md is a fifth file in the diff, as the builder runbook requires; every earlier lane carries one. Brief row 4 lists four files.

## NOT VERIFIED on a live screen

Nothing here is UI. All rows are proved by tests.

Verdict: DONE
