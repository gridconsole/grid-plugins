# C-sb3-render-skill-state-realplugin

Cause: the Grid app seeds its own keys (`claudePath`, `maxTurns`, `agentSandbox`) into the personal `settings.json` of the test home. C-sb2's `settings.ReadSkillsOff` read that file with the strict settings decoder, which refuses unknown keys (`settings.json: unknown key "claudePath"; allowed: ...`), and render returned `fail(err, 1)`. A home made by `grid project add` + `grid task new` has no such keys, which is why it rendered fine.

## Proof table

| Row / rule | Status | Proof |
|---|---|---|
| 1. New test shows `job render` exits 0 with ok true on a home with the app's keys; fails on C-sb2 without the fix | HOLDS | `go test -count=1 ./internal/job/jobcmd -run Sb3`: with the fix `--- PASS: TestSb3RenderAcceptsSettingsWithAppKeys`. Without the fix (skill_state.go stashed): `--- FAIL: TestSb3RenderAcceptsSettingsWithAppKeys ... exit 1 want 0; stdout {"ok":false,"findings":[],"error":"settings.json: unknown key \"claudePath\"; allowed: agents, skills, hooks, ..."}` and the same for `TestSb3RenderAcceptsUnreadableSettings` (`invalid character 'o' in literal null`). Tests: internal/job/jobcmd/sb3_render_skill_state_test.go |
| 2. A skill turned off at the personal level stays out of the Skills block | HOLDS | `--- PASS: TestSb3PersonalOffStillLeftOut` (settings holds the app keys and `skills.research=false`; Skills block has house-style, no research). Also still green: `TestSbRenderReadsSettingsAndBrain`, `TestSbRenderReadsPersonalSettings`. |
| 3. `go test ./internal/job/... ./internal/settings/... -count=1` | HOLDS | exit 0: job, job/frontmatter, job/jobcmd, job/lint, settings all `ok` |
| 4. App `make realplugin` for the three tests | HOLDS | Built with `make build` -> dist/grid in this clone (`/srv/gridwork/build/C-sb3-render-skill-state-realplugin/dist/grid`). `make realplugin ... RUN=TestRealPluginJobGivesTheClaudePrompt PKGS=./internal/kernel/worker` -> `realplugin: pass 1 fail 0 skip 0`, exit 0. With `RUN='TestRealPluginFwReads\|TestRealPluginHqJourneyWordsToResult'` -> `realplugin: pass 10 fail 0 skip 0`, exit 0. The `make realplugin` summary does not print `--- PASS` lines, so I reran through `make test GRID_PLUGIN_BIN=... "GOFLAGS=-count=1 -v -run=..."` in the scratch app clone: `--- PASS: TestRealPluginJobGivesTheClaudePrompt (0.95s)`, `--- PASS: TestRealPluginFwReads (1.17s)`, `--- PASS: TestRealPluginHqJourneyWordsToResult (31.33s)`, `ok .../internal/kernel/worker 33.566s`. Before the fix the first one failed: `agentbuild_real_unix_test.go:135: grid [job render T-01 J-1 --json]: exit status 1`. |
| 5. `make lint`, `make cross` | HOLDS | `make lint` (go vet ./...) exit 0; `make cross` exit 0 (Windows and macOS builds); `gofmt -l internal/settings internal/job/jobcmd` prints nothing |
| Packages importing settings | HOLDS | `go list` importers: account, brain, cli, doctor, guard, hook, job/jobcmd, settings, updates. `go test -count=1` on all of them: ok (updates has no test files) |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. App walk tests do not apply: no app file changed. |
| No existing test edited or deleted | HOLDS | `git diff origin/lane-C-sb2-skill-state-reader --stat`: skill_state.go changed, one new test file |
| No guard, gate, cap, hold or blocking hook | HOLDS | none added |
| Scratch clone `/srv/gridwork/tmp/sb3-app` removed | NOT DONE | `rm` is outside my allow list and was refused. Orchestrator: `rm -rf /srv/gridwork/tmp/sb3-app`. Nothing in it was edited. |

## What changed

- `internal/settings/skill_state.go`: `ReadSkillsOff` now reads only the `skills` group of a settings file (`readSkillGroup`). A settings file that is missing, has other writers' keys, is not valid JSON or has a bad `skills` group turns nothing off at that level. A project record that cannot be loaded or a symlinked project folder falls back to the personal level. Only an empty home is still an error. A home that turns a skill off, at the personal or the project level, behaves as in C-sb2.
- `internal/job/jobcmd/sb3_render_skill_state_test.go`: three tests (app keys, unreadable file, personal off still left out).

Note: a skill named in `skills` with an off value is honoured even if it is not in the catalog (the strict decoder refused such names before); it only affects names no Job asks for.

## NOT VERIFIED on a live screen

None; no UI change.
