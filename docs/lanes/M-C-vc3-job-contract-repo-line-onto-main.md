# M-C-vc3-job-contract-repo-line-onto-main

Lane C-vc3-job-contract-repo-line merged onto plugin main f09dda3a.

## Proof table

| Row | Result | Evidence |
| --- | --- | --- |
| 1. origin/main is an ancestor of HEAD | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0 |
| 1. diff names only lane files | HOLDS | `git diff origin/main --stat`: 21 files, all from C-vc, C-vc2 and C-vc3 (agents/orchestrator.md, contract/schemas/job-record.json, docs/job-contract.md, docs/lanes/C-vc*, internal/cli, internal/contract, internal/job, internal/job/jobcmd, internal/job/lint, internal/orchestrator, skills/plan-jobs/SKILL.md) |
| 2. C-vc3 row: `git grep -n '"repo"' -- contract/schemas/job-record.json docs/job-contract.md` | HOLDS | exit 0: `contract/schemas/job-record.json:24:    "repo": {` and `docs/job-contract.md:29:A code Job names its repo in the Job record as `"repo": "<path>"`...` |
| 2. C-vc3 row: `go test -count=1 -run TestVcRepo ./internal/job/ ./internal/job/lint/ ./internal/orchestrator/ ./internal/cli/` | HOLDS | exit 0: ok job 0.533s, lint 0.230s, orchestrator 2.394s, cli 2.603s |
| 2. C-vc3 row: `grep -c 'NOT DONE' docs/lanes/C-vc-job-repo-key-plugin.md` | HOLDS | prints 0 |
| 3. Walk test `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0: ok internal/cli 0.093s |
| 3. Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0: ok 0.181s |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`, no output) |
| 4. `make cross` | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package ok |
| Tests of changed packages and importers | HOLDS | `go test -count=1 ./...` over the whole module: exit 0, no FAIL line (changed: contract, internal/contract, internal/job, jobcmd, lint, orchestrator, cli; importers include jobgraph, lease, hook, migrate, mcp, ports, cmd/grid, lib, all covered) |
| New tests fail on base, pass on lane | HOLDS (inherited) | This lane adds no test; the vc tests come from C-vc and are unchanged: vc_repo_key_test.go, vc_repo_lint_test.go, vc_spawn_repo_test.go, vc_worker_repo_test.go |
| No test deleted or edited | HOLDS | merge touched no `_test.go` file |
| No guard, gate, cap, hold or blocking hook | HOLDS | none added |
| Small commits, one-sentence messages, no attribution | HOLDS | one merge commit, "Merge origin/main into the job-contract repo-line lane." |
| Real-plugin tests | Not applicable | no row names a *RealPlugin* test |
| App-only walk tests (webui, grid-server, slices) | Not applicable | this is the plugin repo |

## What changed

Merge of origin/main (f09dda3a) into the lane. Two conflicts, both resolved by keeping both sides:

- `agents/orchestrator.md` steps 4 and 5: step 4 keeps the lane's sentence "In a project with more than one repo, a code Job names its `repo`."; step 5 takes main's `grid job post --task <task> --type job --body <json>`.
- `internal/cli/worker.go` imports: both `internal/job/jobcmd` (lane) and `internal/mdfile` (main) kept.

`go build ./...` passes. Nothing else was edited.

## NOT VERIFIED on a live screen

Nothing here is UI. No live screen is involved.

Verdict: DONE
