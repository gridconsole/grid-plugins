# C-run-render-intent-once

Base: plugin main 78b08bc9 (lane branch cut from it).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 New tests fail on main, pass on lane | HOLDS | On main (with only an identity `SourceKey` stub so the file compiles) `go test -count=1 -run 'Request\|Spellings\|DifferentAnchors\|PartlyEqual\|NonTask\|GenerationIsStable\|ReplayResolves' ./internal/job/` failed: `render_intent_once_test.go:37: request text appears 3 times, want 1`; `:57: 2 intent sources in inputs.json, want 1` (pointers `intent.md#request` and `task:intent.md#request`, same sha256); `:69: 4 sources, want 3`; `:100: no Owner, Goal header`. On the lane the same command passes. |
| 2 Key from pointer in render.go | HOLDS | `SourceKey` in internal/job/render.go drops a leading `task:`, writes `:N-M` as `#LN-LM`, and keeps `brain:`/`repo:`/`prototype:`/`job:` text. `seen` uses it (`resolve`). The kept entry's `pointer` is the first spelling (`TestRequestIsOneSourceInInputs`, `TestSpellingsOfOneRangeAreOneSource`). Different ranges or anchors stay separate (`TestDifferentAnchorsStayDifferent`). `TestNonTaskPointersKeepTheirText` covers the key. |
| 3 Intent prints once, slots joined | HOLDS | `### Owner, Item, Goal [person-team] (intent.md#request)` (`TestRequestShownOnceWhenAllIntentSlotsAgree`); partly equal: `### Owner, Goal ...` then `### Item ...` (`TestPartlyEqualIntentSlotsJoinOnlyThoseSlots`); three different sources print as before: `TestBriefGolden` passes with testdata/golden unchanged. |
| 4 Context names every read pointer; generation stable | HOLDS | `- Read task:intent.md#request [person-team]` is asserted in the first test; `TestGenerationIsStableAcrossRenders`; `TestReplayResolvesEverySpelling` (replay of a deduplicated inputs.json resolves both spellings; `Replay.Resolve` falls back to the key). |
| 5 `go test -count=1 ./internal/job/... ./internal/cli/ ./internal/golden/ ./contract/...` and `make lint` | HOLDS | go test exit 0, all packages ok; `make lint` (go vet ./...) exit 0. `git diff --stat main` lists only internal/job/render.go plus new files under internal/job and docs/lanes; nothing under testdata/ or contract/. jobcmd/command_test.go passes unchanged. |
| Importers of internal/job tested | HOLDS | `go list` importers: cli, hook, jobcmd, lint, jobgraph, lease, orchestrator, ports. `go test -count=1 ./internal/hook/ ./internal/jobgraph/ ./internal/lease/ ./internal/orchestrator/ ./internal/ports/` exit 0 (cli, jobcmd, lint ran in row 5). |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. `gofmt -l internal/job` empty. |
| No existing test edited or deleted | HOLDS | Only one new test file added. `TestTiersFollowTheSource` indexes the Replay map by raw pointer, so the map keeps raw pointers and `Resolve` falls back to the key. |
| No guard, gate, cap, hold or blocking hook | HOLDS | None added. |
| Commits: one sentence, no attribution lines | HOLDS | `git log main..HEAD`. |
| internal/job/source.go unchanged | HOLDS | Not in the diff. |

## What changed

- internal/job/render.go: exported `SourceKey`; render's `seen` map uses it; the Intent section groups slots by key and prints each source once with the slot names joined; `Replay.Resolve` accepts any spelling of a source it holds.
- internal/job/render_intent_once_test.go: new tests.
- Sources in inputs.json are now sorted by key, not by written pointer; for pointers with no `task:` prefix or `:N-M` range the key equals the pointer, so existing order is unchanged.

## NOT VERIFIED on a live screen

Nothing here is UI. A real run's jobs/J-2/brief.md was not re-rendered (no live run); proven against the test fixture's intent.md#request only.
