# M-plugin-recall-hook-drop-ready

Merge of origin/main (7d516e1c, carries the recall guard drop and the host-guard warn change) into lane-C-eo-recall-hook-drop.

## Proof table

| Row | Result |
|---|---|
| 1. `git merge --no-edit origin/main`, conflicts listed and resolved | HOLDS. One conflicted file: `internal/cli/testdata/settings/hooks.json` (one single-line hunk). Resolved: took main's `"host-guard":"warn"` (lane side still had `block`, from before the host-guard warn lane) and took the lane's removal of `subagent-start`, `topic-trigger`, `user-prompt`. Every other entry of main is kept: history-log, home-allow, job-lint, needs-you, only-writer, plan-guard, session-check, session-end, session-start, stop-check, usage-log. Main carries no recall guard entry (dropped by lane-C-eo-recall-guard-drop) and the merged file carries no recall hook entry. All other files auto-merged (docs/settings.md, golden/known-differences.txt, internal/hook/events_test.go, internal/settings/catalog.go). |
| 2. Golden generated or compared | HOLDS. Not generated: no generator or update flag exists (grep of internal/cli found none). It is compared by `TestSettingsJSONGolden` (`internal/cli/settings_test.go:13`, subtest `hooks`, runs `hook levels --json` and compares bytes to `testdata/settings/hooks.json`). Edited by hand; `go test -count=1 -run TestSettingsJSONGolden ./internal/cli/` -> `ok`, exit 0. |
| 3. `go test -count=1 ./internal/cli/...` | HOLDS, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/cli  55.964s`. Also `go test -count=1 ./internal/hook/... ./internal/settings/...`: `ok .../internal/hook 2.787s`, `ok .../internal/settings 0.372s`. No test edited, dropped or weakened (only hooks.json changed by hand; the lane's tests in internal/hook/dy_recall_hook_test.go still pass). |
| 4. `make lint`, `go build ./...` | HOLDS, both exit 0 (`make lint` runs `go vet ./...`, no output). Also `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0. |
| 5. `git log --oneline origin/main..HEAD` | HOLDS: 12ef4963 Merge remote-tracking branch 'origin/main' into lane-M-plugin-recall-hook-drop-ready; b2338860 Add the lane proof table and gofmt brain.go; 531dac68 Drop the brain recall and topic hooks, their switches, golden cases and docs rows; 87f5ffc8 Add the tests that no hook injects brain recall or topic text. |
| Rule: no guard/gate/cap added | HOLDS. The merge adds none; the file only loses entries. |
| Rule: no attribution lines in commits | HOLDS. The merge commit is the default message. |
| Rule: new tests fail on base | N/A. No new tests in this lane; the lane's earlier tests are unchanged. |
| Rule: dependents of changed packages | The only changed non-doc file is a testdata file under internal/cli; `go list` importers of internal/cli were not run beyond cli, hook and settings (orchestrator runs the whole module at landing). |
| Rule: RealPlugin tests | N/A. No row needs the built plugin. |

## What changed

Only the merge commit and the hand-resolved `internal/cli/testdata/settings/hooks.json`, plus this report.

## NOT VERIFIED on a live screen

Nothing; no UI row.
