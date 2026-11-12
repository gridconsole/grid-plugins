# C-ti-outcome-item-titles-head

The delivered Outcome names each plan item by title and says which commit was delivered.

Base: `792b8d6e` (main at clone time; the brief quoted `7a4bc0c2`, and the lines it names matched on this base).
Base line, `internal/task/deliver.go:230` on base:
`outcome += fmt.Sprintf("- %s: %s proof saved in proof/%s/\n", item.ID, item.Level, item.ID)`

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. TestTiOutcomeTitles red on base, green on lane | HOLDS | Base (deliver.go stashed): `go test -count=1 -v -run TestTiOutcome ./internal/task/` exit 1, `ti_outcome_items_test.go:65: Outcome lacks "- Check result (i-1): check, 1 file in proof/i-1/":` and the section held `- i-1: check proof saved in proof/i-1/`. Lane: same command exit 0, `--- PASS: TestTiOutcomeTitles`. |
| 2. TestTiOutcomeHead: `head:` equals `git rev-parse HEAD`, last line `Delivered from <branch> at <first 12>.`; no repo writes neither | HOLDS | Base: `ti_outcome_items_test.go:127: head: <nil>, want 86764742b1428d6c7d87425b22659d53494a955d`, `--- FAIL: TestTiOutcomeHead`. Lane: `--- PASS: TestTiOutcomeHead`, subtests `in_a_repo` and `outside_a_repo` PASS (exit 0). The outside case passes on base too, as it asserts absence. |
| 3. Section starts with the summary, holds `### Proof`; `outcome:` unchanged | HOLDS | TestTiOutcomeTitles asserts section lines 1-2 are `Done.`, `Second line.`, the `### Proof` line, and `front["outcome"] == "Done."` in TestTiOutcomeTitles. Passes. Production: `summary` is still computed from the Outcome text before the Proof list and written by the unchanged `setScalar(mapping, "outcome", summary, true)` in deliver.go. |
| Extra: plural, one file, empty title | HOLDS | TestTiOutcomeCounts (`- Check result (i-1): check, 1 file in proof/i-1/`, `- i-2: check, 2 files in proof/i-2/` for a title of backticks only). Red on base (`ti_outcome_items_test.go:104`), PASS on lane. |
| 4. `go test -count=1 ./internal/task/ ./internal/cli/ ./internal/gitx/` exit 0, no existing test edited | HOLDS | exit 0: `ok internal/task 8.182s`, `ok internal/cli 83.860s`, `ok internal/gitx 1.105s`. `git diff main --stat` shows no existing test file. |
| 5. diff names only deliver.go, the new gitx file, the new test file | HOLDS | `git diff main --stat` before this report: `internal/gitx/ti_head.go`, `internal/task/deliver.go`, `internal/task/ti_outcome_items_test.go`. This report file (`docs/lanes/<ID>.md`, which the runbook requires) is the one further file. `make lint` exit 0 (`go vet ./...`). `make cross` exit 0 (darwin/arm64 and windows/amd64, vet and test compile). `gofmt -l internal/task internal/gitx` empty. |
| Importers of the changed packages tested | HOLDS | `go list` of packages whose Deps hold internal/gitx (this holds all that import internal/task): cmd/grid, account, approvefix, brain, braincmd, cli, doctor, env, feed, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, packaging, packaging/cmd, people, ports, projectenv, projectmd, projects, question, routine, settings, signal, task, updates, usage, where, workflow, lib. `go test -count=1` over all of them, plus gitx: exit 0, every line `ok` or `[no test files]` (orchestrator 319.8s, lib 86.7s). |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v`: `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, exit 0. `go test -count=1 ./internal/job/lint/`: `ok`, exit 0. |
| `grid deliver --help`, commands.json, golden/ unmoved | HOLDS | No flag or help text added; none of those files appear in `git diff main --stat`; golden tests pass inside `./internal/cli/` and `make cross`. |
| *RealPlugin* tests | N/A | No row names one; none run, none claimed. |
| No guard, gate, cap, hold or blocking hook added | HOLDS | The change only writes more text and one front matter key. Delivery still moves the task to waiting and raises no gate. |
| Cross-repo readers | HOLDS | The planner's check stands: app `fj_outcome_realplugin_test.go:61` reads `### Proof`, which is kept; app `outcome.go:27` reads the section as text. |
| Commits | HOLDS | Three one-sentence commits, no attribution lines. `git status --short` empty before this report. |

## What changed

- `internal/gitx/ti_head.go` (new): `HeadSHA(dir) (string, bool)` runs `git rev-parse --verify -q HEAD` through the package's `run`; false outside a repo and in a repo with no commit.
- `internal/task/deliver.go`:
  - The Proof line is `- <title> (<id>): <level>, <n> file(s) in proof/<id>/` (`tiOutcomeProofLine`); with an empty title it is `- <id>: <level>, ...`.
  - After the Proof list, `Delivered from <branch> at <first 12 of head>.` is added when both are known (`tiOutcomeDelivered`).
  - Front matter `head: <full sha>` is set beside `branch` and `base` when the head is known.
  - `recordBranch` now runs before the Outcome text is built; its body is unchanged.
- `internal/task/ti_outcome_items_test.go` (new): TestTiOutcomeTitles, TestTiOutcomeCounts, TestTiOutcomeHead.

## Behaviour to know

- The `Delivered from` line uses the branch `recordBranch` answers (a caller's `--branch` wins) and the HEAD of the repo holding Cwd. With `--branch` given but Cwd outside a repo there is no head, so no line and no `head:` key.
- On a detached HEAD there is no branch, so no `Delivered from` line, but `head:` is still written.
- Like `branch` and `base`, `head:` is not removed by a later delivery that finds no head; a stale key stays.

## NOT VERIFIED on a live screen

Nothing in this lane has a screen. The app's Outcome tab renders the section as text; how the new line wraps there is for a checker after landing.

Budget: the brief said about 45 calls; I used more, mostly on reading and on the full importer test run.

Verdict: DONE
