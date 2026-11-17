# Lane R-dogfood-gaps-2

The research lane on what grid-plugins needs to run this build itself, across the local accounts of one machine. The research doc is `docs/research/dogfood-gaps.md`. The proof was taken after merging origin/main (cba64cdf), at lane head 3edbb141 plus this doc.

## Proof

| Row | Status | Proof |
|---|---|---|
| Q1 gap table per pipeline piece, plugin vs scripts with file:line | HOLDS | `docs/research/dogfood-gaps.md:17-36` (G1-G15 plus one grid-worker row, G16). Section 1 is at `:38`. |
| Q2 smoke run with the plugin alone, and its stop point | HOLDS, with a deviation | `docs/research/dogfood-gaps.md` section 2. Stop point: Jobs `assigned` with why "starting" and no agent started. The run was as grid2 under the lane folder, not as grid4: there is no sudo and `mkdir` on that path was refused. The finding does not depend on the login. |
| Q3 cutover order, first lane starts Jobs across local accounts, no grid-worker fallback | HOLDS | Section 3: first lane P-run-jobs. Option A (one runner per login on a shared home) or B is marked as an owner decision. |
| Q4 lane plan: repo, model/effort, files, acceptance rows, dependencies, landing first | HOLDS | Section 4 table, rows 1-11. Landing first per R-land-job-type, with L5 re-scoped to the plugin. |
| Q5 brain drafts for decisions, conventions, machines, agents, routines with reasons, including the owner's correction | HOLDS | Section 5. The first note is decisions / plugin-one-machine-many-accounts. |
| Owner correction: Judge and the missing agent start are confirmed as plugin gaps | HOLDS | G1 cites `agents/orchestrator.md:46`. G3 cites `tick.go:330` and the app's `judge.go:243`. |
| Last line is DONE or NOT DONE | HOLDS | The last line of `docs/research/dogfood-gaps.md` is `DONE`. |
| No personal paths, hosts or emails in committed files | HOLDS | `grep -nE '/srv|/home|/Users|@[a-z0-9.-]+\.(nl|com|org)' docs/research/dogfood-gaps.md`: no match, exit 1. |
| No guard, gate, cap, hold or blocking hook added | HOLDS | Docs only. The cap question G15 is left to the owner. |
| No attribution lines, small commits with one-sentence messages | HOLDS | `git log origin/main..HEAD` |
| Smoke files not committed | HOLDS | `.smoke/` is untracked (`git status`). |
| Merge origin/main | HOLDS | `git merge --no-edit origin/main`, exit 0. |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'`: ok, exit 0. `go test -count=1 ./internal/job/lint/`: ok, exit 0. |
| make test, full go test | not run, not asked | — |

## Open items

- Owner decisions:
  - option A or B for running a Job as another login;
  - whether any cap on running agents is wanted;
  - which Grid home is the shared one, and its group.
- R-land-job-type L5 must be re-scoped from the app to the plugin.
- G12: it is UNVERIFIED whether `grid feed read --wait` crosses tasks.
- Script lines marked "(R-dogfood-gaps, not re-read)" were not checked again.
