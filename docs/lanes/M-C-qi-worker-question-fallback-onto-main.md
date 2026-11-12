# M-C-qi-worker-question-fallback-onto-main

Lane C-qi-worker-question-fallback merged with plugin main f09dda3a. Base of this lane: main f09dda3a; merge commit 70e622c4 (head before this doc). The carried lane's own proof table is `docs/lanes/C-qi-worker-question-fallback.md`.

## Conflict

One file: `skills/questions/SKILL.md`, step 2 (the `Ask:` item). Main added `[--waits-on '<option label>']` and its explanation to the `ask` command line; the lane rewrote the "An agent without GRID_TASK_EPOCH" sentence in the next paragraph. Resolved by keeping main's command line and `--waits-on` text unchanged and the lane's sentence unchanged: the `decisions_needed` fallback. Nothing from either side was dropped. No test, row or error code touched.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 `git merge-base --is-ancestor origin/main HEAD` | HOLDS | exit 0. `git diff origin/main --stat`: `docs/lanes/C-qi-worker-question-fallback.md`, `internal/cli/qi_worker_fallback_test.go`, `skills/questions/SKILL.md` (1 line changed): only the carried lane's files (plus this doc). |
| 2.1 `TestQiOldFallbackRefused` | HOLDS | `go test -count=1 -v -run 'TestQi\|TestKb3\|TestSkillCommandsImplemented' ./internal/cli/ ./internal/packaging/` exit 0: `--- PASS: TestQiOldFallbackRefused`, `--- PASS: TestQiOldFallbackRefusedWithJobEpoch`. |
| 2.2 `TestQiWorkerFallbackRuns` red on base, green on lane | HOLDS | With main's SKILL.md put back (`git checkout origin/main -- skills/questions/SKILL.md`): `qi_worker_fallback_test.go:120: "grid job post --person": exit 2: flag needs an argument: -person` / `--- FAIL: TestQiWorkerFallbackRuns`. File restored (`git checkout HEAD -- skills/questions/SKILL.md`); on the merged head: `--- PASS: TestQiWorkerFallbackRuns`. |
| 2.3 grep and kb3 | HOLDS | `git grep -n 'it posts a person Job with `grid job post --person` and stops' -- skills` prints nothing (exit 1). Same test run above: `--- PASS: TestKb3QAskByWorkerIsRefusedNoJob`, `--- PASS: TestSkillCommandsImplemented`. |
| 2.4 lint, cross, stat | HOLDS | see row 4; stat only the lane's files (row 1). |
| 3 walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (ok); `go test -count=1 ./internal/job/lint/` exit 0 (ok). The app-only walk tests do not apply to this repo. |
| 4 `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| 4 `make cross` | HOLDS | exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| Tests of changed packages and importers | HOLDS | Skill is embedded by the root package; `go test -count=1 . ./cmd/grid ./contract/formats ./internal/packaging ./internal/cli` exit 0: root ok, contract/formats ok, internal/packaging ok, internal/cli ok (cmd/grid has no test files). |
| No existing test edited, none deleted | HOLDS | diff against origin/main touches only the carried lane's files. |
| No guard, gate, cap, hold | HOLDS | one sentence of skill text and one test file. |
| RealPlugin tests | not applicable | no row names one. |

## What changed

Merged origin/main into the lane and resolved `skills/questions/SKILL.md` as above. Nothing else.

## NOT VERIFIED on a live screen

Nothing here is UI; not run with a live agent.

Verdict: DONE
