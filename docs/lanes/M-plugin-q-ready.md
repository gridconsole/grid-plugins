# M-plugin-q-ready

Merges origin/lane-Q-tree-orphan-jobs and origin/lane-Q-skill-plan-jobs-pointers onto plugin main 78b08bc9.

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1a. Merge lane-Q-tree-orphan-jobs | HOLDS | Conflicted file: `internal/orchestrator/ready.go` (one hunk, tail of the ready function). Clashed with main commit e11cf80a ("The orchestrator loses the round cap and review slots"), which removed the `review_capacity` block the lane's base still had. Resolved: main's removal kept, the lane's `return out, input.ReadySince, nil` kept (the lane's `readyAt` and the `Ready` wrapper merged cleanly). `internal/cli/orchestrator.go` auto-merged. |
| 1b. Merge lane-Q-skill-plan-jobs-pointers | HOLDS | Conflicts, both clashing with main's 10 Nov rules commit (the one that rewrote plan-jobs and the orchestrator steps): `agents/orchestrator.md` — main's step 5 (10 Nov checks) kept, the lane's orphan-tick step added as step 6, "Stop" is now step 7, the lane's pointers paragraph and main's owner-decisions paragraph both kept. `skills/plan-jobs/SKILL.md` — main's description, owner decisions, 7 checks and run list kept; the lane's pointer rules, `after` field rule and example Job added as a "Pointers" section; the lane's duplicate field list and "check before posting" dropped because main's text already covers them. The lane's `internal/orchestrator/ready.go` hunk did not conflict again (resolved by 1a). |
| 2. `git log --oneline main..HEAD` has both merges | HOLDS | 423a6865 (pointers merge) and f261a70f (orphan merge) at the top of the log, followed by the 9 lane commits. |
| 3a. `go test -count=1 ./...` | HOLDS | exit 0; every package `ok`, none FAIL. The changed packages (`internal/orchestrator`, `internal/cli`, `internal/job/lint`) and all importers are in the whole-module run. |
| 3b. `make lint` | HOLDS | exit 0 (output: `go vet ./...`). |
| 3c. `GOOS=windows go vet ./...` | HOLDS | exit 0, no output. |
| 3d. `GOOS=darwin go vet ./...` | HOLDS | exit 0, no output. |
| 4. Test changes | HOLDS (none edited) | `git diff --stat main HEAD -- '*_test.go'` shows only three files added by the lanes: `internal/cli/tree_orphan_test.go`, `internal/job/lint/skill_example_test.go`, `internal/orchestrator/tree_orphan_test.go`. No existing test edited or deleted. |
| Rule: no guard, gate, cap or hold added | HOLDS | The only removal on the conflict was the review-slot cap, which main had already removed by owner direction (e11cf80a). The merge adds nothing of the kind. |
| Rule: no pushing, no personal paths/hosts/emails | HOLDS | Nothing pushed; the resolved files hold none. |
| Rule: new tests fail on base and pass on lane | NOT DONE here | The tests are the Q lanes' own, reviewed and accepted alone; this lane adds no new test and does not re-run them on base. |

## What changed

- `internal/orchestrator/ready.go`: merge keeps `readyAt` (returns ready-since times) and drops the review-slot block main removed.
- `agents/orchestrator.md`, `skills/plan-jobs/SKILL.md`: both sides' text combined as above. One deliberate edit in the lane's example Job: `review: L1` and `max_rounds: 2` removed, since main's contract (`docs/job-contract.md:14-17`) says a Job names no review level or round count. The example still lints clean (`internal/job/lint` test passes).

## NOT VERIFIED on a live screen

Nothing here is UI. Orphan lines in `grid job tree` are proved by the lanes' CLI tests, not on a live screen.

DONE
