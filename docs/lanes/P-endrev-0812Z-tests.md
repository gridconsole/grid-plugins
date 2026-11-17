# P-endrev-0812Z-tests: tighten TestOrphanNoExit, make the brain walk test search the exempt files

Source: end review 2026-11-17T0812Z, S3 and S4. Only two test files change, plus this doc.

## Proof table

| Row | Result |
|---|---|
| 1. TestOrphanNoExit asserts nil or never_judged; comment matches | HOLDS. internal/orchestrator/tree_orphan_test.go:172 now fails on `r.Orphan != nil && r.Orphan.Kind != OrphanNeverJudged`; comment at :148-151 says the same. `go test -count=1 ./internal/orchestrator/ -run TestOrphanNoExit -v` exit 0, `--- PASS: TestOrphanNoExit`. |
| 1. fails when the Job is given never_acked (temporary edit, reverted) | HOLDS. Temp edit: replaced `f.exit(t, "done")` with a fresh Job that is never acked. Run exit 1: `tree_orphan_test.go:177: after a worker-exit the Orphan is nil or never_judged (the fold makes it submitted), got &{Kind:never_acked Since:2026-11-15T16:23:39.742Z Detail:attempt 1 assigned to worker@... and not acked before its lease ran out}` / `--- FAIL: TestOrphanNoExit`. Reverted with `git checkout` (then re-applied the real edit before the commit; `git status --short` clean after). |
| 2. Walk test comment and code agree on exempt files | HOLDS. internal/braincmd/pb_rd_walk_test.go:11-17 comment, :22 `search bool`, :146 skip is now `exempt != "" && marker == "" && !search`. Every exempt Go file that exists has `search: true` (brain.go :56, hook.go :58, markdown.go :59, plan.go :60, cb_author_origin.go :33) and is searched for knowledge/ paths in code. `internal/jobcmd/command.go` (:57) stays unsearched because the file is gone from the tree (2.2 doc line stale), said in its reason. `internal/migrate/plan.go` may name knowledge only at its two kind lists (:228, :252, `[]string{"knowledge", "sessions"}`): the migration reads the old layout's folder on purpose to move it; allowed through `old`. internal/hook/brain.go and hook.go and mdfile/markdown.go have no knowledge/ path and are allowed none. `go test -count=1 ./internal/braincmd/ -run TestPbRdReaderTableWalk -v` exit 0, PASS. |
| 2. fail-on-revert run | HOLDS. Temp edit: `var pbRdTmpProbe = "knowledge/probe"` added to internal/hook/brain.go (exempt, now searched) and internal/job/source.go (non-exempt). Exit 1: `pb_rd_walk_test.go:173: internal/hook/brain.go still builds a knowledge/ brain path: var pbRdTmpProbe = "knowledge/probe"` and `pb_rd_walk_test.go:173: internal/job/source.go still builds a knowledge/ brain path: ...` / `--- FAIL: TestPbRdReaderTableWalk`. Reverted with `git checkout -- internal/hook/brain.go internal/job/source.go`; `git status --short` printed nothing. |
| 3. orchestrator and braincmd packages pass | HOLDS. `go test -count=1 ./internal/orchestrator/... ./internal/braincmd/... ./internal/mcp/... ./internal/migrate/... ./internal/runner/... ./internal/cli/...` exit 0: orchestrator ok 100.504s, braincmd ok 1.548s, mcp ok, migrate ok, runner no test files, cli ok 40.228s. |
| Importers of the changed packages | `go list` import scan: orchestrator is imported by internal/mcp, internal/migrate, internal/runner, internal/cli (and through cli cmd/grid, lib); braincmd by internal/cli. All run above. |
| 4. Only two test files change | HOLDS. `git diff --stat origin/main..HEAD`: internal/braincmd/pb_rd_walk_test.go, internal/orchestrator/tree_orphan_test.go, plus this doc. |
| No attribution lines | HOLDS. `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` prints 0. |
| New tests fail on base | HOLDS for the strengthened assertions: the two failing runs above are the proof (base accepts never_acked at :172; base walk skips brain.go). No existing test edited to pass: both edits tighten the assertion. |
| Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0. App-only walks (webui, grid-server, slices): not run, not asked (plugin repo). |
| Merge origin/main before last run | HOLDS. `git fetch origin && git merge origin/main`: Already up to date. |
| make lint | HOLDS. `make lint` exit 0 (`go vet ./...`). |
| Windows and macOS builds | HOLDS. `GOOS=windows GOARCH=amd64 go build ./...` exit 0; `GOOS=darwin GOARCH=arm64 go build ./...` exit 0. |
| RealPlugin tests | not run, not asked |

## What changed

- tree_orphan_test.go: TestOrphanNoExit's last check now asserts "Orphan is nil or never_judged", so never_acked and never_picked_up fail it too; the comment above it says what the code does.
- pb_rd_walk_test.go: new `search` field; exempt Go files that exist are searched for knowledge/ paths in code; plan.go lists its two intended lines; the stale jobcmd reason says why it cannot be searched.

## NOT VERIFIED on a live screen

Nothing here is UI.
