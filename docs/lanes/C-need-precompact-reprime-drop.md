# C-need-precompact-reprime-drop

PreCompact hook no longer prints summary guidance; it only records.

## Proof table

| Row | Result |
|---|---|
| 1. grep precompact in internal, docs/settings.md | HOLDS. Lane: events.go:41, :96, :239 (records), catalog.go:35 (`retiredHooks`), records_test.go:44, plus the new precompact_records_test.go (untracked by git grep until committed). Main also had events.go:118, docs/settings.md:121, catalog.go:30, events_test.go:232, levels_test.go:71, records_test.go:131, testdata/settings/hooks.json:1. No guidance text, no `hookEnabled("precompact"`. (events.go:242 on main is :239 now.) |
| 2. PreCompact test: exit 0, no guidance, record still written | HOLDS. TestPreCompactOnlyRecords. On main: `precompact_records_test.go:34: precompact printed guidance: "Keep the Job id, acceptance ids and open decisions in the summary.\n"`. No other record prints on PreCompact. Passes on lane. |
| 3. old settings naming precompact warn/block loads, key ignored; no row in `grid hook level` | HOLDS. TestRetiredPreCompactKeyIsIgnored (passes on lane; with the name back in `hooks` it fails: `retired hook still listed: map[... precompact:warn ...]`). Golden internal/cli/testdata/settings/hooks.json has no precompact key and the cli tests pass. |
| 4. Deleted tests | TestPreCompactSteersSummary and the levels_test.go:71 row removed; records are covered by TestPreCompactOnlyRecords and TestRecordEventsNeverBlockOrDeny (records_test.go:44); the guidance was removed per simplify.md:57. |
| 5. go test hook, settings, cli; make lint | HOLDS. `go test ./internal/cli/... ./internal/doctor/... ./internal/guard/... ./internal/hook/... ./internal/packaging/... ./internal/settings/...` exit 0 (importers by go list: cli, doctor, guard, hook, packaging, settings). `make lint` exit 0. |
| Windows/macOS builds | HOLDS. `GOOS=windows go vet` and `GOOS=darwin go vet` on hook, settings, cli: exit 0. |
| New tests fail on base | HOLDS, lines above. |
| No guard/gate/cap added | HOLDS; only removals and a retired-name entry. |
| No attribution lines | HOLDS. |

## What changed
events.go drops the PreCompact guidance print; catalog.go moves "precompact" from `hooks` to `retiredHooks`; docs/settings.md row and the golden key removed; two tests deleted, one comment reworded; new internal/hook/precompact_records_test.go.

## NOT VERIFIED on a live screen
Nothing here needs a live screen; no live compaction was run.
