# C-kb2-job-ask-block-text

A Job agent now tells the task something with `grid job post`, which takes its Job lease, and the texts say the `feed_post` tool refuses every write under a Job lease. Base was plugin main `cf5b259f` (the three kb lanes were ancestors of it: `merge-base --is-ancestor` exit 0 for each). Code head `55fa481c`; the report commit is on top of it.

## Proof table

| Row | State | Evidence |
|---|---|---|
| 1. `TestKbJaNoFeedPostWrite` | HOLDS | `go test -count=1 -v -run 'TestKbJa' ./internal/cli/` exit 0: `--- PASS: TestKbJaNoFeedPostWrite`. Base fails (texts checked out from main, new test kept): `kb2_job_ask_block_walk_test.go:192: agents/builder.md: a paragraph names feed_post and does not say it refuses a Job-lease write: To tell the task something during the Job (a question or a blocker), post it to the task's Feed with the feed_post tool; ...` and `:199: agents/builder.md: a sentence tells the agent to post a question, a finding or a blocker with feed_post`. Also failing on base: agents/reviewer.md and skills/run-job/SKILL.md (two paragraphs). |
| 2. `TestKbJaPostSpansRunUnderJobLease` | HOLDS | Same command: `--- PASS: TestKbJaPostSpansRunUnderJobLease` with subtests `agents/builder.md#0`, `agents/reviewer.md#1`, `skills/run-job/SKILL.md#2`, each exit 0 and one new `post` record for J-1 (harness `assignedExitHarness(t, true)`, GRID_ROLE=worker, GRID_LEASE=J-1, GRID_JOB=J-1, GRID_ATTEMPT=1). Base fails: `kb2_job_ask_block_walk_test.go:219: agents/builder.md: no code span starting `grid job post` with `--type post`, so the walk would pass without running an example from it` (same line for skills/run-job/SKILL.md and agents/reviewer.md). |
| 3. `TestKbJaPersonSpansRunOnTaskEpoch` | HOLDS | Same command: `--- PASS: TestKbJaPersonSpansRunOnTaskEpoch`, three subtests (builder.md, reviewer.md, run-job SKILL.md), each exit 0 under `newAgentTask` + `agentEnv` (GRID_LEASE=J-1 with the Job epoch, GRID_TASK_EPOCH the task epoch), and `grid job tree --json` shows J-2 with role `Person`. I also set GRID_JOB=J-1 and GRID_ATTEMPT=1 (the app gives an agent both); `--person` with GRID_JOB=J-1 and body id J-2 is accepted (orchestrator.go:124-136). Base fails: `kb2_job_ask_block_walk_test.go:247: agents/builder.md: no code span starting `grid job post` with `--person` and `--body <json>`, ...` (and the other two texts). |
| 4. Contract and job pins | HOLDS | `go test -count=1 -v -run 'TestImFeedRunJobNamesFeedPost\|TestSkillAndAgentCommandsExist\|TestSkillExamplesValidate\|TestRunJobSkillHoldsTheRules\|TestKbRa\|TestKbRj\|TestKbRv' ./internal/contract/` exit 0: PASS for TestImFeedRunJobNamesFeedPost, TestSkillAndAgentCommandsExist, TestSkillExamplesValidate, TestRunJobSkillHoldsTheRules, TestKbRaAgentPostsFound, TestKbRaBashLineAllowsFound, TestKbRaFindingNotOnFeedPost, TestKbRjSkillPostsSecurityFound, TestKbRjBuilderAgentPointsAtFound, TestKbRvSkillPostsFound, TestKbRvFoundBeforeVerdict; no FAIL. `go test -count=1 -v -run TestGaFeedPostLine ./internal/job/` exit 0: `--- PASS: TestGaFeedPostLine`. `git grep -n 'feed_post' -- internal/contract/im_run_job_feed_post_test.go internal/job/ga_feed_post_line_test.go` prints nothing on the lane (exit 1, no match); on base it printed `im_run_job_feed_post_test.go:10`, `:15` and `ga_feed_post_line_test.go:15`. |
| 5. Boundary unchanged | HOLDS | `go test -count=1 -v -run 'TestKbJa\|TestMCPFeedPostRefusesJobSideWrites\|TestExitRefusalJobSide' ./internal/cli/` exit 0: `--- PASS: TestMCPFeedPostRefusesJobSideWrites`, `--- PASS: TestExitRefusalJobSide` (7 subtests). `git diff main --stat` before this report: agents/builder.md, agents/reviewer.md, internal/cli/kb2_job_ask_block_walk_test.go, internal/contract/im_run_job_feed_post_test.go, internal/job/ga_feed_post_line_test.go, skills/run-job/SKILL.md: 6 files. This report file, docs/lanes/C-kb2-job-ask-block-text.md, is the seventh path in the bundle, as the runbook requires; it is not product code. feed.go, mcp.go, internal/mcp, internal/orchestrator, contract/schemas, README.md and docs/feed-mcp.md are untouched. |
| 6. Walk tests, lint, cross | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (ok 0.069s); `go test -count=1 ./internal/job/lint/` exit 0 (ok 0.433s); `make lint && make cross` exit 0 (go vet, darwin/arm64 and windows/amd64 vet and test compile). |

