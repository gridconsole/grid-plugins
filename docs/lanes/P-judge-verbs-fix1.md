# P-judge-verbs-fix1: list grid judge in the walk tables, update the judge text pin

Lane branch lane-P-judge-verbs-fix1, from lane-P-judge-verbs (998030c0). It carries P-judge-verbs and closes that lane's two NOT DONE rows (`docs/lanes/P-judge-verbs.md:17` and `:20`). I ran `git fetch origin` and `git merge origin/main` ("Already up to date.") before the first edit. Every proof run below came after them.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. `go test -count=1 ./internal/judge/... ./internal/cli/... ./internal/job/lint/...` exits 0 | HOLDS | exit 0: `ok .../internal/judge 0.004s`, `ok .../internal/cli 38.735s`, `ok .../internal/job/lint 0.213s` |
| 2. TestGgUnknownProjectWalk passes, with no new skip entry, and the diff shows the change in internal/cli/judge.go | HOLDS | The `fail` helper now calls `env.recordError(err)` before the stderr line, as `internal/cli/feed.go:89` does (`internal/cli/judge.go:100-106`). The deferred `commandFailure.finish` (`internal/cli/error_codes.go:126`) then prints `{"error":{"code":"no_such_project","message":<stderr>}}` on stdout with `--json`. The stderr text is unchanged. Before the fix (walk rows added, judge.go untouched): `gg_unknown_project_walk_test.go:319: judge review [judge review J-01 --project absent]: stdout is empty: exit 1; stderr "project \"absent\" is not registered"; stdout ""`. The same line appeared for person, rework and verdict. `TestGgUnknownTaskWalk/judge_*` failed the same way: `empty stdout: grid judge review J-01 --task T-99 --json: exit 1, stderr: task T-99 not found in project example`. After the fix, `go test -count=1 -run 'TestGgUnknown\|TestGuestProjectParity\|TestLtReadme' ./internal/cli/` exits 0. `git diff --stat 998030c0..HEAD -- internal/cli/gg_unknown_project_walk_test.go` prints nothing, so `ggProjectSkip` and `ggProjectKnownOpen` are still empty. `ggTaskSkipped` and `ggTaskKnownOpen` are unchanged too: the only change to `gg_unknown_task_walk_test.go` is one added line in `ggTaskWalked` |
| 3. r23_judge_text_test.go still exists, pins the new sentence, and fails on lane-P-judge-verbs | HOLDS | The pin is at `internal/job/lint/r23_judge_text_test.go:26`. The test name, its `never_judged` pin and its purpose (the doc says who posts the review and the rework) are kept, and the comment is updated (`:19-22`). It now pins `docs/job-contract.md:138`. To run it against the lane-P-judge-verbs doc, I ran `git checkout 998030c0 -- docs/job-contract.md`, then `go test -count=1 -run TestR23JobContract ./internal/job/lint/`, which exits 1: `--- FAIL: TestR23JobContractSaysTheJudgePostsReviewAndRework` `r23_judge_text_test.go:30: docs/job-contract.md does not contain "The review of a submitted Job and the rework after a rejection are posted by the plugin's ``grid judge review`` and ``grid judge rework``, and the app's Judge keeps running until A-worker-out"`. Then `git checkout HEAD -- docs/job-contract.md`, and `git status --short` printed nothing. On the lane, `go test -count=1 -run TestR23 -v ./internal/job/lint/` passes both R23 tests |
| 4. No attribution lines | HOLDS | `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` printed `0` |
| Ruling 1: three walk tables list the new command | HOLDS | `internal/cli/gg_unknown_task_walk_test.go:28` (the four judge members); `internal/cli/guest_parity_test.go:74-77` (`{"J-01"}`; rework adds `--from J-01.1`, so its usage check passes before the task is read); `README.md:40` (`grid judge` bullet in `## Commands`). `TestGgUnknownTaskWalkCoversTheContract`, `TestGuestProjectParity` and `TestLtReadmeNamesEveryCommand` pass (row 1). The parity rows passed with no `guestParityEarlyAnswer` entry and no leak |
| Ruling 3: the doc sentence is the new decided text | HOLDS | `docs/job-contract.md:138`. The plugin's `grid judge review` and `grid judge rework` post the review and the rework, and the app's Judge keeps running until A-worker-out. The old clause "nothing in the plugin reads a review's verdict" is now false, so it now says `grid judge verdict` reads `ACCEPTED`, `REJECTED` or `STUCK` and posts nothing (`internal/cli/judge.go`, `internal/judge/`). "Without the app's worker a done Job stays `submitted`" now reads "Without the app's worker and with no `grid judge review` call ...". The `never_judged` sentence is unchanged |
| Pinning test changed, not deleted (ruling 3) | HOLDS | Orchestrator ruling 3 (16:20) asks for this change to the existing test `TestR23JobContractSaysTheJudgePostsReviewAndRework`. Only the pinned string and its comment changed, and the change is in the same commit as the doc |
| New tests fail on base and pass on the lane | HOLDS | No new test function was added. The new walk rows and the new pin fail before the fix (rows 2 and 3 above) and pass on the lane (row 1) |
| No existing test edited to make it pass | HOLDS | The test files touched are two walk tables that the new command forces (ruling 1; runbook-reviewer "Touches", end review 0415Z M5), plus the pin that ruling 3 orders changed. No skip, known-open or early-answer entry was added |
| Importers tested | HOLDS | Running `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...` filtered on internal/cli, internal/judge and internal/job/lint gives `cmd/grid`, `internal/cli`, `internal/job/jobcmd`, `lib`. `go test -count=1 ./cmd/grid/ ./lib/ ./internal/job/jobcmd/` exits 0; internal/cli as row 1 |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok .../internal/cli 0.039s`); `go test -count=1 ./internal/job/lint/` exit 0 (row 1) |
| `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| `make cross` (Windows and macOS) | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package `ok` or `[no test files]` |
| gofmt | HOLDS | `gofmt -l internal/cli internal/judge internal/job/lint` printed nothing |
| No guard, gate, cap, hold or blocking hook | HOLDS | The only production change is `env.recordError(err)` in the failure path of `grid judge`. It changes what a failure prints, not whether a call runs |
| Touches | HOLDS | `git diff --stat 998030c0..HEAD`: README.md, docs/job-contract.md, docs/lanes/P-judge-verbs-fix1.md, internal/cli/judge.go, internal/cli/gg_unknown_task_walk_test.go, internal/cli/guest_parity_test.go, internal/job/lint/r23_judge_text_test.go. These are the files the brief's rulings name |
| Public repo: no personal paths, hosts or emails | HOLDS | The changed files name only repo paths |
| No sudo or root, nothing outside the lane clone and the hand folder, no push | HOLDS | none run |
| `make realplugin ... TestLcHostRealPlugin\|TestKdLeakRealPlugin\|TestKdProjRealPlugin` | not run, not asked | an app-repo guest walk; this lane is in the plugin repo |
| *RealPlugin* tests | not run, not asked | the brief names none |
| Whole-module `go test ./...` | not run, not asked | run by the orchestrator at landing. `make cross` compiles every test package |
| "then review P-judge-verbs" (brief title) | not run, not asked | no acceptance row; it is the orchestrator's review step (the brief says "Review: yes") |
| `grid job result` (grid:run-job) | not run, not asked | this lane came as a brief file with no Grid task or Job; the handoff is the bundle and this report |

