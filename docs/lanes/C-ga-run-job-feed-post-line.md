# C-ga-run-job-feed-post-line

Base: c3a4c6e1 (lane-C-ga-skills-one-command-bash proof table is its head commit; its branch name does not exist in this clone, so `git merge-base --is-ancestor` could not be run by name; the commit is HEAD~0 of the base and the stacked start says it is merged). None of the three files named feed_post at base (`git grep feed_post` printed nothing). Head: see the report.

| Row | State |
|---|---|
| 1. one feed_post line per file | HOLDS. `git grep -n feed_post HEAD -- agents/builder.md agents/reviewer.md skills/run-job/SKILL.md`: agents/builder.md:20, agents/reviewer.md:21, skills/run-job/SKILL.md:51. Base: no output. |
| 2. TestGaFeedPostLine | HOLDS. Pass on lane (in the row 4 run, exit 0). On base (agents/skills stashed): `--- FAIL: TestGaFeedPostLine` / `ga_feed_post_line_test.go:17: builder: Instructions do not name feed_post`. Test renders builder and reviewer through renderRole and asserts the text after "### Instructions" names feed_post and `grid job result`. |
| 3. steps 7 and 8 unchanged | HOLDS. `git diff c3a4c6e1 -- skills/run-job/SKILL.md` adds only 2 lines at line 51, after the blocked paragraph, outside the numbered steps. |
| 4. `go test ./internal/contract/ ./internal/job/... -run 'Skill|Role|GaFeedPost' -count=1` | HOLDS, exit 0. |
| New test fails on base, passes on lane | HOLDS (row 2). |
| Packages that import the changed ones | HOLDS. Tests of ./internal/job/... ./internal/cli/ ./internal/hook/ ./internal/jobgraph/ ./internal/contract/ . all ok (importers of internal/job from `go list`: cli, hook, jobcmd, lint, jobgraph). |
| Lint | HOLDS. `make lint` (go vet ./...) exit 0. |
| Windows and macOS builds | HOLDS. `GOOS=windows go vet ./...` exit 0, `GOOS=darwin go vet ./...` exit 0. |
| No guard/gate/cap; only Touches files | HOLDS. Changed: agents/builder.md, agents/reviewer.md, skills/run-job/SKILL.md, internal/job/ga_feed_post_line_test.go (plus this report). Tool names written as plain words. |
| Small commit, no attribution | HOLDS. One commit with a one-sentence message. |
| RealPlugin tests | Not applicable; no row names one. |

## What changed
One sentence in each of the builder and reviewer agent bodies (they arrive in "### Instructions" on every Job) and once in run-job outside the numbered steps, saying to post a question, finding or blocker to the task's Feed with feed_post (feed_types lists record types), while the result or verdict still goes with `grid job result`. New test in internal/job.

## NOT VERIFIED on a live screen
Nothing needs a screen. Not run: the whole-module test and cross-repo set (orchestrator at landing); a live Job agent actually calling feed_post.
