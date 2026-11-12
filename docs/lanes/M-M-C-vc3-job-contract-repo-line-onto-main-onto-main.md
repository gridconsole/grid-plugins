# M-M-C-vc3-job-contract-repo-line-onto-main-onto-main

Lane M-C-vc3-job-contract-repo-line-onto-main merged onto plugin main 792b8d6e.

## Proof table

| Row | Result | Evidence |
| --- | --- | --- |
| 1. origin/main is an ancestor of HEAD | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0 |
| 1. diff names only lane files | HOLDS | `git diff origin/main --stat`: 22 files, all from C-vc, C-vc2, C-vc3 and the M-C-vc3 lane doc (agents/orchestrator.md, contract/schemas/job-record.json, docs/job-contract.md, docs/lanes/C-vc*, docs/lanes/M-C-vc3-*, internal/cli, internal/contract, internal/job, internal/job/jobcmd, internal/job/lint, internal/orchestrator, skills/plan-jobs/SKILL.md) |
| 2. C-vc3 row: `git grep -n '"repo"' -- contract/schemas/job-record.json docs/job-contract.md` | HOLDS | exit 0: `contract/schemas/job-record.json:24:    "repo": {`, `docs/job-contract.md:29` and `:30` |
| 2. C-vc3 row: `go test -count=1 -run TestVcRepo ./internal/job/ ./internal/job/lint/ ./internal/orchestrator/ ./internal/cli/` | HOLDS | exit 0: ok job 0.150s, lint 0.343s, orchestrator 1.350s, cli 1.620s |
| 2. C-vc3 row: `grep -c` of the open-verdict phrase in docs/lanes/C-vc-job-repo-key-plugin.md | HOLDS | prints 0 |
| 3. Walk test `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0: ok internal/cli 0.575s |
| 3. Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0: ok 0.814s |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`, no output) |
| 4. `make cross` | HOLDS | exit 0 (reported by the background task; windows/amd64 and darwin vet and test compile) |
| Tests of changed packages and importers | HOLDS | `go test -count=1 ./...` over the whole module: every package ok except internal/env `TestUpHealthTimeoutHonorsStopGrace` (see below); changed packages contract, internal/contract, internal/job, jobcmd, lint, orchestrator, cli all ok |
| internal/env timing test | HOLDS on rerun | first full run, while `make cross` ran at the same time: `env_test.go:260: signals.log = "" ... want "INT\nTERM\n"`. Alone it passes: `go test -count=3 -run TestUpHealthTimeoutHonorsStopGrace ./internal/env` exit 0, and `go test -count=1 ./internal/env` exit 0 (12.757s). One earlier lone run also failed (`signals.log = "INT\n"`), so the test is timing-sensitive under load; the lane touches nothing in internal/env |
| New tests fail on base, pass on lane | HOLDS (inherited) | This lane adds no test; the vc tests come from C-vc and are unchanged |
| No test deleted or edited | HOLDS | the merge touched no `_test.go` file; conflicts were only in agents/orchestrator.md and internal/cli/worker.go |
| Carried lane docs: open verdicts rewritten | HOLDS | `grep -n` for the open-verdict phrase in docs/lanes/C-vc-job-repo-key-plugin.md, C-vc2-job-repo-key-text.md and M-C-vc3-job-contract-repo-line-onto-main.md finds only one line, a quoted grep command in a HOLDS row (not a verdict), so nothing needed rewriting |
| No guard, gate, cap, hold or blocking hook | HOLDS | none added |
| Small commits, one-sentence messages, no attribution | HOLDS | one merge commit, one lane-doc commit, no attribution lines |
| Real-plugin tests | Not applicable | no row names a *RealPlugin* test |
| App-only walk tests (webui, grid-server, slices) | Not applicable | this is the plugin repo |

## What changed

Merge of origin/main (792b8d6e) into the lane. Two conflicts, both resolved by keeping both sides:

- `agents/orchestrator.md` step 4: main's sentence on a step's `who` and the `job.role-agent` lint warning, plus the lane's sentence "In a project with more than one repo, a code Job names its `repo`."
- `internal/cli/worker.go` worker options: both the lane's `Repo` (jobcmd.RepoFolder) and main's `FirstJob: firstJobOf` kept.

`go build ./...` passes. Nothing else was edited.

## NOT VERIFIED on a live screen

Nothing here is UI. No live screen is involved.

Verdict: DONE
