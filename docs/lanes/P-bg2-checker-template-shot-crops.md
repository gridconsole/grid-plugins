# P-bg2-checker-template-shot-crops

Base sha 656e6bce. Base check: `agent-templates/grid-checker.md:15` says only "the screenshot the browser tool returned" and has no `grid shot crop`; `agents/checker.md:19` holds the "Numbers before pictures" rule.

## Proof table

| Row | Result |
|---|---|
| 1. Red on base | HOLDS. `go test -count=1 ./internal/contract/ -run TestBg2CheckerTemplateReadsShotsByNumbers` exit 1 on base: `agent-templates/grid-checker.md lacks "grid shot diff"`, `"grid shot crop"`, `"taller than one viewport"`, `"3840-wide"`. On the lane: `--- PASS: TestBg2CheckerTemplateReadsShotsByNumbers (0.00s)`, exit 0. |
| 2. Template pins | HOLDS. `go test -count=1 ./internal/contract/ ./internal/job/ ./internal/cli/ ./internal/settings/ .` exit 0, with no existing test edited. |
| 3. Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0. `go test -count=1 ./internal/job/lint/` exit 0. |
| 4. `make lint` | HOLDS, exit 0 (`go vet ./...`). |
| 4. `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64, vet and test compile). |
| 4. Diff touches | The template, the new test file and this doc only. |
| Merge origin/main | HOLDS. `git fetch origin && git merge origin/main` reported "Already up to date". |
| Packages importing the changed ones | No Go package imports the test file or the template. The brief's package list is the set run. |
| Guest walk tables, `make realplugin` | Not run, not asked: this is the plugin repo. |
| Guards, gates, caps | None added. |

## What changed

- `agent-templates/grid-checker.md`: one new paragraph line after line 15 that tells the subagent to read a screenshot by numbers first. It names `grid shot diff` and `grid shot crop --box`, and it forbids opening a full-page shot taller than one viewport or a 3840-wide shot whole. It has no pipe and no redirect.
- `internal/contract/bg2_checker_template_shots_test.go`: new test `TestBg2CheckerTemplateReadsShotsByNumbers`.

## NOT VERIFIED on a live screen

Whether the synced `grid-checker` subagent follows the sentence in a real run. This lane ran no browser or claude.
