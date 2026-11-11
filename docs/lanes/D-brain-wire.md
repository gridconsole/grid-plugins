# D-brain-wire: the brain in the shipped plugin

Base: plugin main eb833a2d. Branch lane-D-brain-wire.

## Proof table

| Row / rule | Result |
|---|---|
| 1. One topic path, documented, other redirected, test loads a topic from it | HOLDS. Path is `topics/<topic>/TOPIC.md` (`internal/brain/topic_text.go:19` TopicFile, read by `topics.go` topicAt and scopeTopics). `knowledge/<topic>/TOPIC.md` is a read-only redirect, used only when the topics/ file is absent (`topic_text.go:24`). Documented in `topics/README.md`. Tests `TestTopicLoadsFromTopicsPath`, `TestTopicsPathWinsOverLegacyPath` (`internal/brain/topic_text_test.go`); the old `TestCustomTopic` still passes through the redirect. `go test ./internal/brain/` exit 0. |
| 2. PreToolUse injects the topic text (skill frontmatter or tool path), short | HOLDS. `internal/hook/brain_topic.go` topicContext, called from `hook.go` toolUse; text rides on the verdict envelope (`hookio.Decision.Context`) so one JSON object is written. Brief = summary + up to 6 pointers, each clipped to 240 chars (`brain.TopicBrief`). Once per session and agent. Tests with a fixture topic: `TestPreToolUseInjectsTopicFromSkillFrontMatter`, `TestPreToolUseInjectsTopicFromToolPathOncePerSession`, `TestTopicBrief`. `go test ./internal/hook/ ./internal/hookio/` exit 0. |
| 3. `grid brain keep` records a proposal; `keep accept\|refuse <id>`; accepted written as linked notes | HOLDS. `keep --name … --link <json>` records a proposed note (links need `why`, a bare link is refused); `keep accept <id>` runs accept (note file in knowledge/<topic>/ with links); `keep refuse <id> [--why]` runs accept --reject (no note). `internal/braincmd/command.go` (keepArgs, `verb = "propose"` switch). Tests: `TestKeepRecordsAProposalWithLinksAndReasons`, `TestKeepAcceptWritesALinkedNote`, `TestKeepRefuseRecordsTheReasonAndWritesNoNote` (`internal/braincmd/keep_test.go`). Bare `keep` still lists (existing tests pass). Documented in `docs/delivery.md`. |
| 4. Two prebuilt topics hooked from skills | HOLDS. `topics/delivering/TOPIC.md` named by `skills/run-job/SKILL.md:4` (`topics: delivering`); `topics/reviewing/TOPIC.md` named by `skills/review-job/SKILL.md:4`. `TestShippedSkillsNameShippedTopics` reads the real repo files and checks the injected brief for both. |
| 5. go test ./... and make lint: 0 fail | HOLDS. `go test ./...` printed no FAIL, exit 0. `make lint` (go vet ./...) exit 0. `gofmt -l internal cmd` empty. |
| New tests fail on base | HOLDS. With topics.go, braincmd/command.go and hook/hook.go from main: `--- FAIL: TestTopicLoadsFromTopicsPath`, `--- FAIL: TestTopicsPathWinsOverLegacyPath`, `keep_test.go:30: keep record exit 2: brain: unknown flag --name` (three Keep tests), `brain_topic_test.go:75: one JSON object expected: unexpected end of JSON input: ""` (three hook tests). Restored afterwards. |
| No existing test edited to pass | HOLDS. No `_test.go` file that existed on main is changed. Two golden/contract data files changed because they record the keep help and keep flags: `testdata/golden/brain-help.golden`, `testdata/golden/brain-keep-help.golden`, `contract/commands.json` (keep gains the propose flags). Hand-edited because the update env vars were refused in this shell; the tests `TestRemainingTaskOperandHelp` and `TestContractCommandsParserInventory` compare them to the parsers and pass. |
| No test deleted | HOLDS. |
| No guard, gate, cap, hold or blocking hook added | HOLDS. The topic text is context only; it never denies or asks. The 6-pointer and 240-char limits shape the injected text, not any agent action. |
| Importers of changed packages tested | HOLDS. Packages changed: brain, braincmd, hook, hookio. Direct importers (go list): braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, packaging, people, routine, task. `go test ./...` covers all of them, exit 0. |
| Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No personal paths, hosts, emails; no attribution lines | HOLDS. Commit messages are one sentence with no trailer. |

## What changed

- `internal/brain/topic_text.go` (new): TopicsDir, TopicFile, redirect reader, TopicBrief, Store.TopicBriefAt (plugin folder, then project, then home).
- `internal/brain/topics.go`: custom topics read from topics/ then the legacy knowledge/ folder; scopeTopics lists both.
- `internal/hook/brain_topic.go` (new) and `hook.go`: PreToolUse topic injection. `internal/hookio/hookio.go`: optional `Context` on a decision (omitted when empty, so existing output is unchanged).
- `internal/braincmd/command.go`: `keep` carries the propose flags, records with --name or --file, and maps `keep accept|refuse`.
- `topics/` (README, delivering, reviewing), two skill front matter lines, `docs/delivery.md`.

## Open items

- The hook test file names fixture skills only by front matter field `topics`; agents (not skills) do not yet name topics. The brief asked for skill frontmatter or tool path only.
- The curator agent and the brain-keeper skill (other lanes) must call `grid brain keep` / `keep accept|refuse`; nothing here launches them.

## NOT VERIFIED on a live screen

- The host's handling of `additionalContext` on a PreToolUse event for a Skill call in a real Claude Code session (tests cover the emitted JSON only).
- The brief text as a live agent reads it.
