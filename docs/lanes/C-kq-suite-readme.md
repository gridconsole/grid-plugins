# C-kq-suite-readme

New page: evals/suite/README.md. It says how to run one suite case with `claude plugin eval`, what a run costs, where results go, and how a case is checked without a run.

| Row | Result |
|---|---|
| 1 one-case command | HOLDS. `grep -n -e '--eval-dir evals/suite' -e '--case' -e '--runs 1' -e '--no-publish' evals/suite/README.md` prints 7 lines (7, 10, 15, 16, 17, 21, 31), exit 0 |
| 2 every long flag is in the help text | NOT DONE. The help capture (plugin-eval-help-2.1.319.txt) is refused to Bash cat, Bash grep and Read in this sandbox, so the node check could not be run. The flags on the page (`--eval-dir --case --runs --scaffold --allow-tools --trust-plugin --no-publish --ablation --json --output-dir`) come from evals/builtin-fit.md (f), (g), which cites the docs page. builtin-fit.md (g) marks `--output-dir` UNVERIFIED for 2.1.319. The orchestrator should run the node command from the brief |
| 3 costs money, ablation, GradeCase, builtin-fit.md | HOLDS. "A run costs money" (line 29), `--ablation` lines 24 and 31; `grep -c -e GradeCase -e 'builtin-fit.md'` prints 2 |
| 4 no other case named | HOLDS. `git ls-tree --name-only HEAD evals/suite/` lists bash-one-command, builder-not-done, job-result-posted, plan-step-anchor, read-before-edit, replan-dead-job, research-unverified, review-posts-found (before the page). All are in the brief's pattern except research-unverified; the grep prints nothing, exit 1 |
| 5 diff, lint, cross | HOLDS. `git diff main --stat` names evals/builtin-fit.md and evals/suite/README.md (plus this report file). `make lint` exit 0. `make cross` exit 0 |
| Packages changed / importers | Docs only; no Go package changed, so none to list. `go test -count=1 ./internal/evalcmd/ -run 'TestHj'` ok |
| Plugin walk rows | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| New tests | None added (docs only), so no fail-on-base proof. |
| Eval dir rule (d) | builtin-fit.md (d): a loose file in the eval dir is UNVERIFIED but a case needs `prompt.md` or `case.yaml`, so the page is README.md, not SUITE.md. If a live run reads it as a case, rename to evals/SUITE.md |
| RealPlugin, UI rows | None in this brief. |

## Differences from the brief

- The brief asks the page to name `go test ./evals/suitecheck/...`. That folder does not exist on this base, and `go test` would fail, so the page names only `go test ./internal/evalcmd/ -run 'TestHj'`. Add the suitecheck line when that folder lands.
- The ablation default is cited from the docs via builtin-fit.md (f), not from help text lines 16-22, which could not be read.

## NOT VERIFIED on a live screen

No live run was made (rule: no claude, no eval). The command line is not run once.
