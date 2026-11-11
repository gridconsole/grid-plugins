# E-verdict-5-stop: one stop check, that the Job's result is posted

Verdict v-5 (verdicts-11nov, approved by silence): keep one stop check.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Code that tells whether the Job has a result on the Feed | HOLDS | `jobHasResult` at internal/hook/events.go:340 reads the task Feed (`feed.Store.Read`, filtered to the Job) and answers true for a `result` (or earlier `worker-exit`) record of the Job's attempt. `checkStop` calls it at internal/hook/stop.go:128. It already existed for `workerExit` (events.go:280) and is reused unchanged. The Feed fold that records the same fact is internal/contract/fold.go:343-344 (`Job.Results`). |
| 2. checkStop keeps only that check; open Job with no result told once in one plain line; brief-section, task.md-field and done-brief loops removed | HOLDS | internal/hook/stop.go:115-142: no `GRID_JOB` returns 0; a scout returns 0; no result prints `Post the result of Job <id> on the Feed, then stop.` and returns 2; `stop_hook_active` returns 0. Loops and `section()` removed, `git diff main -- internal/hook/stop.go`. |
| 3. No Job (plain session, scout): Stop and SubagentStop return 0, print nothing | HOLDS | `TestStopWithNoJobStopsSilently` (Stop, SubagentStop builder, SubagentStop scout, with a done brief that has no Outcome in the task) and `TestSubagentStopScoutWithJobEnvStopsSilently`: `go test ./internal/hook/...` exit 0. |
| 4. Go tests: Job with result 0; without result 2 then 0 on stop_hook_active; no Job 0 | HOLDS | `TestStopJobWithResultStops`, `TestStopJobWithoutResultBlocksOnce`, `TestStopWithNoJobStopsSilently` in internal/hook/stop_test.go, each for Stop and SubagentStop. Fail on base (main's stop.go and verify.go put back, new tests kept), pasted below. Deletions listed below. |
| 5. `go test ./internal/hook/...` and `make lint` 0 fail | HOLDS | `go test ./internal/hook/...` -> `ok github.com/gridconsole/grid-plugins/internal/hook`, exit 0. `make lint` (go vet + gofmt check) exit 0 after `gofmt -w internal/hook/events_test.go`. |
| Rule: importers of internal/hook tested | HOLDS | `go list` importers of internal/hook: internal/cli, internal/packaging. `go test ./internal/hook/... ./internal/cli/... ./internal/packaging/...` see "Importer tests" below. |
| Rule: Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Rule: no guard, gate, cap, hold or blocking hook added | HOLDS | Nothing added. The one block is the pre-existing stop-check exit 2, now narrower, and it already lets go on `stop_hook_active`. The verify-on-stop cap (8 blocks, 2 on codex) is removed with the verification it limited. |
| Rule: small commits, one-sentence messages, no attribution, no personal paths | HOLDS | `git log main..HEAD`. |
| Rule: only hook.go/catalog untouched, only internal/hook edited | HOLDS | `git diff --stat main`: internal/hook/{stop.go,verify.go (deleted),events_test.go,levels_test.go,stop_test.go,verify_test.go (deleted)} and this file. |

### New tests fail on base

```
--- FAIL: TestStopJobWithResultStops/Stop: code 0 out "" log "grid hook: verify: lstat .../jobs: no such file or directory"
--- FAIL: TestStopJobWithoutResultBlocksOnce/Stop: code 0, want 2: grid hook: verify: lstat .../jobs: no such file or directory
--- FAIL: TestStopJobWithoutResultBlocksOnce/SubagentStop: code 0, want 2:
--- FAIL: TestStopWithNoJobStopsSilently: Stop  with no Job: code 2 out "" log "Not finished yet:\n- 01-build.md: write the Outcome section.\nFix this, then stop again.\n"
```

### Deleted or changed existing tests

- `TestStopGuardBlocksOnceAndMarksOwnedBrief` (events_test.go) deleted: the brief Outcome/Verdict check and brief marking are removed by the owner's direction (verdicts-11nov v-5). `TestStopJobWithoutResultBlocksOnce` still covers the remaining check (block once, then stop on `stop_hook_active`).
- `TestStopRunsSavedJobVerificationAndCapsBlocks` and `TestVerificationCountFormats` (verify_test.go) deleted with verify.go: running the saved Job's verify command at Stop is not the one remaining check (v-5). The same verification still runs under the Job commands (internal/job/verify.go and its tests in internal/job). `TestStopJobWithResultStops` covers the remaining Stop path for a Job.
- `TestCatalogStopLevels` (levels_test.go) edited, not deleted: its Stop, SubagentStop and stop-check cases used a done brief to trigger the removed check, so they now use a Job without a result (`GRID_JOB=J-1`); the `verify` kind (a failing verify command) is dropped. Level coverage (off, warn, block, project, broken settings) is the same and passes.

### Importer tests

```
ok  github.com/gridconsole/grid-plugins/internal/hook      8.989s
ok  github.com/gridconsole/grid-plugins/internal/cli      71.383s
ok  github.com/gridconsole/grid-plugins/internal/packaging (cached)
```
Exit 0.

## What changed

`checkStop` (internal/hook/stop.go) now checks one thing: when `GRID_JOB` names a Job and the Feed holds no result for it, tell the agent once to post it (exit 2); the retry (`stop_hook_active`) and everything else stops at once. Scouts and sessions without a Job return 0 silently. Removed: brief Outcome/Verdict check and marking, task.md summary/next check, done-brief loop, `section()`, and the saved-Job verification (verify.go).

## Decisions to check

- The verification-on-stop (verify.go) is removed, because the brief says checkStop keeps only the result check. If the owner wants it kept, restore verify.go from main and call `verifyStop` first in `checkStop`.
- SubagentStop applies the check when `GRID_JOB` is set in the environment, except for scouts. A non-scout subagent that inherits its parent worker's `GRID_JOB` is held to the same result check.
- Left alone, outside Touches: docs/settings.md:116 still says stop-check includes "saved Job verification"; `session.Activity.VerifyBlocks/VerifyFailed` are now unwritten; `task.MarkBrief` has no caller outside tests.

## NOT VERIFIED on a live screen

Nothing here is a UI row. A real Stop and SubagentStop in a running Worker with the real Feed is for the orchestrator's live check.
