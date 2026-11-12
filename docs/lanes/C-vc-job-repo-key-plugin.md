# C-vc-job-repo-key-plugin: a Job may name its repo; the spawn line carries its folder

Base: 2b47f62181522cb3036a3e38e186ec6d9e779eb8 (main = origin/main at lane start).

## Base check (main)

`go test -count=1 -v -run TestVcRepo ./internal/job/ ./internal/job/lint/ ./internal/orchestrator/ ./internal/cli/` on main with only the new tests added:

```
vc_repo_key_test.go:51: DecodePlanned problems: [unknown planned field "repo" json: unknown field "repo"]
--- FAIL: TestVcRepoPlannedJobNamesItsRepo
--- FAIL: TestVcRepoBriefMetadataKeepsTheRepo
vc_repo_lint_test.go:53: want one job.repo finding, got [{Rule:job.record ... Message:$: unknown property repo ...}]
--- FAIL: TestVcRepoLintUnknownRepo / TestVcRepoLintKnownRepo / TestVcRepoLintNoRepoRecord
internal/orchestrator/vc_spawn_repo_test.go:48:4: s.Repo undefined (type *Stream has no field or method Repo)
FAIL internal/orchestrator [build failed]
vc_worker_repo_test.go:72: $.body: oneOf matched 0 branches: $.body: missing required ref
--- FAIL: TestVcRepoWorkerNextSpawnCarriesTheRepoFolder
```

Main refuses `repo` (planned.go:31 and the job-record schema); it does not accept or print it.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1 decode + lint | HOLDS | `go test -count=1 -v -run TestVcRepo ./internal/job/ ./internal/job/lint/` exit 0: PASS TestVcRepoPlannedJobNamesItsRepo, TestVcRepoPlannedJobWithoutRepo, TestVcRepoBriefMetadataKeepsTheRepo, TestVcRepoLintUnknownRepo (one `job.repo` error naming nope, app, plugin), TestVcRepoLintKnownRepo, TestVcRepoLintNoRepoRecord |
| 2 spawn line | HOLDS | `go test -count=1 -v -run TestVcRepo ./internal/orchestrator/` exit 0: PASS TestVcRepoSpawnLineCarriesTheRepoFolder (`"repo":"/abs/plugin"`, func got `plugin`; Job naming none asks with ""), TestVcRepoNilRepoFuncPrintsNoRepoKey, TestVcRepoFuncErrorIsOneDiagLine |
| 3 grid worker next | HOLDS | `go test -count=1 -v -run TestVcRepo ./internal/cli/` exit 0: PASS TestVcRepoWorkerNextSpawnCarriesTheRepoFolder (plugin folder for a Job naming plugin, app folder for a Job naming none, the project folder for a one-repo project with repo `.`; the two-repo folder is a temp dir, not a git repo) |
| 4 contract doc + skill text | HOLDS (finished by C-vc2-job-repo-key-text) | `go test -count=1 -run TestHfJr ./internal/job/` exit 0; `repo` is in contract/schemas/job-record.json and docs/job-contract.md; the sentence is at skills/plan-jobs/SKILL.md:44 and agents/orchestrator.md:25 ("a code Job names its `repo`") |
| 5 package tests | HOLDS | `go test -count=1 ./internal/job/... ./internal/contract/ ./internal/orchestrator/` exit 0; `go test -count=1 -run 'TestWorkerNext\|TestHsIpWalksEveryCommand\|TestIlGuestSiteList' ./internal/cli` exit 0 |
| 6 lint, cross, diff | HOLDS | re-run on the C-vc2 head: `make lint` exit 0 (go vet), `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile); diff names the Touches plus this file |
| New tests fail on base | HOLDS | base output above |
| No existing test edited | HOLDS | only new files vc_repo_key_test.go, vc_repo_lint_test.go, vc_spawn_repo_test.go, vc_worker_repo_test.go |
| No guard, gate, cap or hold | HOLDS | the Repo func error is one diag line and the spawn line still prints (stream.go Repo branch) |
| Walk tests (plugin) | HOLDS | `./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `./internal/job/lint/` exit 0 (in `./internal/job/...`) |
| Importers of changed packages (go list) | HOLDS | `go list` importers of internal/job, contract, orchestrator, jobcmd: artifact brain braincmd cli feed guard hook intent jobgraph land lease mcp migrate ports question session settings signal task updates usage where workflow (+ job/lint). `go test -count=1` on all exit 0, including full `./internal/cli` (ok 78.9s) |
| Runbook-reviewer check | HOLDS | read in C-vc2; walk tests `./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok, `./internal/job/lint/` ok |
| Small commits | NOT DONE | the C-vc commit is one; C-vc2 adds a second |
| Budget | overrun | about 62 tool calls of 60; no work-in-progress bundle at call 35 |

## What changed

- internal/job/job.go: `Repo string json:"repo,omitempty"`; planned.go allows `repo`; source.go renders it among the optional brief fields.
- internal/job/lint/lint.go: `jobRepo`: a Job's `repo` not in Options.Repos (when not empty) is a `job.repo` error naming the repos.
- contract/schemas/job-record.json and docs/job-contract.md: the `repo` property and row.
- internal/contract: Job.Repo, folded from the job post body.
- internal/orchestrator: Spawn.Repo filled in tick.go; `Stream.Repo func(dir, repo string) (string, error)`; spawn line `repo`, omitted when empty; an error is one diag line.
- internal/job/jobcmd/command.go: `RepoFolder(task, repo, e)`: named repo folder from the project record, `.` the project folder, "" the first repo, no record "", an unlisted name an error.
- internal/cli/worker.go: Stream.Repo wired to jobcmd.RepoFolder.

## Open items

- `repo: "."` passes RepoFolder but lint refuses it when the project lists named repos, because roots never puts `.` in f.Repos. The brief does not cover this case.

NOT VERIFIED on a live screen: nothing (no UI).
