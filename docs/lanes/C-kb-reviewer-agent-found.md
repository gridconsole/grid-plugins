# C-kb-reviewer-agent-found

The reviewer agent names the `found` post, and its Bash line allows it.

## Proof table

| Row | Result |
| --- | --- |
| 1. `TestKbRaAgentPostsFound` | HOLDS. `go test -count=1 -v -run 'TestKbRa' ./internal/contract/` prints `--- PASS: TestKbRaAgentPostsFound`, exit 0. On base it failed: `kb_reviewer_agent_found_test.go:64: agents/reviewer.md: no numbered step holds a code span starting `grid job post` with `--type found` and `--job <review-job>``. The step is agents/reviewer.md:19; the `grid job result` step is :20. |
| 2. `TestKbRaBashLineAllowsFound` | HOLDS. Same command prints `--- PASS: TestKbRaBashLineAllowsFound`. On base: `kb_reviewer_agent_found_test.go:82: ... the "Use Bash only" sentence does not name `grid job post --type found`, so the agent's own rule forbids the post`. Now agents/reviewer.md:11. |
| 3. `TestKbRaFindingNotOnFeedPost` | HOLDS. Same command prints `--- PASS: TestKbRaFindingNotOnFeedPost`. On base: `kb_reviewer_agent_found_test.go:98: ... a paragraph sends "a finding" to feed_post, which a Job lease refuses`. The feed_post paragraph (agents/reviewer.md:24) still names `grid job result`. |
| 4. `TestSkillAndAgentCommandsExist`, `TestImBash`, `TestDtGateWordsTemplates` | HOLDS. `go test -count=1 -v -run 'TestKbRa\|TestSkillAndAgentCommandsExist\|TestImBash\|TestDtGateWordsTemplates' ./internal/contract/` exit 0, `--- PASS: TestSkillAndAgentCommandsExist`, no FAIL. All flags in the new code span (`--task`, `--job`, `--type`, `--body`) exist for `grid job post` in contract/commands.json:1164-1273. |
| 5. diff names only the Touches files | HOLDS. `agents/reviewer.md`, `internal/contract/kb_reviewer_agent_found_test.go`, plus this report `docs/lanes/C-kb-reviewer-agent-found.md` as the runbook requires. |
| 5. `make lint` | HOLDS. exit 0 (`go vet ./...`). |
| 5. `make cross` | HOLDS. exit 0: darwin/arm64 and windows/amd64 each ran vet and a test compile. |
| New tests fail on base, pass on lane | HOLDS. Failure lines pasted in rows 1 to 3 (agents/reviewer.md stashed, test file kept). |
| No guard, gate, cap or hold on agents | HOLDS. The change adds an allowed command and a step; nothing is blocked. Nothing removed. |
| Package tests, changed packages and importers | HOLDS. Changed: `internal/contract` (test file) and the embedded `agents/reviewer.md` (root package `grid-plugins`). Importers of the root embed and of contract that read the agent text: `internal/job`, `internal/job/...`, `internal/packaging/...`, `internal/cli`, `lib`, `cmd/grid`, `contract/...`. `go test -count=1 . ./internal/contract/... ./internal/job/... ./internal/packaging/... ./internal/cli/ ./lib/ ./cmd/grid/ ./contract/...` exit 0, every package `ok`. |
| Walk tests (plugin) | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` -> `ok  .../internal/cli 4.415s`. `go test -count=1 ./internal/job/lint/` -> `ok  .../internal/job/lint 3.937s` (in the package run above). |
| Windows and macOS builds | HOLDS. Same `make cross` run, exit 0. |
| `*RealPlugin*` tests | Not named by a row; none run. |
| No attribution lines, no personal paths | HOLDS. Commit message is one sentence; no paths, hosts or emails in the files. |

## What changed

- agents/reviewer.md:11: the Bash sentence now also allows `grid job post --type found` for each finding.
- agents/reviewer.md:19: new step 5 posts each security problem and must-fix bug as a `found` record: `grid job post --task <task> --job <review-job> --type found --body <json>`. It lists the body fields from contract/schemas/found.json (`job`, `re`, `at`, `text`, `kind`, `class`) and says the printed ids go in the result's `found` list. The verdict step is now step 6.
- agents/reviewer.md:24: feed_post stays for a question or a blocker; findings go through step 5, since feed_post is refused under a Job lease.
- internal/contract/kb_reviewer_agent_found_test.go: `TestKbRaAgentPostsFound`, `TestKbRaBashLineAllowsFound`, `TestKbRaFindingNotOnFeedPost`.

## NOT VERIFIED on a live screen

- A real reviewer agent posting a `found` record under its Job lease: no claude run and no app in this sandbox. The text is checked against contract/commands.json and contract/schemas/found.json only.
- The `re` field is described as "the id of the builder's result" (the schema only says it is a record id); the app's reading of `re` was not checked.
