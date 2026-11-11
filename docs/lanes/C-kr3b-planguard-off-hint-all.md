# C-kr3b-planguard-off-hint-all

Every plan-guard refusal in `evaluatePlan` now ends with the `off` hint.

## Proof table

| Row | Result |
|---|---|
| 1. `go test ./internal/guard/ -count=1` | HOLDS, exit 0 (`ok internal/guard`), includes the new test |
| 2. `go test ./internal/guard/ -run 'TestKr3' -count=1 -v` | HOLDS, exit 0. `TestKr3PlanGuardEveryRefusalEndsWithTheOffHint` runs 4 cases x default/block: reviewer, checker, builder writing the brain, folder name beside tasks (all PASS) |
| 3. Deny count vs Deny lines ending in `off` | HOLDS: `git grep -c 'hookio.Deny' -- internal/guard/plan.go` = 5; `grep -c 'hookio.Deny(.*+ off)$' internal/guard/plan.go` = 5 (plan.go:52, 56, 66, 86, 92). The odd folder name refusal (:52) already ended with `off`. |
| 4. Diff scope | HOLDS: `git diff origin/main --stat -- internal/guard/` = plan.go (2 lines), kr3_planguard_hint_all_test.go (new), planguard_warn_test.go (2 lines). The warn test diff is only the reviewer expected string (:83) and the brain-write one (:93), each gaining `+ off`. |
| 5. `make lint`, `make cross` | HOLDS, both exit 0 (go vet; darwin/arm64 and windows/amd64 vet plus test compile) |
| 6. Walk tests | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. The app-only walks do not apply to this repo. |
| New test fails on base | HOLDS. With plan.go stashed to base: `FAIL .../reviewer_outside_the_projects_root/default: warning "plan-guard: A grid-reviewer writes only its Verdict inside a task folder; <path> is outside it." does not end with " To turn this guard off, run grid hook level plan-guard off (or start Claude Code with GRID_PLAN_GUARD=off)."`. The same failure shows for checker (default and block) and builder writing the brain (default and block, "...instead of <path>." has no hint). The odd folder name case passed on base (it already had the hint). |
| Packages that import guard | HOLDS. `go list` importers: internal/hook, internal/packaging (via hook), internal/cli, cmd/grid, lib. `go test -count=1 ./internal/guard/ ./internal/hook/... ./internal/packaging/... ./internal/cli/... ./cmd/... ./lib/...` all ok (cmd/grid has no test files). |
| Existing tests | Only the two expected strings in planguard_warn_test.go were edited; no test deleted. |
| No guard/gate added | HOLDS: only message text changed. |
| Commits | One commit, one sentence, no attribution lines. |
| *RealPlugin* tests | Not applicable to this brief. |

## What changed

- `internal/guard/plan.go`: the reviewer/checker refusal (:56) and the non-curator brain-write refusal (:66) now end with `+ off`.
- `internal/guard/planguard_warn_test.go`: the two pinned strings (:83, :93) end with `off`.
- `internal/guard/kr3_planguard_hint_all_test.go`: new; asserts the warning (default level) and the deny (block level) end with the literal `krPlanGuardHint` for reviewer, checker, builder brain write and odd folder name.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Not run in a live Claude Code session: the hint text as shown in the terminal.
