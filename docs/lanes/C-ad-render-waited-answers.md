# C-ad-render-waited-answers: `grid job render` gives a Job the answers it waited for

Base: main 2221cb86 (the Render call is at internal/job/jobcmd/command.go:166 on this base, not :164).

## Proof table

| Row | State | Proof |
|---|---|---|
| Red on base | HOLDS | Test-only commit 8d7c4aa4 run on base code: `go test -count=1 -run TestAdRa ./internal/job/jobcmd/` exit 1: `ad_render_answers_cmd_test.go:111: job render lacks "### Answers this Job waited for\n"`, `... lacks "J-2: ada answered at "`, `... lacks "Ship the blue one"`, `job show --json brief lacks "Ship the blue one"`; `TestAdRaQuestionGate ... brief lacks "gate:Q-2: ada answered at ..."`, `... "gate:Q-3: default at 2020-01-02T03:04:05.000Z ..."`, `... "J-5: ada sent back at "`; `TestAdRaGuestAnswerFromFeed ... brief lacks "J-6: gus answered at " or the fence`. internal/job/ad_render_answers_test.go cannot build on base (job.Answer does not exist there). |
| 1 TestAdRaPersonAnswer | HOLDS | `go test -count=1 -run TestAdRa ./internal/job/ ./internal/job/jobcmd/ -v` exit 0, `--- PASS: TestAdRaPersonAnswer`. Covers the saved brief and the `brief` of `grid job show --json` (the field the app reads, agentbuild.go:278). The answered brief has no lint error. |
| 2 TestAdRaQuestionGate | HOLDS | Same run, `--- PASS: TestAdRaQuestionGate`. Covers Q-2 answered through the events.go:391 control, Q-3 opened on its default (kind `default`), J-5 sent back (kind `sent back`), in after-edge order. |
| 3 TestAdRaGuestFenced | HOLDS | Same run, `--- PASS: TestAdRaGuestFenced`. The answer sits after DataNotice inside `<data tier="person-guest" from="J-6">`, with `</data>` escaped and nothing left outside the fence. A tier that is not team, agent or guest is fenced as external. Also `--- PASS: TestAdRaGuestAnswerFromFeed`: the tier is read from the guest's answer record (TierFor("guest")), not from the lease holder's control. |
| 4 TestAdRaNoEdgeUnchanged | HOLDS | Same run, `--- PASS: TestAdRaNoEdgeUnchanged`: Answers nil and empty give the same brief and inputs.json bytes as main's options. `go test -count=1 ./internal/job/...` exit 0. render_test.go, render_job_skills_test.go and the testdata goldens are unchanged (`git diff main --stat`). |
| 5 tests, lint, cross | HOLDS | All six `--- PASS: TestAdRa*` (GuestFenced, TeamAnswerQuoted, NoEdgeUnchanged, PersonAnswer, QuestionGate, GuestAnswerFromFeed). `make lint` exit 0. `make cross` exit 0. |
| 6 diff and skill readers | HOLDS | `git diff main --stat`: internal/job/render.go, internal/job/jobcmd/command.go, skills/run-job/SKILL.md, the two new test files, and this report docs/lanes/C-ad-render-waited-answers.md. `go test -count=1 ./internal/cli/ ./internal/contract/` exit 0. |
| Importers tested | HOLDS | Packages that import internal/job or jobcmd (go list): internal/cli, internal/hook, internal/job/jobcmd, internal/job/lint, internal/jobgraph, internal/lease, internal/migrate, internal/orchestrator, internal/ports. `go test -count=1 ./internal/job/... <those> ./internal/contract/` all ok, exit 0. |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. |
| No guard, gate, cap or hold | HOLDS | The change only adds text to the brief. It refuses nothing and holds no agent. |
| Commits, no attribution | HOLDS | One-sentence messages with no trailers (`git log origin/main..`). |
| Re-render after an answer | NOT DONE (outside Touches) | If a brief was saved before the answer, a later render keeps it: save.go:47-78 reuses a saved bundle when job.yaml and the non-brain sources are the same, and answers are not sources. A Job is normally rendered once it is ready, after its edges are accepted, so this only bites a manual render made before the answer. The fix belongs in jobcmd/save.go, which this lane may not touch. I proved this case red during the build and then took it out of the test. |

## What changed

- internal/job/render.go: `RenderOptions.Answers []Answer` and `Answer{Edge, Who, At, Text, Tier, Kind}`. `answersBlock` is called once, right after `## Context`, and writes `### Answers this Job waited for`, then `<edge>: <who> <kind> at <at>` and the text for each answer.
  - Team and agent text is quoted with `> `, as sourceBlock does, so a line such as `## Acceptance` in an answer cannot forge a section.
  - Any other tier goes only through `fenced()`, and a fenced answer sets `DisableSkillShellExecution`, as a marked source does at render.go:114.
  - A `default` answer has no who, so its line is `<edge>: default at <gate at>`.
  - No answers write nothing.
- internal/job/jobcmd/command.go: `waitedAnswers` runs before the Render call. It reads the counted Feed records only, folded with `feed.FoldReadAt`. Nothing else in the command changed.
  - Person Job edge: the control in the Job's history whose status equals the folded status and that carries `answer`. Accepted gives `answered`; rejected with `send_back` gives `sent back`.
  - Who, when and tier come from the earliest unleased `answer` record on the Job's first record that carries the same text (person.go:64). Without one (the leased path, person.go:271) they come from the control's author.
  - `gate:Q-<n>` edge: the answer record that the newest `answered` control names (events.go:391), else the person layer's applied answer, else the default the question opened on (ready.go:178) with the gate's `at`. The tier of a default is the question author's.
  - The clock is read here, never in Render.
  - A Feed that cannot be read (no stable id) gives no answers; any other read error fails the render.
- skills/run-job/SKILL.md: one sentence after :51. It says the block is the person's decision: act on it and do not ask again.
- `PastedChars` does not count answer text, because answers are not Sources. The JSON keys of Inputs and Rendered are unchanged.

## NOT VERIFIED on a live screen

Nothing in this lane is UI. Not run here: the app's agent build reading the new block end to end (app agentbuild.go), and the whole-module and cross-repo sets, which run at landing.

## Open items

- The stale brief on a re-render after an answer (save.go, above).
