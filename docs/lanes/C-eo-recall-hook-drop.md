# C-eo-recall-hook-drop: proof table

Base: `bb3c1340` (the brief names d528178a; the lines it cites held on this base).
Verdict v-6 (simplify.md:57 row 6): brain entries reach agents as skills, rules and Job pointers; no hook injects recall or topic text.

| Row | Result |
| --- | --- |
| 1 Deleted functions and cases, with file:line on base | HOLDS, list below |
| 2 PreToolUse Agent grid-planner on a task with a brain note: no "Brain recall", no `brain recall` history line | HOLDS: `TestPlannerDispatchWritesNoBrainRecall` (internal/hook/dy_recall_hook_test.go). On base it failed at the SubagentStart that follows the Agent call: `hook output holds "Brain pointers": {"hookSpecificOutput":{"additionalContext":"Brain pointers for this task; open only notes that apply:\n- brain:home/cache ...`. The PreToolUse Agent call itself only recorded `PlannerTask` on base (brain.go:280) and wrote no text, so the brief's "fails on base" holds for the dispatch sequence, not the Agent call alone. |
| 3 No topic or recall text on Skill with topics, Read under knowledge/<topic>/, SubagentStart, UserPromptSubmit; exit 0 | HOLDS: `TestNoTopicOrRecallTextFromAnyHook`. On base: `PreToolUse Skill: hook output holds "Brain topic": {"hookSpecificOutput":{"additionalContext":"Brain topic fixture-topic: Fixture summary.\n- docs/a.md: why a matters here","hookEventName":"PreToolUse"}}`. The other hooks on the event (history, session) still run: exit 0, empty stderr, session activity readable. Heartbeat acts only on PostToolUse, Stop and PreCompact (events.go:239), so it never ran on these two events. |
| 4 settings.json with the three names at "block" loads, entries ignored, `levels` lists none | HOLDS: `TestRetiredRecallHookNamesAreIgnored` (Effective, ReadHookLevels and HookNames, the sources `grid hook levels --json` reads at cli/settings.go:110-122; SetHook refuses the names). On base: `topic-trigger still in the effective hooks: map[... subagent-start:block topic-trigger:block ... user-prompt:block]`. The `hooks.json` golden in internal/cli passes without the three keys. |
| 5 `git grep -n -E "topicContext\|skillTopics\|pathTopics\|brain-recall\|func recall\(" -- internal/hook internal/settings internal/golden docs/settings.md` | HOLDS: one hit, docs/settings.md:137, the line the brief assigns to C-eo-recall-guard-drop. |
| 6a `git diff` files | NOT as the brief states: three files outside Touches changed, see "Outside Touches". |
| 6b `go test -count=1 ./internal/hook/... ./internal/settings/... ./internal/cli/... ./internal/golden/...` | HOLDS: exit 0, four ok. |
| 6c Importers of hook and settings (hook: cli, packaging; settings: cli, doctor, guard, hook), from `go list`: `go test -count=1 ./internal/packaging/... ./internal/doctor/... ./internal/guard/...` | HOLDS: exit 0, four ok. |
| 6d `make lint` | HOLDS: exit 0 (after `gofmt -w internal/hook/brain.go`). |
| 6e `GOOS=windows go vet ./...` | HOLDS: exit 0. |
| 6f macOS: `GOOS=darwin CGO_ENABLED=0 go build ./...` | HOLDS: exit 0. |
| Rule: small commits, one-sentence messages, no attribution | HOLDS: three commits, none with an attribution line. |
| Rule: new tests fail on base | HOLDS: all three tests failed on `bb3c1340` with the lines above before the deletions (commit order: tests first). |
| Rule: no existing test edited to pass | HOLDS: every edited test lost only recall behaviour; each is named below. |
| Rule: no guard, gate, cap, hold or hook added | HOLDS: only deletions plus the new test file. |
| Rule: no personal paths, hosts or emails | HOLDS: none added. |

