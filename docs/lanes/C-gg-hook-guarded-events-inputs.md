# C-gg-hook-guarded-events-inputs

Base: plugin main b87fc36458332a9e2bd680ff9e3ed5a896638900 (lane branch start).
I read records_test.go:19-135 (homes :38-42, inputs :57-65, assertNoDecision :123) and tooluse_test.go:115-121 is the one-input `{` guard test the new walk widens. records_test.go is not edited; assertNoDecision is reused.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. TestGgGuardedInputsNeverDecide: 4 events, 6 inputs, 4 homes (under a file, read-only, missing, fresh empty), exit 0 and assertNoDecision, no GRID_JOB/GRID_FEED_TASK | HOLDS | `go test -count=1 -v -run TestGgGuardedInputsNeverDecide ./internal/hook/` exit 0; 192 subtests PASS (4 homes x 6 inputs x 4 events x 2 spellings); gg_guarded_inputs_test.go:78-124 |
| 2. Same walk with lower-case event name | HOLDS | Same test, the `lower` and `table` spellings are the two subtests per case (gg_guarded_inputs_test.go:113) |
| 3. TestGgGuardedEventsMatchTheRegistration | HOLDS | `go test -count=1 -v -run TestGgGuardedEventsMatch ./internal/hook/` PASS; the test also checks that ggMissing reports a name in neither list (gg_guarded_inputs_test.go:128-157) |
| 4. Contrast: well-formed PermissionRequest Write to task.md denied, exit 0; cut at 40 bytes answers no decision | HOLDS | TestGgGuardedContrastDenyWellFormedNotCut PASS (gg_guarded_inputs_test.go:160-190), eventSetup fixture, settings `{"hooks":{"host-guard":"block"}}` |
| 5a. diff names only the new file | HOLDS | `git diff main --stat`: the new test file and this report only (docs/lanes/ is the report the runbook asks for) |
| 5b. `go test -count=1 -run TestGgGuarded ./internal/hook/` | HOLDS | exit 0, 0 fail |
| 5c. `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| 5d. `GOOS=windows go vet ./internal/hook/` | HOLDS | exit 0 |
| Mac build | HOLDS | `GOOS=darwin go vet ./internal/hook/` exit 0 |
| Packages that import internal/hook (go list) | HOLDS | internal/cli, internal/packaging; `go test -count=1 ./internal/hook/ ./internal/cli/ ./internal/packaging/` all ok, exit 0 |
| No home read or written | HOLDS | ggIsolate sets HOME, CLAUDE_CONFIG_DIR, CODEX_HOME, GRID_HOME under t.TempDir() and t.Fatal when HOME is not under it (:30-43); Getenv returns no key; no child process is started |
| New tests fail on base | NOT APPLICABLE | The walk found no bad answer: all four events already exit 0 with no decision on every input, so the tests pass on base and on the lane. No production code changed. Row 3 fails when a name is missing (self-check inside the test). |
| No guard, gate, cap or hook added | HOLDS | only a test file added |
| Real-plugin tests | NOT APPLICABLE | no test named *RealPlugin* in this lane |
| Commit rules | HOLDS | one-sentence commits, no attribution lines |

## What changed
One new test file, internal/hook/gg_guarded_inputs_test.go, plus this report. No production code.

## Not verified on a live screen
Nothing here needs a screen. The tests call Run directly; a real `grid hook` process under Claude Code was not run.

DONE
