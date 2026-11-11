# C-im-run-job-feed-post-pin: proof table

Base: main ee8a96c126781462cbe0067771db40d01911cd76. Lane head: see the bundle (`git log -1 lane-C-im-run-job-feed-post-pin`).

Base check: skills/run-job/SKILL.md:51 holds the sentence "To tell the task something during the Job (a question, a finding, a blocker), post it to the task's Feed with the feed_post tool; feed_types lists the record types. The result still goes with `grid job result`." `internal/job/ga_feed_post_line_test.go:8-20` reads only the rendered builder/reviewer Instructions (agent files). `grep -rn "feed_post\|feed_types\|run-job/SKILL" --include=*_test.go` found no test that reads the run-job sentence: the run-job readers (internal/contract/skill_examples_test.go:72,94,135,171, internal/contract/skill_run_job_budget_test.go:10, internal/hook/stop_command_test.go:58) name no Feed word. So no test pinned it.

| Row | State | Proof |
|---|---|---|
| A1 `go test ./internal/contract/ -run TestImFeed -count=1 -v` | HOLDS | exit 0: `--- PASS: TestImFeedRunJobNamesFeedPost (0.00s)` / `ok github.com/gridconsole/grid-plugins/internal/contract 0.017s` |
| A2 delete SKILL.md:51 | HOLDS | new test exit 1: `im_run_job_feed_post_test.go:40: run-job (skills/run-job/SKILL.md): no paragraph names all of ["feed_post" "Feed" "feed_types" "`grid job result`"]; the closest paragraph lacks ["feed_post" "Feed" "feed_types" "`grid job result`"]`; `go test ./internal/job/ -run TestGaFeedPostLine -count=1` exit 0 (`ok .../internal/job 0.019s`) on the same tree; `git checkout -- skills/run-job/SKILL.md`, `git status --short` then showed only the new test file. Note: the closest paragraph lacks all four because :18 writes `` `grid job result --task ...` `` and so never holds the exact backticked token `` `grid job result` ``. |
| A3 `feed_post` -> `feed_write` on :51 only | HOLDS | new test exit 1: `run-job (skills/run-job/SKILL.md): no paragraph names all of [...]; the closest paragraph lacks ["feed_post"]`; restored with `git checkout -- skills/run-job/SKILL.md`, `git status --short` clean apart from the new test. |
| A4 diff, tests, vet | HOLDS | `git diff main --stat` lists only internal/contract/im_run_job_feed_post_test.go and docs/lanes/C-im-run-job-feed-post-pin.md (DIFFSTAT); `go test ./internal/contract/ ./internal/job/ -count=1` exit 0 (contract 2.450s, job 3.407s); `go vet ./internal/contract/` exit 0, no output. |
| New test fails on base | HOLDS (by mutation) | The pinned sentence is already on main, so the test passes on base ee8a96c1 on purpose: it is a pin. The failing lines are the A2 and A3 runs above. |
| No existing test edited or deleted | HOLDS | `git diff main --stat` shows only the two new files. |
| Only Touches files changed; no production or skill text change | HOLDS | as A4; SKILL.md mutations were restored with `git checkout --` before every commit. |
| Names: package contract, `imFeed*` package names, `TestImFeed*` test, readPluginFile | HOLDS | internal/contract/im_run_job_feed_post_test.go:1,13,15,18,28,29 |
| Importer tests | HOLDS | `go list -f '{{.ImportPath}} {{.Imports}}' ./...` shows internal/contract is imported by artifact, brain, braincmd, cli, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, question, session, settings, signal, task, usage, where, workflow (all under internal/). `go test -count=1` on those 25 plus ./internal/contract/ exit 0, every package `ok` (cli 110.461s, orchestrator 147.666s). The change is a _test.go file, so no importer compiles differently. |
| Lint | HOLDS | `make lint && make cross` exit 0; cross runs only after lint (`go vet ./...` + gofmt -l) exited 0. |
| Windows and macOS builds | HOLDS | same run exit 0; output shows `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`, no FAIL. |
| No guard, gate, cap, hold or blocking hook | HOLDS | test file only; no production code. |
| RealPlugin tests | N/A | no *RealPlugin* test is named in the brief or added. |
| No attribution lines; no personal paths, hosts or emails | HOLDS | `git log origin/main..HEAD --format=%B` shows one-sentence messages only; the test file holds only the repo-relative path skills/run-job/SKILL.md. |
| Runbook-reviewer read | HOLDS | /srv/gridwork/tmp/hand/runbook-reviewer.txt read; every row above gives a command with its exit code or a file:line. |

## What changed

- New internal/contract/im_run_job_feed_post_test.go: `TestImFeedRunJobNamesFeedPost` splits skills/run-job/SKILL.md on blank lines and passes when one paragraph holds all of `feed_post`, `Feed`, `feed_types` and `` `grid job result` ``. If none does, it fails naming run-job, the file and the words the closest paragraph lacks. Only the words are pinned, not the sentence.
- This proof table.

## NOT VERIFIED

Nothing needs a live screen. Whole-module test and the cross-repo set are left to the orchestrator at landing.

DONE