## Rules

| Rule | State | Evidence |
|---|---|---|
| Small commits, one sentence, no attribution lines | HOLDS | Two code commits and this report; none carries a Co-Authored-By or session line (the owner's rule wins over the reminder). No personal paths, hosts or emails in the files. |
| New tests fail on base, pass on lane; no existing test edited to pass | HOLDS | The three `TestKbJa` tests: failing lines pasted above. The two existing pins (`im_run_job_feed_post_test.go`, `ga_feed_post_line_test.go`) were edited, not to make a test pass but as the brief orders ("fixed, not deleted"): they pinned the dead word `feed_post`. No test deleted. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | Only markdown texts and tests changed; no Go source outside `_test.go`. |
| Tests of changed packages and of every importer | HOLDS | Changed: internal/cli, internal/contract, internal/job. Importers per `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...`: cmd/grid, lib, internal/{artifact,brain,braincmd,feed,guard,hook,intent,jobgraph,job/jobcmd,job/lint,land,lease,mcp,migrate,orchestrator,ports,question,session,settings,signal,task,updates,usage,where,workflow}, root package and internal/packaging/... (they embed or read the texts). `go test -count=1` on all of them, with the walk tests first, exit 0 (every package `ok`; cmd/grid, internal/updates and job/jobtest have no test files). |
| Repo-wide walk tests of the plugin runbook | HOLDS | `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` ok; `internal/job/lint` ok (row 6). |
| Windows and macOS builds | HOLDS | `make cross` exit 0. |
| `*RealPlugin*` tests | n/a | No row names one. |
| UI rows | n/a | No UI change. |

## What changed

- `agents/builder.md`, `agents/reviewer.md`, `skills/run-job/SKILL.md`: the "To tell the task something during the Job" paragraph now says: post to the Feed with `grid job post`, which checks the Job lease; a message or a blocker you are working around with `--type post` and a body such as `{"ref":"task","kind":"message","text":"<what>"}`; a question only a person can answer as a person Job with `grid job post --person <owner> --task <task> --body <json>`, as the `questions` skill says; a blocker that stops you goes in the result (the verdict, for the reviewer); the feed_post tool refuses every write under a Job lease; feed_types lists the record types; the result still goes with `grid job result`. Reviewer's Bash list also names `grid job post` for a message or a person Job, with `grid job post --type found` kept. In run-job's security paragraph, "is refused" became "refuses every write", so every paragraph naming feed_post says `refuses`.
- `internal/contract/im_run_job_feed_post_test.go`: `imFeedWords` is now `Feed`, "`grid job post", `--type post`, `feed_types`, "`grid job result`"; comment says why. `internal/job/ga_feed_post_line_test.go`: wanted words `grid job post`, `--type post`, `grid job result`.
- `internal/cli/kb2_job_ask_block_walk_test.go` (new): walks every file in agents/ and every skill an agent lists under `skills:`. Row 1 checks each paragraph that names feed_post, per sentence, for `refuses` and `Job lease`; row 2 and 3 run each matching `grid job post` span through the harness. Each of the three Touches texts must hold a span of each kind, so the walk cannot pass empty.
- Owner question (person Job or only `decisions_needed`): built the default, a person Job, as skills/questions/SKILL.md:18 says.

## NOT VERIFIED

- That a live app-started agent follows the new text; the walk runs the spans through the plugin's CLI harness with the env the brief names, not through the app's worker (job.go:369-383 in the app repo was not read).
- The body example `{"ref":"task","kind":"message","text":"<what>"}` is not itself run (it is not a `grid job post` span); the same shape (`exitMessage`) is what the row 2 walk posts.
- The sentence splitter in row 1 cuts on `. `, `! ` and `? `; a future sentence naming feed_post that holds a full stop inside a code span could be read as two sentences.

## Budget

The 60-call budget was passed (about 66 calls, many of them parallel reads and one batched proof run). I finished all rows rather than hand off a NOT DONE row; no work-in-progress bundle was written at call 35.
