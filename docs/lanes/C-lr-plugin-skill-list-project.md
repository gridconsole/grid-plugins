# C-lr-plugin-skill-list-project: proof table

Base check HOLDS: on base, `internal/cli/settings.go:68-72` had `if listing {` with an empty body and `project = fs.String(...)` only in the `else`; so `list` defined no `project` (settingsStore at :32-50 only resolved cwd or Personal).

| Row | Result | Proof |
|---|---|---|
| 1 Per project | HOLDS | `go test -count=1 -v -run TestLrSlp ./internal/cli/` exit 0: `--- PASS: TestLrSlpListsForTheNamedProject`. On base (settings.go stashed) it FAILS: `[skill list --project beta --json]: exit 2, ... flag provided but not defined: -project` (same for alpha and for `agent list`). |
| 2 No flag, no change | HOLDS | `go test -count=1 -v -run 'TestSettingsJSONGolden\|TestSettingsCLIOverridesAndFailures\|TestEzAppKeysListsDoNotShowThem' ./internal/cli/` exit 0, `--- PASS:` for each of the three. `git diff main -- internal/cli/testdata/` is empty (diff --stat shows no testdata). |
| 3 Refusals | HOLDS | `TestLrSlpRefusals` PASS: `skill list --project nosuch` (and `agent list`) exit 1, empty stdout, stderr `skill: project "nosuch" is not registered\n`; guest gus `--project alpha` exit 0, `--project beta` exit 1 empty stdout. `go test -count=1 -run 'TestSettingsPersonalReadsRefused\|TestSettingsPersonalWritesRefused' ./internal/cli/` PASS (exit 0). |
| 4 Contract | HOLDS | `git diff main -- contract/commands.json`: two hunks (@@ -3554 agent list, @@ -3796 skill list) each add `--project` takes `value`, required false; nothing under `hook levels`. `TestContractCommandsParserInventory`, `TestContractCommandsInventory`, `TestContractCommandsApp` each `--- PASS:`. `make generate` touched only contract/commands.json (git status showed no other file changed), so nothing was dropped. |
| 5 Walks + packages | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok; `go test -count=1 ./internal/cli/ ./internal/settings/ ./internal/where/ ./internal/job/lint/ ./cmd/grid/ ./lib/` exit 0. `git merge main`: Already up to date. Importers of internal/cli (go list): cmd/grid, lib. |
| 6 lint, cross, stat | HOLDS (with one stated addition) | `make lint` exit 0; `make cross` exit 0 (darwin/arm64 and windows/amd64 vet + test compile). `git diff main --stat`: settings.go, commands.json, the new test file, and `internal/cli/guest_parity_test.go` (+2 lines, see below), plus this report. |

## Rules

- Commits: one-sentence messages, no attribution lines. HOLDS.
- New tests fail on base, pass on lane: HOLDS (failing lines pasted in row 1).
- No existing test edited to pass; no test deleted. `guest_parity_test.go` gained two map rows (`"agent list": {}`, `"skill list": {}`), no assertion changed. HOLDS, but outside the brief's Touches: see below.
- No guard, gate, cap or hold added on agents. HOLDS: the lane only defines a flag that goes through the existing where.Resolve.
- Walk tests: `TestHsIpWalksEveryCommand|TestIlGuestSiteList` ok; `internal/job/lint` ok. The app walk tests do not apply (plugin repo).
- `*RealPlugin*` tests: none named by the brief; none run.

## What changed

- `internal/cli/settings.go`: `agent list` and `skill list` define `--project` (resolved through settingsStore and where.Resolve, as the writes do); `hook levels` still has no `--project` (`TestLrSlpHookLevelsTakesNoProject`, exit 2). Usage lines at :21 and :23 name `[--project P]` on the list form.
- `contract/commands.json`: `--project` on `agent list` and `skill list`.
- `internal/cli/lr_skill_list_project_test.go`: three tests (`TestLrSlp...`).
- `internal/cli/guest_parity_test.go`: the first `make test` run of internal/cli was RED, `TestGuestProjectParity`: `missing guest parity command: agent list` and `skill list`. The walk requires a row for every contract command that has `--project`. Adding the two rows (empty base args) makes it PASS, so the guest-versus-invisible-project outputs of the new forms are compared byte for byte. The brief said new tests go in the new file only, but this is a walk table that a new `--project` flag cannot avoid; reviewer to confirm.

## NOT VERIFIED on a live screen

Nothing here is UI. The app's skill_usage handler (C-lr-skill-usage-handler) calling `grid skill list --project <p> --json` is not run here; no `*RealPlugin*` run was needed.

Verdict: DONE
