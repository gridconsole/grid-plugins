# R-land-job-type: lane proof

This lane was research and a plan only. Its one change is `docs/research/land-job-type.md`. The code that differs from the lane base came in with the `origin/main` merge, which brought the `internal/account` codex-read files.

| Row | Result | Proof |
|---|---|---|
| Q1: what the plugin has after review, and what landing lacks | HOLDS | `docs/research/land-job-type.md` section 1, gaps M1-M6 with path:line |
| Q2: every script rule placed in the land Job, the brain, or gone | HOLDS | section 2 table, each row cites a script line |
| Q3: land Job design (posting, deps, runner with no agent, red paths) | HOLDS | section 3; speculative stacking picked with check times from the land logs (5-12 min) and 12 cores (`reserve-cpu.sh:3`) |
| Q4: CPU for land Jobs before builders, without a cap | HOLDS | section 4: agent spawns get a higher nice, nothing is capped; a negative nice is left as an owner decision |
| Q5: moving this build onto Grid, steps and the 23 Nov cutover | HOLDS | section 5 table and order of steps |
| Q6: lane plan with repo, model, files, acceptance rows and deps | HOLDS | section 6 table (L1-L6, brain notes) |
| Doc's last line is exactly DONE | HOLDS | `tail -n 1 docs/research/land-job-type.md` prints `DONE` |
| Doc copied to the hand folder as R-land-job-type.md | HOLDS | `cp` exit 0 |
| No code change by this lane | HOLDS | `git diff --stat 2fdfe3eb HEAD`: only the doc, besides the files from the main merge |
| No personal paths, hosts or emails in the doc | HOLDS | a grep for home paths, server paths, account names and `@` printed nothing (exit 1) |
| No guard, gate, cap or hold proposed for agents | HOLDS | sections 3-4 propose a niceness change and dependencies only; batch caps, hold lists and CPU pinning are marked to go |
| Scripts in the hand folder not edited | HOLDS | only read with Read, grep and git |
| No push, no sudo, nothing touched outside the clone and the hand folder | HOLDS | command history of this session |
| No attribution lines in commits | HOLDS | `git log -2 --format=%B` |
| `git fetch origin && git merge origin/main` | HOLDS | merge exit 0, main 62876ff3 merged |
| `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0, `ok internal/cli 0.653s` |
| `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0, `ok internal/job/lint 0.370s` |
| make lint | not run, not asked (docs-only lane) | |
| GOOS=windows / GOOS=darwin builds | not run, not asked (docs-only lane) | |
| Bundle in the hand folder | HOLDS | `git bundle create` exit 0 |

## What changed

- One new doc: `docs/research/land-job-type.md`.
- This lane doc.

## NOT VERIFIED

- Peak arrival rate of accepted lanes per hour: the doc estimates at least about 5 per repo from the Aint1789 count.
- The CFS weight ratio for nice 10 on this kernel build.
- Whether a routine can post a Job on another task.
- Whether `grid land change` collides with how `grid land` parses its arguments today. L2 checks this.
- Whether the app shows per-Job cost the way the run log does.
- Readiness for the 23 Nov cutover depends on L1-L5 landing. None of them exist yet.
