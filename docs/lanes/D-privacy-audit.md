# D-privacy-audit: public audit and the publish-set deny list

Base: main eb833a2d. Branch lane-D-privacy-audit. Two commits.

## Proof table

| Row | Status | Evidence |
|---|---|---|
| 1. docs/public-audit.md: all seven folders checked, one row per hit with file:line and fix, greps pasted | HOLDS | docs/public-audit.md:9-19 (patterns), :21-34 (hits). Greps over evals, corpus, recordings, fixtures, golden, docs, agent-templates for home paths, emails, tokens, names, IPs, client names. No secret was printed. |
| 2. Fix test-data hits; list the rest NOT DONE | HOLDS | Test data (evals, corpus, recordings, fixtures, golden, agent-templates) had no personal hit, so no placeholder swap was needed. Fixed in docs/lanes: a first name (L160.md:6,23), a macOS temp hash path (L135R.md, 8 lines), a VM brief path (S6b.md:17, PLUG-shell.md:26). Re-run of the name, temp and host path greps over the seven folders prints nothing. Remaining items are in docs/public-audit.md "NOT DONE". |
| 3. dist/plugin has no evals, corpus, recordings; test checks the tree against a deny list | HOLDS | internal/packaging/dist.go:`stage`, `shippedDirs`; internal/packaging/publish_set_test.go `TestPluginTreeHasNoInternalFolders` (stages the real repo root, walks the tree, deny list evals, corpus, recordings, fixtures, golden, testdata, lanes). `go test ./internal/packaging/... -count=1` exit 0. |
| 3b. The new test fails when the deny list is broken | HOLDS | With `evals` added to `shippedDirs` (reverted): `--- FAIL: TestPluginTreeHasNoInternalFolders: public plugin tree holds internal folders: evals` and `--- FAIL: TestShippedDirsAvoidTheDenyList: shippedDirs lists denied folder evals`. On base the file does not compile (`stage` is new), so it cannot pass there. |
| 4. go test ./... and make lint: 0 fail | HOLDS | `go test ./... -count=1`: no line other than ok or no test files, exit 0. `make lint`: exit 0 (go vet and gofmt -l clean). |
| Packages changed and their importers | HOLDS | `go list` on importers: internal/packaging and internal/packaging/cmd. `go test ./internal/packaging/... -count=1` exit 0 (cmd has no test files). |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| No existing test edited or deleted | HOLDS | `git diff origin/main..HEAD --stat`: no change to an existing `_test.go`; one new test file. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change is a build-time copy list and a test; nothing runs on an agent. |
| No attribution lines, no personal paths, hosts or emails in commits or files | HOLDS | Commit messages are one sentence with no trailers. The seven-folder greps for names, temp paths and VM paths print nothing after the fixes, including this file. |
| Nothing pushed | HOLDS | Bundle only. |

## What changed

- `internal/packaging/dist.go`: `Build` now calls a new `stage` function (Write, copy of `shippedDirs`, LICENSE). Same output as before; `shippedDirs` names the only source folders that ship: skills, agents, agent-templates.
- `internal/packaging/publish_set_test.go` (new): the deny list test on the real repo root, a test on a synthetic root that holds every denied folder, a test that the walker finds a planted folder, and a test that `shippedDirs` avoids the list.
- `docs/public-audit.md` (new): patterns, hit table, NOT DONE.
- `docs/lanes/L160.md`, `L135R.md`, `S6b.md`, `PLUG-shell.md`: a name, a temp path and two VM paths replaced.

## NOT DONE

- Physically moving evals, corpus, recordings, fixtures and golden out of the source repo. Tests in internal/eval, internal/evalcmd and internal/golden read them by relative path and `make eval-check` and `make golden` depend on them, so a move touches packages outside this brief and is a repo-split decision for the owner. The publish set is closed instead (row 3). Reason also in docs/public-audit.md.
- Whether docs/lanes should stay in the repo that is published: the owner's call. They are not in dist/plugin.

## NOT VERIFIED on a live screen

Nothing here has a UI. `make dist` (a full release build with five cross-compiles) was not run; the tree check runs `stage`, the part of `Build` that decides what is copied, on the real repo root. I did not install or load the built plugin in a Claude session.
