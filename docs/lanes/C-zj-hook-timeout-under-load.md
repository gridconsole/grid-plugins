# C-zj-hook-timeout-under-load

Guard hooks get 30 s so a loaded machine does not skip them. This uses Claude Code's own hook `timeout` field; no Grid-made limit is added and no guard decision changed.

## Proof table

| Row | Result | Proof |
|---|---|---|
| hooks.json shows 30 for PreToolUse, PermissionRequest, UserPromptSubmit; old values elsewhere | HOLDS | `grep -n timeout hooks/hooks.json`: PermissionRequest 30 (line 9), PostToolUse 10, PostToolUseFailure 10, PreCompact 10, PreToolUse 30 (line 53), SessionEnd 10, SessionStart 10, Stop 60, SubagentStart 10, SubagentStop 60, UserPromptSubmit 30 (line 120) |
| `make generate` then `git diff --exit-code hooks/hooks.json` | HOLDS | `make generate` exit 0; `git diff --exit-code hooks/hooks.json && echo exit0` printed `exit0`; `git status --short` clean |
| `go test ./internal/hook/ -count=1` | HOLDS | `ok  github.com/gridconsole/grid-plugins/internal/hook 10.252s` |
| `go test ./internal/guard/ -count=1` | HOLDS | `ok  github.com/gridconsole/grid-plugins/internal/guard 2.807s` |
| New test fails on base, passes on lane | HOLDS | Base (events.go stashed): `--- FAIL: TestGuardHookTimeoutsSurviveLoad` / `UserPromptSubmit timeout 10, want 30` / `PreToolUse timeout 10, want 30` / `PermissionRequest timeout 10, want 30`. Lane: passes inside the hook package run above |
| Importers of changed package tested | HOLDS | `go list` importers of internal/hook: internal/cli, internal/packaging. `go test -count=1 ./internal/cli ./internal/packaging`: both `ok` (87.0 s, 7.9 s) |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0, no output |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: ok; `go test -count=1 ./internal/job/lint/`: ok. The app-only walks do not apply to this repo |
| No existing test edited or deleted | HOLDS | Only a new file internal/hook/zj_timeouts_test.go was added |
| No guard, gate, cap or hold added | HOLDS | Only the timeout integers in the events table changed |
| Touches list kept | HOLDS | Files changed: internal/hook/events.go, hooks/hooks.json, internal/hook/zj_timeouts_test.go, docs/lanes/C-zj-hook-timeout-under-load.md. `make generate` also rewrites doctor and contract goldens; they came out unchanged (status clean after generate) |
| RealPlugin tests | n/a | No row names one |

## What changed

- internal/hook/events.go: timeout 10 to 30 for UserPromptSubmit, PreToolUse and PermissionRequest; comment updated.
- hooks/hooks.json: regenerated, three timeout values.
- internal/hook/zj_timeouts_test.go: new test pinning all eleven event timeouts. No test pinned them before.

## NOT VERIFIED on a live screen

Behaviour under load 350-450 in a real Claude Code session; the tests pin the registered value only.

DONE
