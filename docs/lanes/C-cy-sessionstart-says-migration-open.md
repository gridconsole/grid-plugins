# C-cy-sessionstart-says-migration-open

Base: plugin main 952f3413. At cli.go:223-230 the refusal branch returned 0 with nothing printed for every `grid hook` (`return 0 // a hook never speaks into a session`), so the base check does not stop the lane.

## Proof table

| Row | Result |
|---|---|
| 1 red on base | HOLDS. Base, new test only: `go test -count=1 -v -run TestCySess ./internal/cli/` printed `grid hook sessionstart: exit 0, stdout "", stderr ""` and `stdout "" is not one JSON object: EOF`; same for `SessionStart`; `--- FAIL: TestCySessSessionStartSaysMigrationOpen`. |
| 1 green after | HOLDS. Same command exits 0: `grid hook sessionstart: exit 0, stdout "{\"hookSpecificOutput\":{\"additionalContext\":\"A migration of ~/.absent stopped part way. Run: grid migrate --apply\",\"hookEventName\":\"SessionStart\"}}\n", stderr ""`; `--- PASS: TestCySessSessionStartSaysMigrationOpen/sessionstart` and `/SessionStart`. The test decodes with unknown fields refused, checks one value, hookEventName, additionalContext == vrPendWant, empty stderr, and vrPendHomeAbsent. |
| 2 other events silent | HOLDS. `--- PASS: TestCySessOtherEventsStaySilent` with subtests stop, pretooluse, posttooluse (exit 0, no stdout, no stderr, no home). It passed on base too, as it pins what stays unchanged. |
| 3 other refusals silent | HOLDS. `go test -count=1 -v -run 'TestFormatVersionCheck' ./internal/cli/` exit 0, `--- PASS: TestFormatVersionCheck/hook_stays_silent`. The branch also requires `migrate.Pending`, so VERSION 99 stays silent. |
| 4 no journal unchanged | HOLDS. `-run 'TestVrPend|TestHookSessionStartGolden'` exit 0: `--- PASS: TestVrPendGridLeavesTheHomeAloneWhileAJournalIsOpen`, `--- PASS: TestVrPendNoJournalLeavesTheCommandsAsTheyWere`, `--- PASS: TestHookSessionStartGolden`. `git diff origin/main --stat` lists no file under internal/golden or testdata. |
| 5 TestCySess | HOLDS. Both PASS lines as in rows 1 and 2. |
| 6 package tests | HOLDS. Importers of internal/cli by `go list`: cmd/grid, lib. `go test -count=1 ./internal/cli/ ./internal/hook/ ./internal/migrate/ ./internal/job/lint/ ./cmd/grid/ ./lib/` exit 0 (cli 52.9s ok, hook ok, migrate ok, job/lint ok, lib ok, cmd/grid no test files). |
| 6 walk tests | HOLDS. `go test -count=1 -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList' ./internal/cli/` exit 0; `internal/job/lint` ok (above). |
| 6 lint | HOLDS. `make lint` (go vet ./...) exit 0. |
| 6 cross | HOLDS. `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test compile, all ok. |
| Main merge | HOLDS. `git fetch origin && git merge origin/main`: Already up to date; the walk tests ran after it. |
| Existing test edited | The brief directs the change to the subtest at vr_migrate_pending_test.go:55-63: it is renamed and now expects the JSON line, and the "makes no home" half stays. No other test edited, none deleted. |
| App guest walk tables (`make realplugin`) | not run, the brief names no app change and this is the plugin repo. |
| Plugin build for RealPlugin tests | not run, no row names a RealPlugin test. |

## What changed

- internal/cli/cli.go (refusal branch): for `grid hook` with exactly one arg equal to `SessionStart` (strings.EqualFold) and `migrate.Pending(env.GridHome)`, print the refusal message as SessionStart `additionalContext` JSON (the hook.go:145-147 keys) and exit 0. Every other event and refusal prints nothing.
- internal/cli/vr_migrate_pending_test.go: the hook subtest expects the JSON line and still no home.
- contract/README.md:445: one clause for the exception.
- internal/cli/cy_sessionstart_migration_test.go: the two new tests.

## NOT VERIFIED on a live screen

A real Claude Code session start showing the line as context was not run. The JSON shape is the one hook.go already emits for the same event.

## Verdict

DONE
