# C-r10-curator-job-renders: deliver's curator Job passes its own Job lint

Base: origin/main 301d1185 (the merge before the last proof run said "Already up to date").

## Cause (brief step 1)

- The curator Job is built at `internal/cli/deliver_keeper_job.go:64` (`curatorJobBody`), posted by `postCuratorJob` (`:103`).
- Its read pointers are `brain-proposals/<name>.md` with no range or anchor (`:67`). The lint at `internal/job/lint/lint.go:288` (was `:282`) refuses any unranged read as `job.read-range`, an **error**. That error is why render refused J-5 in r10 (`jobcmd/command.go:163` refuses only on `HasErrors`).
- Its `intent.item` is `task.md#outcome`, the task's free-text Outcome. The lint checks acceptance words against the item text (`lint.go:273-282`, `:333` on the lane head), so every word of the Outcome was "absent from acceptance". This is `job.acceptance-coverage`, a **warning**: it was in the message but did not block render.

## Fix (brief step 2), and why

- **Read range: the lint accepts a whole-file read of one task brain proposal** (`lint.go:40-41`, `:288`). The pattern is `^(task:)?brain-proposals/<name>.md$`, with one segment and no `repo:`. I did not add a range to the pointer, for two reasons:
  - A proposal is one note the curator decides on whole, so a range adds nothing.
  - A range taken at post time falls outside the file once the proposal is edited before render. `source.go:402` ("range outside file") would then refuse render again.
  - Also, `deliver_keeper_job_test.go:93` pins the read pointers unranged.
  - The resolver call still runs (`lint.go:291`), so a missing proposal is still a `job.read-range` error.
- **Coverage: a curator Job's plan is its title** (`lint.go:313-318`). Its item is the delivered Outcome, which the keeper links notes to, not a plan step it builds. This follows the orchestrator precedent just above it. The rule stays live: the one acceptance row now names the title's words ("Curate the N brain proposal(s) of T-xx: ...", `deliver_keeper_job.go:86`). The two substrings that `deliver_keeper_job_test.go:96` pins are kept.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| New test fails on main, passes on lane | HOLDS | `TestR10DeliverCuratorJobPassesRenderLint` (`internal/cli/r10_curator_job_renders_test.go`): deliver on a task with one waiting proposal, then the posted body linted with render's options, then `grid job render` writes `brief.md`. It failed on base, exit 1 (first run, before the fix): `curator Job finding job.acceptance-coverage: Plan words absent from acceptance: amounts, cents, check, down, file, handling, i-1, money, outcome, proof, result, rounding, rounds, rule, tally, whole, written` and `curator Job finding job.read-range: Read pointer has no range: brain-proposals/tally-money-handling.md` and `render would refuse the curator Job`. On the lane: `--- PASS: TestR10DeliverCuratorJobPassesRenderLint`, exit 0. |
| Lint unit test fails on base, passes on lane | HOLDS | `TestR10WholeProposalReadPasses` (`internal/job/lint/r10_proposal_read_test.go`). With the fix stashed, exit 1: `brain-proposals/tally-money-handling.md: whole proposal read refused: [{Rule:job.read-range ... Severity:error Message:Read pointer has no range: brain-proposals/tally-money-handling.md ...}]`, and the same for `task:brain-proposals/...`. On the lane: `--- PASS`, exit 0. It also proves that `task:research.md`, `brain-proposals/sub/x.md` and a missing proposal are still `job.read-range`. |
| `make lint` | HOLDS | `make lint`: exit 0 (`go vet ./...`, no findings). `gofmt -l internal/cli internal/job/lint`: empty. |
| Tests of internal/job/lint, the deliver/curator package (internal/cli) and importers | HOLDS | `go list -test -f '{{.ImportPath}} {{join .Deps " "}}' ./...` gives these packages: `cmd/grid`, `contract/formats`, root `.`, `internal/account`, `internal/braincmd`, `internal/cli`, `internal/contract`, `internal/golden`, `internal/jobgraph`, `internal/job/jobcmd`, `internal/job/lint`, `internal/land`, `internal/mcp`, `internal/packaging`, `internal/question`, `internal/settings`, `lib`. `go test -count=1` on all of them: every package ok, exit 0 (`cmd/grid` has no test files). |
| GOOS=windows build | HOLDS | `GOOS=windows go build ./...`: exit 0 |
| GOOS=darwin build | HOLDS | `GOOS=darwin go build ./...`: exit 0 |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: ok, exit 0. `go test -count=1 ./internal/job/lint/`: ok, exit 0. Both were run after the merge. |
| Fetch and merge origin/main before the last proof run | HOLDS | `git fetch origin && git merge origin/main`: "Already up to date." |
| No existing test edited | HOLDS | `git diff origin/main --stat` shows two new test files and two production files. |
| Rerun on a head with the lane doc | HOLDS | The proofs above ran on `24e5d934`. Later commits change only `docs/lanes/C-r10-curator-job-renders.md`. The same 17-package `go test -count=1`, plus `./internal/orchestrator` (curator tests), was rerun after the doc commit: every package ok, exit 0. It includes both plugin walk packages. |
| No guard, gate, cap, hold or hook on agents | HOLDS | `lint.go:288` loosens the unranged-read rule for proposal reads. `lint.go:317` points a curator Job's coverage check at its title. Nothing is added. |
| No attribution lines, and no personal paths in the repo | HOLDS | The commit subjects are: "Add a test that the curator Job deliver posts for a waiting proposal passes the render lint and renders.", "Let the lint read a task brain proposal whole and check a curator Job's acceptance against its title, so deliver's curator Job renders.", "Add the lane proof table for C-r10-curator-job-renders." None has a trailer. The code adds no paths. |
| Whole-module test and cross-repo set | not run, not asked | The orchestrator runs these at landing. |
| RealPlugin tests | not run, not asked | |

## What changed

- `internal/job/lint/lint.go`: `proposalRead` pattern, exempted from the unranged-read rule. The coverage check for a curator Job uses its title.
- `internal/cli/deliver_keeper_job.go`: the curator acceptance row names the title's words. The doc comment says why.
- New tests: `internal/cli/r10_curator_job_renders_test.go` and `internal/job/lint/r10_proposal_read_test.go`.

## NOT VERIFIED

- That a live worker now runs the curator Job to the end and files the note into the brain. This lane proves that the Job renders, not the curator agent's run.

## Notes

- Budget: the lane used about 65 tool calls against the brief's "about 50".
- `job.acceptance-coverage` was a warning and never blocked render alone. The r10 refusal was the `job.read-range` error.

DONE
