# M-P-projectenv-deploy-kind-onto-main

Carries lane P-projectenv-deploy-kind (kind `deploy` for a project environment) onto plugin main. Landing log pint214.log skipped it because `internal/cli/ln_project_env.go` conflicted.

## The conflict and how it was resolved

One conflict, in `internal/cli/ln_project_env.go`, the `projectEnvUsage` constant. The lane changed `--kind local|remote|cloud` to `--kind local|remote|cloud|deploy`. Main added `[--step WF:STEP]...` to the same line. Resolved by keeping both: `--kind local|remote|cloud|deploy` and `--step WF:STEP`. Nothing else conflicted. No test, row or error code of either side was dropped. The lane's `--kind` flag help text change merged on its own.

`git diff origin/main --stat` after the merge: `contract/schemas/cmd-project-env.json`, `docs/lanes/P-projectenv-deploy-kind.md`, `internal/cli/ln_project_env.go`, `internal/cli/pd_project_env_deploy_test.go`, `internal/projectenv/pd_deploy_kind_test.go`, `internal/projectenv/projectenv.go`. Those are the files the lane touched, plus this doc.

## Proof table

| Row | Verdict |
|---|---|
| 1a. `git merge-base --is-ancestor origin/main HEAD` | HOLDS, exit 0 |
| 1b. `git diff origin/main --stat` names only lane files | HOLDS, the six files above |
| 2a. P-projectenv-deploy-kind row 1: an environment of kind deploy is written and read back | HOLDS, `go test -race -count=1 -v -run "Deploy\|Pd" ./internal/projectenv/ ./internal/cli/` exit 0: TestPdDeployKindRenderParse, TestPdDeployKindSaveList, TestPdDeployKindBare, TestPdProjectEnvDeployKind PASS |
| 2b. P-projectenv-deploy-kind row 2: old kinds unchanged | HOLDS, TestPdDeployOldKindsUnchanged PASS (same run), plus the older projectenv and cli env tests in the full run below |
| 2c. P-projectenv-deploy-kind row 3: tests pass, `go test -race` | HOLDS, `go test -race -count=1 ./internal/projectenv/ ./internal/cli/ ./cmd/grid/ ./lib/` exit 0 (projectenv ok, cli ok 242.98s, cmd/grid no test files, lib ok) |
| 2d. P-projectenv-deploy-kind row 4: `make lint` | HOLDS, exit 0 (`go vet ./...`) |
| 2e. CLI walk test, usage names deploy | HOLDS, TestPdProjectEnvUsageNamesDeploy PASS; `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 |
| 3a. Walk test `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0, ok 0.173s |
| 3b. Walk test `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0, ok 0.352s |
| 4a. `make lint` | HOLDS, exit 0 |
| 4b. `make cross` | HOLDS, exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package ok |
| Importers of changed packages | `internal/projectenv` is imported by `internal/cli`; `internal/cli` is imported by `cmd/grid` and `lib`. All four tested above. |
| Carried lane NOT DONE rewrite (end review 0415Z M4) | HOLDS, nothing to rewrite: `docs/lanes/P-projectenv-deploy-kind.md` has no NOT DONE verdict, row or last line (its last line is DONE). Its only match for "NOT DONE" is line 48, a row title quoting the allowed last-line values "DONE or NOT DONE", left unchanged |
| `make test` whole module, cross-repo set | not run, not asked (the orchestrator runs them at landing) |
| `make realplugin` guest walk tables | not run, not asked (that rule is for the app repo; this is the plugin repo) |

## What changed

- Merge commit of origin/main into the lane. Conflict resolved in `internal/cli/ln_project_env.go` as above.
- This doc.

## NOT VERIFIED on a live screen

No UI changed. The app drawing a deploy target card from the record needs the app-side kinds map (carried from the lane's open items) and is not verified here.

DONE
