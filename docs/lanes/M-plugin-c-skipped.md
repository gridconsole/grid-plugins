# M-plugin-c-skipped

Merges C-need-planguard-warn and C-run-render-intent-once onto main 3a3d99ea. Batch Pint56 skipped both on conflicts.

## Proof table

| Row | Result |
|---|---|
| 1. Merge `origin/lane-C-need-planguard-warn` | HOLDS. One conflicted file, see below. Merge commit 95bb5fc8. |
| 1. Merge `origin/lane-C-run-render-intent-once` | HOLDS. One conflicted file, see below. Merge commit a4d18413. |
| 2. `git log --oneline main..HEAD` has both merges | HOLDS (log below). |
| 3. `go test -count=1 ./...` | HOLDS. Every package ok; filtered output has 0 non-ok lines. |
| 3. `make lint` | HOLDS, exit 0. |
| 3. `GOOS=windows go vet ./...` | HOLDS, exit 0. |
| 3. `GOOS=darwin go vet ./...` | HOLDS, exit 0. |
| 4. Test or golden changed | HOLDS. No golden regenerated. One new-lane test line changed, see below. |
| No guard, gate, cap on agents | HOLDS. The plan-guard lane lowers a default to warn; this lane adds none. |
| No push | HOLDS. A bundle only. |

Packages importing `internal/job` and `internal/settings` ran inside the whole-module test (`contract/formats` and `internal/job` first failed, then passed).

## Conflicts

### C-need-planguard-warn

- `internal/cli/testdata/settings/hooks.json`: the lane sets `plan-guard` to `warn` and still listed `precompact`. Main commit 00d85d46 ("Stop the PreCompact hook from steering the summary; it only records") retired `precompact` in `internal/settings/catalog.go`. Resolved by keeping both: `plan-guard` is `warn` (lane) and `precompact` is absent (main). Every other key matches main. The catalog in the merge result agrees: no `precompact` in `hooks`, and it is in `retiredHooks`.

### C-run-render-intent-once

- `internal/job/render.go` (Intent section): main commit 6f9902d0 ("Save how the brief used each source...") routes each pasted source through `paste(label, source)`, which records the `shown` mode and the pasted-character count. The lane replaced the three writes with a loop that prints a source once under joined slot labels. Resolved by keeping the lane's loop and having it call `paste(label, sl.source)`. A duplicated source is counted once in `pasted_chars` and `shown`.

## Follow-on fixes (not conflicts; main made them necessary)

- `internal/job/render.go:378`: main's `shown` map is keyed by `s.Pointer`, the lane's `seen` map by `SourceKey`. The end loop read `shown[k]`, so any source whose key differs from its pointer (`task:research.md#format`) lost its `shown` field. `TestBriefGolden` failed (`job-inputs differs from the golden file`, the `shown: summary` line missing) and so did `contract/formats` `TestFormatSavedRenderWriter` (`full.json differs from the writer capture`). Changed to `shown[s.Pointer]`. Both pass again with the goldens untouched.
- `internal/job/render_intent_once_test.go:48`: the new lane test expected the bare read line `- Read task:intent.md#request [person-team]`. Main commit cc388991 ("Render each read line with the file's summary and the reason its link gives") prints the file's summary, so the line is `- Read task:intent.md#request · Request {#request} [person-team]`. Updated the expectation. The failing line before the fix was `render_intent_once_test.go:49: the Read line for the second spelling is gone`. The test still checks that the duplicate read line is kept.

## git log --oneline main..HEAD

```
e9747234 Key the shown mode by the kept pointer and update the intent-once Read line to main's summary format.
a4d18413 Merge remote-tracking branch 'origin/lane-C-run-render-intent-once' into lane-M-plugin-c-skipped
95bb5fc8 Merge remote-tracking branch 'origin/lane-C-need-planguard-warn' into lane-M-plugin-c-skipped
a2dbaf56 Show a source once in a rendered brief whatever spelling or intent slot points at it.
8adcead8 Add the lane proof table for the plan guard warn change.
d2f0eebd The plan guard warns by default and its two approval-line denies are removed.
```

## NOT VERIFIED on a live screen

Nothing here touches a UI. Nothing was run live.
