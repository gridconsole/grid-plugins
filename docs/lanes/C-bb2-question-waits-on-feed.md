# C-bb2-question-waits-on-feed: lane report

Repo: plugin. Base: main c7c10553. Branch: lane-C-bb2-question-waits-on-feed.

A question asked with `--waits-on 'Not yet'` now carries `waits_on` on its Feed question post. An answer equal to one of those labels is posted as a leaseless person control `question_event: "waiting"`. The fold keeps that control in the history without changing State or Answer. The question stays open, its person Job is not closed, and a later "Done, carry on" answers and closes it as before. A question asked without the flag behaves exactly as on base, and its file has no `waits_on` field.

## Base check (base c7c10553, in-process)

The new tests were run on base before any production edit.
- `TestBbWaitNoFlagAsBase` PASSES on base. That is the base check: ask with options 'Done, carry on' / 'Not yet', then answer 'Not yet'. The result is State answered, Answer 'Not yet', and closes `[{J-7 Not yet}]`. A later 'Done, carry on' is refused with exit 1 and `Q-1 is answered`.
- The other four fail on base with:

```
=== RUN   TestBbWaitNotYetKeepsOpen
    bb_waits_on_test.go:81: ask --waits-on: exit 2: flag provided but not defined: -waits-on
--- FAIL: TestBbWaitNotYetKeepsOpen (0.04s)
    --- FAIL: TestBbWaitThenDone/owner (0.00s)
    --- FAIL: TestBbWaitThenDone/guest_addressed (0.00s)
--- FAIL: TestBbWaitRefusesUnknownLabel (0.00s)
--- PASS: TestBbWaitNoFlagAsBase (0.11s)
--- FAIL: TestBbWaitFeedFold (0.00s)
FAIL	github.com/gridconsole/grid-plugins/internal/question	0.237s
```

## Proof table