## Deleted, with base file:line and the test that still covers the hook it sat in

- `recall` internal/hook/brain.go:262-398, `namedTaskDir` :227-260, `namedTask` :225 (only recall used the last two). `brainRead`, `brainShow`, `shellWords`, `parseBrainShow`, `recordBrainRead`, `brainStore` stay: all still used (brain.go read paths, `brainStore` at brain.go:56 and :115).
- `topicContext` brain_topic.go:22, `skillTopics` :80, `pathTopics` :121: the file is deleted.
- hook.go:296 `recall(o, *in)` and :297-299 `decision.Context = topicContext(...)`; events.go:71 component entry, :113-114 case, :122 default call.
- Tests in brain_topic_test.go (deleted): `TestPreToolUseInjectsTopicFromSkillFrontMatter` :69, `TestPreToolUseInjectsTopicFromToolPathOncePerSession` :87, `TestShippedSkillsNameShippedTopics` :101. Behaviour removed by verdict v-6; the absence is covered by `TestNoTopicOrRecallTextFromAnyHook`.
- levels_test.go `TestCatalogContextLevels` rows user-prompt :71 and subagent-start :72: the session-start row stays and covers the context level; the retired names by `TestRetiredRecallHookNamesAreIgnored`.
- levels_test.go `TestHistoryOffCoversEveryWriter` the brain-recall run :221-224 and the "brain recall" word :232: file written, subagent finished and brain read stay in the same test.
- levels_test.go `TestContextComponentLevels` :274-301 (user-prompt and subagent-start only): removed with their switches; `TestCatalogContextLevels` session-start covers context levels.
- records_test.go:46 "brain-recall" removed from the event list: the other component events stay in it.
- corpus_test.go:48 "brain-recall" removed from the components map.
- Golden cases golden/brain-recall/{no-hits,accepted,duplicate,planner-dispatch}, golden/INDEX.md:90-93, golden/known-differences.txt:40-41. golden/INDEX.md:162 (the old script hash) is kept. Hook behaviour is covered by the new tests above; brain-read golden cases stay.
- catalog.go:30 `subagent-start`, `topic-trigger`, `user-prompt` moved to `retiredHooks` (:35); the three keys are dropped from internal/cli/testdata/settings/hooks.json.
- docs/settings.md rows for subagent-start and user-prompt, the topic-trigger words, "brain-recall history entries" and "recall context continues"; a sentence names the three retired names.

## Outside Touches (needs the owner's or the reviewer's say)

- `internal/hook/lockorder_test.go`: the `brain recall` subtest of `TestHookAndAttachLockOrderFinishes` (it asserted a `brain recall` history line from UserPromptSubmit, now gone). The bash history and brain read subtests still cover the lock order.
- `internal/hook/events_test.go`: `TestRecallPointersReadDedupeAndPlanChange`: the recall half (UserPromptSubmit pointers, dedupe, plan change) removed; the brain-read dedupe half stays and passes.
- `internal/hook/dy_recall_hook_test.go` is the new file named in Touches.
- Neither test file is listed in Touches; both asserted recall output and would fail otherwise.

## What changed

PreToolUse, SubagentStart, UserPromptSubmit and the plan-skill PostToolUse no longer inject brain text; the `brain-recall` component entry is gone; three settings names are retired.

## Open items

- `session.Activity.PlannerTask` (internal/session/activity.go:29) is now never set or read. It is outside Touches and was left.
- `brain.Store.TopicBriefAt` in internal/brain has no caller in internal/hook now; internal/brain is Not touched.
- A stray untracked `.brain_new.go` in the clone root (not committed; Go ignores it; I could not remove it).
- docs/settings.md:137 still says "brain-recall check on plan.md": C-eo-recall-guard-drop's line.

## NOT VERIFIED on a live screen

Nothing here is UI. A live Claude Code session was not run: the hooks are proved with the Go hook entry point and real files only.
