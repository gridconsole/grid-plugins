# C-kb-run-job-found-text

A builder posts a security problem it meets as a `found` record (`grid job post --type found`, kind `security`) and lists its id in the result's `found`.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. TestKbRjSkillPostsSecurityFound | HOLDS | `go test -count=1 -v -run 'TestKbRj' ./internal/contract/` exit 0, prints `--- PASS: TestKbRjSkillPostsSecurityFound`; text at skills/run-job/SKILL.md:53. On base it failed: `kb_run_job_found_test.go:46: skills/run-job/SKILL.md: no paragraph holds a `grid job post ... --type found ... --job <job>` span and also names security and the result's `found``, `--- FAIL: TestKbRjSkillPostsSecurityFound`. |
| 2. TestKbRjBuilderAgentPointsAtFound | HOLDS | same command, `--- PASS: TestKbRjBuilderAgentPointsAtFound`; text at agents/builder.md:24. On base it failed: `kb_run_job_found_test.go:57: agents/builder.md: a paragraph still sends a finding to feed_post` and `:61: no `grid job post ... --type found ... --job <job>` span`. |
| 3. Pinned tests | HOLDS | `go test -count=1 -v -run 'TestKbRj\|TestImFeedRunJobNamesFeedPost\|TestSkillAndAgentCommandsExist\|TestSkillExamplesValidate\|TestRunJobSkillHoldsTheRules\|TestSkillRunJobBudgetIsNotAStopRule\|TestImBash' ./internal/contract/` exit 0; PASS lines for `TestImFeedRunJobNamesFeedPost`, `TestSkillAndAgentCommandsExist`, no FAIL. The :51 paragraph is unchanged. |
| 4a. diff names only the Touches files | HOLDS | `git diff main --stat`: agents/builder.md, internal/contract/kb_run_job_found_test.go, skills/run-job/SKILL.md (plus this report under docs/lanes/). |
| 4b. `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| 4c. `make cross` | HOLDS | `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test compile, all packages ok. |
| Small commits, no attribution | HOLDS | one commit, one sentence, no attribution lines. |
| New tests fail on base, pass on lane | HOLDS | failure lines in rows 1 and 2. |
| Existing test untouched, nothing deleted | HOLDS | the diff only adds a file and edits the two texts (one line in builder.md reworded). |
| No guard, gate, cap or hold | HOLDS | text only. |
| Tests of changed packages and importers | HOLDS | The skill and agent files are embedded by the root package, whose importers are internal/cli, internal/job, internal/settings and internal/contract. `go test -count=1 . ./internal/cli ./internal/contract ./internal/job/... ./internal/settings ./internal/hook ./internal/jobgraph ./internal/lease ./internal/orchestrator ./internal/migrate ./internal/mcp` all `ok`, exit 0. |
| Plugin walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. |
| RealPlugin tests | not applicable | no *RealPlugin* test names this lane's text. |

## What changed

- skills/run-job/SKILL.md: a new paragraph after the feed_post one. A security problem the builder meets, in or outside its scope, is posted at once with `grid job post --task <task> --job <job> --type found --body <json>`, kind `security`; its id goes in the result's `found`; other things stay in `proposed_jobs` or `decisions_needed`; the reviewer posts the other kinds.
- agents/builder.md: the feed_post line no longer lists "a finding"; a new paragraph points a security problem at the found post.
- internal/contract/kb_run_job_found_test.go: `TestKbRjSkillPostsSecurityFound`, `TestKbRjBuilderAgentPointsAtFound`.

## Open items

- contract/schemas/found.json requires `job`, `re`, `at`, `text`, `kind`, `class`, and `re` must be a record id. A builder has no obvious record to name there. The brief makes this lane text only, so no body shape is in the skill; it belongs with C-kb-found-worker-role.

## NOT VERIFIED on a live screen

- That a live builder agent follows the new text and that the post path accepts the `found` record under a worker-role Job lease; that path is C-kb-found-worker-role. I did not run the CLI against a lease.
- I did not check the body shape of a `found` record against the schema; the text says `<json>` and the kind `security`.