| Row | Status | Proof |
|-----|--------|-------|
| C-bb 1 TestBbWaitNotYetKeepsOpen | HOLDS | `go test -count=1 -run 'TestBbWait' -v ./internal/question/` exit 0: `--- PASS: TestBbWaitNotYetKeepsOpen`. After two 'Not yet' answers and a `Sync` rebuild from the Feed: State open, Answer "", two `waiting` events with text, author, post and time, `waits_on` ["Not yet"] read back from the Feed, `calls.closes` empty, the question post still in `NeedsYou`, and Q-1.md holds `"waits_on"` and `"kind": "waiting"`. Base failing line above. |
| C-bb 2 TestBbWaitThenDone (+ guest) | HOLDS | Same run: `--- PASS: TestBbWaitThenDone/owner`, `--- PASS: TestBbWaitThenDone/guest_addressed`. Owner: 'Not yet' then 'Done, carry on' gives State answered, history waiting then answered, closes exactly `[{J-7 Done, carry on}]`, and a later 'Not yet' is refused with `Q-1 is answered`. Guest: the guest rule at internal/question/question.go:259 refuses an unaddressed guest; the addressed guest's 'Not yet' keeps the question open with no Close, and their 'Done, carry on' closes it once. |
| C-bb 3 TestBbWaitRefusesUnknownLabel, TestBbWaitNoFlagAsBase | HOLDS | Same run: both PASS. `--waits-on Later` exits 2 with one stderr line `question: waits-on "Later" must name a distinct option` (question.go:130). No Q-1.md is written, there is no question post and no Job post. With no flag, 'Not yet' answers and closes as on base, and Q-1.md has no `waits_on`. `go test -count=1 ./internal/question/` exit 0, existing tests unchanged. |
| C-bb 4 contract | HOLDS | `go test -count=1 -v ./internal/contract/... ./internal/cli/ -run 'TestContractCommands\|Schema\|Fixture'` exit 0. `TestFixtures` (internal/contract/schema_test.go:21) loads contract/fixtures/question-file: `--- PASS: TestFixtures/question-file/valid/waiting.json`, `.../invalid/waits-on-empty.json`, plus control `valid/question-waiting.json`, `invalid/question-waiting-leased.json` and question `valid/waits-on.json`, `invalid/waits-on-empty-label.json`. `--- PASS: TestContractCommandsInventory`, `--- PASS: TestContractCommandsParserInventory`, `--- PASS: TestPersonWithdrawControlSchema`. |
| C-bb 5 tests, lint, cross | HOLDS | `go test -count=1 ./internal/question/ ./internal/cli/` exit 0; `make lint` exit 0; `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| C-bb 6 walk rows and grep | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (both PASS); `go test -count=1 ./internal/job/lint/` exit 0. `git grep -n -- '--waits-on' agent-templates/grid-planner.md skills/questions/SKILL.md` prints agent-templates/grid-planner.md:59 and skills/questions/SKILL.md:17 (the ask line is :17 on base too, not :16). |
| C-bb Touches | HOLDS | The diff is C-bb's Touches plus the four files of bb2 and their goldens and fixtures. C-bb's files: question.go, job.go, command.go, question-file.json and its fixtures, commands.json, grid-planner.md:59, SKILL.md, bb_waits_on_test.go. bb2's: question.json, control.json, internal/feed/questions.go, internal/question/feed.go. Fixtures: contract/fixtures/{control,question,question-file}. Golden: testdata/golden/mcp-session.golden, the contract golden these schemas feed. `TestSessionGolden` embeds the schemas and was regenerated with `make test UPDATE_GOLDEN=1 "GOFLAGS=-count=1 -run=^TestSessionGolden$"`; its word diff is only the new schema text. No golden records `question ask` help. internal/cli/inprocess.go is unchanged. |
| bb2: `go test -count=1 ./internal/feed/... ./internal/question/... ./contract/...` | HOLDS | exit 0 (feed, question, contract, contract/formats ok). |
| bb2 Do 2: app copy of the schemas | HOLDS, no app brief needed | App main has no copy of control.json or question.json: `git ls-tree -r main` finds no schemas, and `git grep question_event main` finds no Go reader. The app reads question files with plain `json.Unmarshal` into `questionfile.Question` (app internal/kernel/questionfile/questionfile.go:17-35), which ignores the unknown `waits_on`. History `Kind` is a free string (:38-43). The state switch (:80-84) accepts "open", which is what a waiting question keeps. |
| Rule: importers tested | HOLDS | `go list` importers of internal/feed, internal/question, internal/contract: contract, contract/formats, approvefix, artifact, brain, braincmd, cli, contract, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, ports, question, session, settings, signal, task, updates, usage, where, workflow, lib. contract reaches most of the module, so the whole module was run: `go test -count=1 ./...` gave 69 packages ok, 0 failing. |
| Rule: walk tests | HOLDS | See C-bb 6 (plugin set). |
| Rule: RealPlugin tests | HOLDS (none apply) | `git grep -l 'func Test[A-Za-z]*RealPlugin'` in the plugin finds none. No app test was run. |
| Rule: no guard, gate, cap or hold | HOLDS | `--waits-on` only names which answers keep a question waiting. The deadline default is unchanged: the fold still defaults an open question at its deadline (internal/feed/questions.go:156), and Tick still posts the default, so a waiting question never holds past its default. A waiting control posted at or after the deadline is treated like a late withdrawal (questions.go:121). |
| Rule: new tests red on base | HOLDS | Base failing lines are pasted above. All tests are in the one new file, internal/question/bb_waits_on_test.go. No existing test was edited or deleted. |
| Rule: commits | HOLDS | Small one-sentence commits with no attribution lines. |

## What changed

- contract/schemas/question.json: a `waits_on` body property (non-empty strings, at least 1, unique).
- contract/schemas/control.json: `waiting` added to the `question_event` enum (:222). The person variant (lease null, gen 0, fence 0) admits `withdrawn` or `waiting`. The task-lease variant refuses `waiting` through a `body.not` clause, so a waiting control is always a person's.
- contract/schemas/question-file.json: a `waits_on` property and the `waiting` history kind.
- Six fixtures: one valid and one invalid each for control, question and question-file.
- internal/feed/questions.go: `personWaiting` admits owner, member or lead, or a guest whose id is the question's `who`, as `PersonAnswer` does. It requires nil lease and fence/gen 0, and does not use the withdraw policy. The fold keeps such a control in `d.Events` only while State is open and only when its text is one of the question post's `waits_on` labels (:145-150). State and Answer are unchanged.
- internal/question: `Record.WaitsOn` and `Input.WaitsOn` (omitempty). `validateInput` checks every label names a distinct option. `Answer` records a label answer as `waiting`; on a non-open question it is refused with `Q-n is <state>`. `publishQuestion` posts `waits_on`. `publishEvents` posts `waiting` with the person writer and no lease (feed.go:87), as an answer is posted. `settleJob` skips `waiting` explicitly (job.go:119).
- `--waits-on` (repeatable) is on the shared add/ask flag set, so commands.json lists it under both `question add` and `question ask`; the parser inventory test requires both.
- Docs: grid-planner.md:59 and SKILL.md:17 name the flag.

## Open items

- The app task view: the app reads the waiting question as open (questionfile.go:80-84), so it is not hidden. Whether the task view shows the `waiting` history entry ("Not yet, <time>") is NOT VERIFIED on a live screen. `QuestionEvent.Kind` is read, but I did not check how it is rendered.
- contract/README.md:296 documents only the person `withdrawn` control. One line for `waiting` would help readers, but the README is outside Touches and is left for the merger or a follow-up.
- `validateRecord` (internal/question/files.go, outside Touches) ignores `waiting` events and does not re-check `waits_on` labels. The fold already admits only label texts while open, so a record that breaks this cannot reach a file. Noted, not changed.
- Data format review: one new optional field on the question post and the question file, and one new person control kind `waiting` on the Feed contract.

## NOT VERIFIED

- App task view rendering of a `waiting` history entry (live screen).

DONE
