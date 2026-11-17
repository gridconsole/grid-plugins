# Lane P-judge-land-decide

This is a research lane. It produces one decision doc, `docs/research/judge-land.md`, and builds nothing. Every check below ran on the lane head after `git fetch origin` and `git merge origin/main`; the merge reported "Already up to date".

## Proof table

| Row | Result | Proof |
|---|---|---|
| Brief: the decision doc opens with a decision note covering what was decided, why, and the rejected options | HOLDS | `docs/research/judge-land.md:9-93` |
| Brief Q1: lists the app Judge's posts, logged events and rules, the named tests (judge_rework_newjob, judge_stall, judge_post_request_id, cancelled review), and where each part moves | HOLDS | `docs/research/judge-land.md:94-215` |
| Brief Q2: gives the judgement sequence, says the one-rework default comes from the brain workflow, and lists post types and fields based on attempts | HOLDS | `docs/research/judge-land.md:216-270` |
| Brief Q3: the land Job covers one Job per change, order by "after L lands", rebase, trivial conflicts versus `conflict`, push with lease and launcher retry, `landed` with the main SHA, the status block, and deploy unchanged | HOLDS | `docs/research/judge-land.md:271-330` |
| Brief Q4: covers the runner condition, red bisected within the change's commits, flaky landed with a fix Job, the delete-on-second-red rule with the incident exception, and every land.sh rule mapped to a Go file and test, or dropped with a reason | HOLDS | `docs/research/judge-land.md:331-403` |
| Brief Q5: reconciles L1–L6 (cited at `land-job-type.md:207-213`), and says whether P-land-kind-2 continues as is | HOLDS | `docs/research/judge-land.md:404-426` |
| Brief Q6: the lane plan gives order, repo, model and effort (xhigh), files, rows and dependencies, and maps the owner's acceptance | HOLDS | `docs/research/judge-land.md:427-463` |
| Brief: every answer cites evidence as file:line | HOLDS | each section cites plugin, `app:` or `land.sh:` lines |
| Brief: the last line is exactly DONE or NOT DONE | HOLDS | `tail -n 1 docs/research/judge-land.md` printed `DONE`, exit 0 (line 464) |
| Brief: the doc is copied to the hand folder as R-judge-land.md | HOLDS | `diff` of the doc against the copy, exit 0, no output |
| Brief: nothing is built in this lane | HOLDS | `git diff --stat origin/main..HEAD` shows only the two docs files, exit 0 |
| Rule: walk test, CLI | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` printed ok, exit 0 |
| Rule: walk test, lint | HOLDS | `go test -count=1 ./internal/job/lint/` printed ok, exit 0 |
| Rule: fetch and merge main before the last proof | HOLDS | `git fetch origin` exit 0; `git merge origin/main` "Already up to date", exit 0 |
| Rule: small commits, one-sentence messages, no attribution lines | HOLDS | `git log origin/main..HEAD --format=%B` shows one sentence per commit and no trailer |
| Rule: no personal paths, hosts or emails in committed files | HOLDS | A grep of both docs for machine paths, user names and mail addresses found nothing (exit 1) |
| Rule: no guard, gate, cap, hold or blocking hook added | HOLDS | No code changed. The plan adds none, and states "no cap" for the launcher retry (`judge-land.md`, Q3 step 6) |
| Rule: no push; nothing touched outside the lane's clone and the hand folder | HOLDS | No push was run; files were written only in the clone and as copies in the hand folder |
| Reviewer runbook read and the lane checked against it | HOLDS | The walk tests are pasted above, and the changed files are inside the brief (docs only) |
| `go test ./...` across the whole repo | not run, not asked | — |
| `make` checks, GOOS builds | not run, not asked | — |

## What changed

- `docs/research/judge-land.md`: new. The decision doc.
- `docs/lanes/P-judge-land-decide.md`: new. This file.

## NOT VERIFIED

- The launcher interfaces assumed in Q3 and Q6 lane 6 are not verified. These are the wake on exit, the start conditions (including cross-task "after L lands"), and the new attempt on `push_lease_lost`. The sibling doc from P-run-jobs-decide had not been handed in.
- The app evidence was read from app `main` at the time of writing. Its line numbers may drift.
- The lane-P-run-jobs `internal/runner` references cite commit 43d26ad5, which is not on main. The P-land-kind-2 files cite commit aa0cf14a, which is not on main either.
