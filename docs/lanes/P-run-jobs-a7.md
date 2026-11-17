# P-run-jobs-a7: proof table

This lane folds owner Addendum 7 (production actions; Job code on hidden refs) into `docs/research/run-jobs-launcher.md`. It is research only. No code changed.

| Row | Status | Proof |
|---|---|---|
| Do 1: section "Addendum 7" deciding A and B with file:line evidence in the plugin and the app ref | HOLDS | `docs/research/run-jobs-launcher.md:458` (section), `:465` (A, V26), `:533` (B, V27). Evidence cited includes `internal/routine/execute.go:192-199`, `internal/runner/runner.go:95-96, 160-161, 255-256, 272-296, 388-393`, `internal/cli/run.go:34, 57`, `internal/cli/worker.go:317-336`, `internal/orchestrator/stream.go:331-337`, `internal/land/land.go:117, 176, 265-267`, `internal/workflow/parse.go:234-240, 441-451`, `contract/commands.json:866, 2525`, `contract/schemas/submit.json:33-41, 68-76`, `internal/brain/ln_secret.go:3-13`; app main @e60fab587 `internal/kernel/worker/agentbuild.go:150-169, 234-235, 552-598`, `internal/kernel/worktree/worktree.go:153-167, 248-252`, `internal/slices/settings/agent.go:26, 33` |
| Do 1, B: what replaces today's lane branches and land path | HOLDS | B1 table (`run-jobs-launcher.md`, "B1. What this replaces") |
| Do 1, B: the fetch refspec | HOLDS | B2 (`+refs/grid/*:refs/grid-remotes/origin/*`, ensured once per clone; narrow per-Job fetch) |
| Do 1, B: cleanup | HOLDS | B7 (a `cleanup` command Job on task close, including a close without landing; prune on other machines) |
| Do 1, B: the secrets check before push | HOLDS | B3 step 2-3 (every commit in `base..HEAD`; a hit gives `secret_found` and no push; `carry` for the rework) |
| Do 1, B: what the land Job does with `refs/grid/<T>/head` | HOLDS | B5 (integrate onto head with a lease) and B6 (one atomic push moves the target and head to the landed sha; with a PR rule head is not moved). UNVERIFIED against P-judge-land-a7, which was not in the hand folder (open item in the doc) |
| Verified CLI fact applied: Jobs run dontAsk plus the exact rule, never auto or bypassPermissions | HOLDS | A1 and A2; decision note item 8 |
| Do 2: decision note updated | HOLDS | decision note items 8 and 9, `source:` line; ledger V-list at `run-jobs-launcher.md:7` |
| Do 2: lane plan with new or changed lanes, xhigh builders, V-numbers, acceptance rows from those rows | HOLDS | lane plan table: V-numbers column on every lane; new lanes 14 P-loop-refs (V27) and 15 P-loop-permissions (V26) at opus xhigh; lanes 1, 4, 12 and 13 changed; V26 and V27 rows added to the owner-acceptance table |
| Decision gate: owned V-numbers listed at the top with where decided | HOLDS | `run-jobs-launcher.md:7-27`, covering V5-V10, V18, V20-V24, V26 and V27 |
| Addenda 4/5 rulings in P-run-jobs-decide.md applied | HOLDS | already in Q4-Q8 from P-run-jobs-decide; the existing text that Addendum 7 overrides was changed (Q1 worktree row, Rejected item, the Q2 `base:` example, the Q7 post types, Q9's "only assignment a launcher writes") |
| Do 3: a commit after each section | HOLDS | 9db60a56 (Addendum 7 section), 3c4c4677 (lane plan) |
| Do 3: last line of the doc exactly DONE | HOLDS | `tail -n 2 docs/research/run-jobs-launcher.md` gives `DONE` |
| Do 3: doc copied to the hand folder as R-P-run-jobs-a7.md | HOLDS | `cp`, exit 0 |
| Do not build | HOLDS | `git log --no-merges --stat 57cd4c04..HEAD`: this lane's commits touch only `docs/research/run-jobs-launcher.md` and this file; the other commits listed came with the origin/main merge (judge-land docs). Against origin/main the bundle also carries the unlanded base lanes P-run-jobs and P-run-jobs-decide |
| No private paths, hosts or emails in committed files | HOLDS | `grep -n -E '/srv/\|/home/\|@gmail\|prod-eu' docs/research/run-jobs-launcher.md`: no match |
| No attribution lines in commits | HOLDS | `git log origin/main..HEAD` messages are one sentence each |
| Merge origin/main before the last proof run | HOLDS | `git fetch origin && git merge origin/main`: merge made (judge-land.md arrived), exit 0 |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: ok, exit 0; `go test -count=1 ./internal/job/lint/`: ok, exit 0 |
| Tests of changed packages and their importers | not run, not asked | no Go package changed (docs only) |
| Lint and the Windows and macOS builds | not run, not asked | no Go file changed |
| New tests fail on base and pass on the lane | not run, not asked | research lane, no tests added |

## What changed
- `docs/research/run-jobs-launcher.md`:
  - a ledger V-list at the top;
  - decision note items 8 and 9;
  - a new section "Addendum 7" (A1-A6 for V26, B1-B9 for V27);
  - text that Addendum 7 overrides, updated in Q1, Q2, Q7 and Q9;
  - the lane plan: a V-numbers column, new lanes 14 and 15, and changes to lanes 1, 4, 12 and 13;
  - V26 and V27 rows in the acceptance table, and new open items.
- `docs/lanes/P-run-jobs-a7.md`: this table.

## NOT VERIFIED on a live screen
Nothing in this lane has a screen.

## Open items
These are listed in the doc:
- the person's own allow rules under dontAsk;
- whether a grant may ever cover a production approval;
- Codex has no exact-rule equivalent;
- the CLI's field name for permission denials;
- the land side is UNVERIFIED against P-judge-land-a7;
- hosts other than GitHub.

DONE