## What changed

- `internal/cli/judge.go`: a failed `grid judge` call records its error. With `--json`, stdout then carries the failure object (code and message) that every other verb prints, e.g. `no_such_project` and `no_such_task`. Plain output is unchanged.
- `internal/cli/gg_unknown_task_walk_test.go`, `internal/cli/guest_parity_test.go`, `README.md`: the judge rows and the `grid judge` bullet (ruling 1).
- `docs/job-contract.md:138` and `internal/job/lint/r23_judge_text_test.go`: the decided sentence and its pin (ruling 3).

## NOT VERIFIED and open items

- Nothing here needs a live screen.
- `internal/orchestrator/write.go:326` (the `never_judged` orphan Detail) still says "the review and any rework are posted by the app's Judge". `agents/orchestrator.md:28` says the same and tells the orchestrator to wait for the app's Judge. `TestR23OrchestratorNamesNeverJudged` pins "app's Judge" in that agent text. Both stay true while the app's Judge runs (until A-worker-out), but neither names `grid judge`. They are outside this brief.
- Carried from P-judge-verbs, not in this brief:
  - With no task head, the rework base is the target branch name, not its tip SHA.
  - The review and rework bases are proven on hand-built records only.
  - Posts do not set the record's `attempt` header.
  - `grid judge rework` and `grid judge verdict` have no CLI end-to-end test.

DONE
