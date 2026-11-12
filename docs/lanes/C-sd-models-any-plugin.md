# C-sd-models-any-plugin proof table

Base sha 2b47f62181522cb3036a3e38e186ec6d9e779eb8. On base, catalog.go:27 is `var models = []string{"haiku", "sonnet", "opus"}` (no fable) and validateAgent is at catalog.go:58-72, so the build went ahead.

| Row / rule | Result |
|---|---|
| 1. TestSdAgentSetFable red on base, green here | HOLDS. Base stderr (source files stashed, exit 1): `agent set grid-builder --model fable: 1 agent: unknown model "fable"; allowed: haiku, sonnet, opus`. Lane: `go test -count=1 -v -run TestSd ./internal/cli/` exit 0, `--- PASS: TestSdAgentSetFable`. |
| 2. TestSdModelValues | HOLDS. `--- PASS: TestSdModelValues` (internal/settings/sd_models_any_test.go). Covers accepted, case-folded and stored values, refusals with the exact wording, and the empty model and effort message. |
| 3. `go test -count=1 -v -run TestSd ./internal/settings/ ./internal/cli/` | HOLDS, exit 0, both PASS lines. `go test -count=1 ./internal/settings/` exit 0, so budget_agentsync_test.go:58 still passes. |
| 4. usage strings and docs | HOLDS. `git grep -n 'fable' -- internal/cli/settings.go docs/settings.md` prints settings.go:21, settings.go:76, docs/settings.md:25 and docs/settings.md:185. The old `model   haiku, sonnet, opus\neffort` string is gone. |
| 5. `make lint` / `make cross` / diff | `make lint` (go vet) exit 0. `make cross` (darwin/arm64 and windows/amd64 vet and test compile) exit 0. The diff holds only the Touches files, plus docs/settings.md:185, which states the same list and is in a Touches file, and this proof table, which the runbook requires. |
| Packages that import settings | `go list` importers: account, cli, doctor, guard, hook, updates, cmd/grid, lib. `go test -count=1` on all of them exit 0. updates and cmd/grid have no test files. |
| Walk tests | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok. `go test -count=1 ./internal/job/lint/` ok. |
| gofmt | `gofmt -l internal/` prints nothing. |
| No guard, gate or cap added | HOLDS. This widens an existing check. |
| RealPlugin tests | No row names one; NOT RUN. |

## What changed
- catalog.go: `fable` joins `models`. New `sdNormalizeModel` accepts the four names with an optional `[1m]` in any case and stores them lower-case. It accepts `claude-[a-z0-9.-]+` with an optional `[1m]` and stores it as given. validateAgent uses it and keeps the "unknown model" wording with the new allowed list.
- store.go: SetAgent and SetAgents store the normalized value, replacing the old plain lower-casing, which would have lower-cased claude- ids.
- cli/settings.go (two strings), docs/settings.md:25 and :185, and the two new tests.

## NOT VERIFIED on a live screen
Nothing UI. I did not run Claude Code, so I haven't checked that Claude Code itself accepts these ids.
