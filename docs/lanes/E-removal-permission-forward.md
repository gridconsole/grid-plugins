# E-removal-permission-forward: drop the permission-forward hook

Owner's direction: gaps-11nov g-removals names the permission-forward hook; Claude Code's own permission prompt and deny rules replace it.

## Proof table

| Row | Result |
|---|---|
| 1. Other users of PermissionRequest, and whether the hooks.json entry stays | HOLDS. The home fallback is the other user: `internal/hook/hook.go:230` (the CLI event `permissionrequest` selects the event), `hook.go:250-266` (guards are rechecked, then the home allow is converted to the PermissionRequest decision shape), `internal/guard/home.go:297` (home grant as `addDirectories`), and the too-large deny at `hook.go:242-243`. The entry in `hooks/hooks.json:3` therefore stays; its timeout drops from 100 to 10 s because nothing waits for a person any more (`events.go:46`, `hooks.json:9`). |
| 2. permission.go deleted, nothing forwards; `grep -rn "forwardPermission\|permission-forward" internal hooks` empty | HOLDS. `git rm internal/hook/permission.go` and `permission_test.go`; the call in `hook.go` is gone; the `permission-forward` component is gone from `events.go` (map and switch case). The grep exits 1 with no output. `GRID_PERMISSION_TIMEOUT_MS` is gone with it. |
| 3. Home fallback still needs PermissionRequest, so its Go test passes unchanged | HOLDS. No home-fallback test was edited: `go test ./internal/guard/ ./internal/hook/` exit 0 (`guard/home_test.go:49,77,191`, `guard/home_personal_test.go`, `hook/tooluse_test.go`, `hook/settings_test.go`, `hook/hold_hook_test.go:177-179` all pass as on main). The hooks.json entry stays, so no removal test is needed. |
| 4. `go test ./...` and `make lint` | HOLDS. `go test ./...` exit 0, every package ok. `make lint` (go vet ./...) exit 0. |

## Rules

| Rule | Result |
|---|---|
| New test fails on base, passes on the lane | HOLDS. `internal/hook/noforward_test.go` `TestPermissionRequestIsNotForwardedToGrid`. On main: `FAIL ... noforward_test.go:27: forwarded: code=0 out="{\"hookSpecificOutput\":{\"hookEventName\":\"PermissionRequest\",\"decision\":{\"behavior\":\"allow\"}}}" log="" calls=1`, exit 1. On the lane: PASS, exit 0. |
| No existing test edited to pass | HOLDS, with two deletions. Deleted `internal/hook/permission_test.go` (TestPermissionForwardAndGuardPrecedence, TestPermissionTimeoutAndInvalidResponseAllow, TestPermissionTimeoutDefaultOverrideAndCap, TestPermissionSlowAnswerWithinWindowIsUsed, TestPermissionForwardCannotApprovePersonal): the behaviour was removed by the owner's direction (this brief, g-removals). Local guard precedence and Personal refusal on PermissionRequest stay covered by `internal/cli/home_personal_test.go` TestHookPersonalScope, `internal/hook/tooluse_test.go` and `settings_test.go`. Two lists lost the dead `permission-forward` name: `internal/cli/home_personal_test.go:14` (event list; the `PermissionRequest` case still covers the same path) and `internal/golden/corpus_test.go:49` (a component map key; that recorded case is now skipped because `hook.Handles` is false). Both are outside the brief's Touches list and needed for row 2 and for TestHookPersonalScope to pass. |
| No guard, gate, cap, hold or blocking hook added | HOLDS. Only code was removed; the new test adds no runtime code. |
| Tests of changed packages and their importers | HOLDS. Importers from `go list` (Imports, TestImports, XTestImports): cli, golden, hook, packaging. `go test ./internal/hook/ ./internal/cli/ ./internal/golden/ ./internal/packaging/ ./internal/doctor/ ./internal/guard/` exit 0. |
| Lint, Windows and macOS builds | HOLDS. `make lint` exit 0; `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0; `gofmt -l internal` empty. |
| Public repo: no personal paths, hosts or emails | HOLDS. None added. |

## What changed

- Deleted `internal/hook/permission.go` and `permission_test.go` (the POST to `GRID_URL/hooks/permission`, its timeout env and its tests).
- `internal/hook/hook.go`: removed the `forwardPermission` call. A PermissionRequest no guard decides now prints nothing, so Claude Code asks in its own prompt and applies its own deny rules.
- `internal/hook/events.go`: removed the `permission-forward` component and its switch case; PermissionRequest timeout 100 to 10 s; comment updated.
- `hooks/hooks.json`: PermissionRequest timeout 100 to 10 s (still generated from the `events` table).
- Added `internal/hook/noforward_test.go`.
- Left alone (not in Touches): historical docs and the `golden/` recordings and `recordings/` scripts that mention permission-forward; `docs/old-plugin-map.md` describes the old plugin.

## NOT VERIFIED on a live screen

- That Claude Code shows its own permission prompt for an undecided request with the plugin installed (needs a live session).
