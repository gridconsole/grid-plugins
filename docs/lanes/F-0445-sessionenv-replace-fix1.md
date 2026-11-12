# F-0445-sessionenv-replace-fix1: fix round for F-0445-sessionenv-replace

Branch lane-F-0445-sessionenv-replace-fix1, from lane-F-0445-sessionenv-replace. Head: see the report line.

## Cause

`TestHeMvMessagesNameRealSubs` (internal/contract/he_mv_message_verbs_test.go) walks the string literals of the non-test Go files and checks every `grid <verb> <sub>` against contract/commands.json. The lane's marker lines `# >>> grid session env >>>` and `# <<< grid session env <<<` (internal/hook/hook.go:207-208) read as the command `grid session env`; `env` is not a sub of `grid session`. Failure on the unfixed lane:

```
he_mv_message_verbs_test.go:208: internal/hook/hook.go:207: `grid session env`: "env" is not a sub of `grid session` in contract/commands.json; ...
he_mv_message_verbs_test.go:208: internal/hook/hook.go:208: `grid session env`: "env" is not a sub of `grid session` in contract/commands.json; ...
FAIL	github.com/gridconsole/grid-plugins/internal/contract
```

## Fix

The marker words are now `grid-session-env` (`# >>> grid-session-env >>>`, `# <<< grid-session-env <<<`). They name no command, so no test is edited and no `heMvProse` entry is added. The five goldens that hold the marker lines were re-worded to match (same line count and layout). `internal/hook` tests use the `envBlockStart`/`envBlockEnd` constants, so they follow without change.

## Proof table

| Row | Result |
| --- | --- |
| `git merge origin/main` | HOLDS: `Already up to date.`, exit 0; no file resolved (the lane was already built on origin/main 792b8d6e) |
| `go test ./... -count=1 -run 'TestHeMvMessagesNameRealSubs'` before the fix | FAILED as quoted above |
| `go test ./... -count=1 -run 'TestHeMvMessagesNameRealSubs'` after | HOLDS: `ok  github.com/gridconsole/grid-plugins/internal/contract 0.126s`, no FAIL line, exit 0 |
| `go test ./internal/contract/ -count=1` | HOLDS: `ok  github.com/gridconsole/grid-plugins/internal/contract 0.535s`, exit 0 |
| `make lint` | HOLDS: `go vet ./...`, exit 0 |
| `make cross` | HOLDS: darwin/arm64 and windows/amd64 vet and test compile, every package `ok`, exit 0 |
| `git diff origin/main --stat` names only F-0445 files | HOLDS: docs/lanes/F-0445-sessionenv-replace.md, the five go-expected/files.diff goldens, internal/hook/{hook.go,ex_outside_home_test.go,sessionenv_replace_test.go}, plus this lane doc; no other test file touched |
| Tests of changed packages and importers (`go list`: internal/cli, internal/hook, internal/packaging) | HOLDS: `go test -count=1 ./internal/hook/ ./internal/golden/ ./internal/cli/ ./internal/packaging/ ./internal/job/lint/` all `ok`, exit 0 |
| Walk tests `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` | HOLDS: `ok .../internal/cli 0.028s` |
| Walk test `go test -count=1 ./internal/job/lint/` | HOLDS: `ok .../internal/job/lint 0.117s` |
| App-only walk tests | n/a: plugin repo |
| No test deleted, skipped or loosened | HOLDS: no test file changed in this round |
| No guard, gate, cap or hold added | HOLDS: only the marker words changed |
| `*RealPlugin*` tests, UI rows | n/a |

## What changed

- `internal/hook/hook.go`: marker constants reworded.
- `golden/agent-routing/{empty-object,session}`, `golden/grid-env/{discovered,explicit,no-config}` (`go-expected/files.diff`): the two marker lines reworded.
- `docs/lanes/F-0445-sessionenv-replace.md`: the marker words in "What changed" updated, with a pointer to this round.

## NOT VERIFIED on a live screen

Nothing here is UI. A marker written by an earlier build (`grid session env` words) is now an ordinary line to the new code; the new block is appended after it and its values win when the file is sourced, as for any foreign copy. No migration is added; I did not check whether the old marker words reached a release.
