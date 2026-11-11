# C-qc-printed-commands-exist

Base: plugin 2b47f621 (lane-Pint134 merge; `git grep -n 'grid brain topic %s' -- internal/job/render.go` hit at :237, `git grep -n 'lease take-over' -- internal/cli/task.go` hit at :312 on it, so neither was gone). Commits: 06a45de8 (banner), e35eb905 (advice and heMvProse), dd27a92f (docs). Commit identity: Grid lane, no attribution lines.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. Red on base, green on lane | HOLDS | With only the three lines at he_mv_message_verbs_test.go:50-52 deleted, on base code: `go test -count=1 -v -run TestHeMvMessagesNameRealSubs ./internal/contract/` exit 1: `he_mv_message_verbs_test.go:207: internal/cli/task.go:312: `grid lease take-over`: "take-over" is not a sub of `grid lease` ...` and `...:207: internal/job/render.go:237: `grid brain topic`: "topic" is not a sub of `grid brain` ...` then `--- FAIL: TestHeMvMessagesNameRealSubs`. On the lane: `--- PASS: TestHeMvMessagesNameRealSubs`, `--- PASS: TestHeMvProseListBothWays`. `git diff main -- internal/contract/he_mv_message_verbs_test.go` shows three deleted lines, no added line. |
| 2. TestQcPcCutBannerNamesShow | HOLDS | internal/job/qc_cut_banner_test.go. Lane: `go test -count=1 -v -run TestQcPc ./internal/job/` prints `--- PASS: TestQcPcCutBannerNamesShow`. Against base render.go (stashed my render.go change): exit 1, `qc_cut_banner_test.go:38: brief lacks "1 entries cut (inputs.json not_rendered); `grid brain show <id>` reads one"` and `:41: brief names `grid brain topic`, which no dispatcher has`, `--- FAIL: TestQcPcCutBannerNamesShow`. |
| 3. Fixtures and pins | HOLDS | elided.json recaptured: `go test -count=1 ./contract/formats/ -run TestFormatSavedRenderWriter -update-format-fixtures` exit 0, and `go test -count=1 ./contract/... ./internal/job/... ./internal/contract/` exit 0 (no flag). written-by-main.json edited by hand (contract/fixtures/fmt-job-render/valid/written-by-main.json:2, `<`/`>` escapes as the file already uses). render_context_test.go:109 wants the new text. No cut set in the render_context_test.go table (:98-101) moved: the new banner is shorter, every budget case still passes unchanged. render_test.go:74 wants `1 entries cut (inputs.json not_rendered);`. |
| 4. Printed commands in Go | HOLDS | `git grep -n 'lease take-over' -- '*.go' ':!internal/cli/liveness_test.go'` prints nothing; `git grep -n 'grid lease take <task> --takeover' -- internal/cli/task_test.go` hits :457; `... --expected-epoch' -- internal/cli/task.go` hits :312; `git grep -n 'grid brain topic ' -- '*.go' ':!*_test.go'` prints nothing. liveness_test.go untouched. |
| 5. Docs | HOLDS | `git grep -n PreToolUse -- topics skills` prints nothing; SessionStart hits topics/README.md:9 and skills/brain-keeper/SKILL.md:32; `hooked, not dumped (owner decision)` still at SKILL.md:32; `git diff main -- skills/run-job skills/review-job` is empty (0 lines). `go test -count=1 -v -run TestSkillCommandsImplemented ./internal/packaging/` prints `--- PASS: TestSkillCommandsImplemented`. I also dropped the stale `:111` from the hook.go pointer in SKILL.md:32 (line 111 of hook.go is not the index). |
| 6. Package tests, lint, cross, stat | HOLDS | `go test -count=1 ./internal/job/... ./internal/braincmd/ ./internal/cli/ ./internal/packaging/ ./internal/contract/ ./contract/...` all ok. `TestHsIpWalksEveryCommand` PASS. `make lint` exit 0. `make cross` exit 0 (Windows and macOS builds). `git diff main --stat` names only Touches files (plus this report). |
| Importers of changed packages | HOLDS | `go list` of packages importing internal/job or internal/cli: cmd/grid, internal/hook, internal/job/jobcmd, internal/job/lint, internal/jobgraph, internal/lease, internal/migrate, internal/orchestrator, internal/ports, lib. `go test -count=1` on them: all ok (cmd/grid has no test files). |
| Walk tests (plugin) | HOLDS | `go test -count=1 -v -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' ./internal/cli` `--- PASS` for both; `go test -count=1 ./internal/job/lint/` ok. The app-only walks do not apply to the plugin repo. |
| New tests fail on base, pass on lane | HOLDS | Rows 1 and 2. |
| No existing test edited to pass | HOLDS | Edits to existing tests are the five pins the brief names (task_test.go:457, render_test.go:74, render_context_test.go:109, he_mv heMvProse three lines). No test deleted. |
| No guard, gate, cap or hold added | HOLDS | Only two message strings, dead topic collection and docs changed. |
| RealPlugin tests | HOLDS (none named) | No row names a *RealPlugin* test. |
| Commit rules | HOLDS | Three one-sentence commits, no attribution lines (`git log` shows 0 Co-Authored-By), no personal paths, hosts or emails in the diff. |

## What changed

- internal/job/render.go: the banner is `%d entries cut (inputs.json not_rendered); \`grid brain show <id>\` reads one`; the topic collection and the `topicName` var are gone, and so is the then-unused `regexp` import.
- internal/cli/task.go: the advice names `grid lease take <task> --takeover --expected-epoch <gen.fence>`.
- he_mv_message_verbs_test.go: the two excused entries and their comment are removed.
- Pins: the five test and fixture lines named above; new internal/job/qc_cut_banner_test.go.
- topics/README.md and skills/brain-keeper/SKILL.md say how topics reach agents now.

## NOT VERIFIED on a live screen

Nothing here is a screen. The attach advice was checked only through its unit test, not a live `grid task attach` against a held lease.
