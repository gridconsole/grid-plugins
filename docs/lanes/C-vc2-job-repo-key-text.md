# C-vc2-job-repo-key-text: the "a code Job names its repo" sentence

Base: 85bfcaa5 (lane-C-vc-job-repo-key-plugin).

| Row | State | Proof |
|---|---|---|
| Sentence in skills/plan-jobs/SKILL.md | HOLDS | skills/plan-jobs/SKILL.md:44 (`repo` key, beside the other Job keys) |
| Sentence in agents/orchestrator.md | HOLDS | agents/orchestrator.md:25 ("In a project with more than one repo, a code Job names its `repo`.") |
| All rows of C-vc-job-repo-key-plugin | HOLDS | rows 1-3: `go test -count=1 -v -run TestVcRepo ./internal/job/ ./internal/job/lint/ ./internal/orchestrator/ ./internal/cli/` exit 0, 13 PASS lines; row 4: `-run TestHfJr ./internal/job/` ok; row 5: `./internal/job/... ./internal/contract/ ./internal/orchestrator/` exit 0 and `-run TestWorkerNext ./internal/cli` ok; row 6: `make lint` and `make cross` exit 0. Detail in docs/lanes/C-vc-job-repo-key-plugin.md |
| Importers (go list) | HOLDS | all importers of the changed packages ok, listed in the C-vc lane doc |
| Walk tests | HOLDS | `./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `./internal/job/lint/` ok |
| No existing test edited, no guard | HOLDS | text and doc files only |

What changed: two sentences and the C-vc lane doc proof table. The skill golden/text tests (full `./internal/cli`) pass.

Open item from C-vc: `repo: "."` is refused by lint when the project lists named repos.

NOT VERIFIED on a live screen: nothing (no UI).
