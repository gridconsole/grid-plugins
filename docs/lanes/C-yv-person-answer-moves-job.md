# C-yv-person-answer-moves-job

A person's answer to a person Job now also moves the builder Job it was raised for, when that Job is still Submitted.

Base: main 219ca2bb3e97ac493b3888649013dce4985d8063. Base check, `git grep -n '"target"' -- internal/orchestrator/person.go`: :106 `"target": item.ID`, :194 `"target": id` (sentBackControl), :271 `"target": id`. None names a discovered_from Job, so the lane went ahead.

## What changed

- `internal/orchestrator/person.go`: one helper, `yvPamControl`, and two call sites.
  - `personAnswers` calls it just before the person Job's control, with the pass snapshot and a per-pass `moved` set. The set stops two person Jobs raised for one builder, answered in one pass, from posting two controls on it (the pass reads one snapshot).
  - `AnswerPersonWith` (leased path) calls it just before its control, with the snapshot it loaded under the lock and no `moved` set.
  - Each posts the builder's control (`Re` the answer message) before the person Job's control.
  - Finding b: start at the person Job's `discovered_from`; while the Job's role normalizes to `reviewer` or `checker`, step to its `discovered_from`; the first other Job is b. A visited set ends a cycle. No `discovered_from`, an unknown id, or a chain that ends on a review: nothing is posted.
  - Only a Submitted b moves. Answer: `{target b, status accepted, reason "answered by <who> through <person Job>", answer}`. Send-back: `{target b, status rejected, reason and close_reason "sent back by <who> through <person Job>", answer}`, with no `send_back` or `sent_back_hash`. `sendsBack` is unchanged.
- `internal/orchestrator/yv_person_answer_moves_test.go`: new, `yvPam` helpers and `TestYvPam` tests.
- `docs/lanes/C-yv-person-answer-moves-job.md`: this report. It is the one file outside the brief's Touches list, written because the runbook asks for it; `git diff main --stat` named only person.go and the test file before it was added.
- No command, flag, record type, golden, walk table or ops row changed.

## Proof table

| Row / rule | Result |
|---|---|
| 1 Unleased answer moves J-1 | HOLDS. `TestYvPamUnleasedAnswerMovesBuilder`: J-2 accepted ("answered by dana"), J-1 accepted with reason "answered by dana through J-2" and answer "Looks right.". J-1 stays submitted until the holder's tick. |
| 1 Red on base | HOLDS. Base 219ca2bb (new test file only, person.go unchanged): `yv_person_answer_moves_test.go:89: J-1 is submitted, want accepted`. Green on the lane: `--- PASS: TestYvPamUnleasedAnswerMovesBuilder (0.43s)`. |
| 2 Leased answer, no tick | HOLDS. `TestYvPamLeasedAnswerMovesBuilder`: red on base `yv_person_answer_moves_test.go:100: J-1 is submitted, want accepted`; `--- PASS ... (0.23s)` on the lane. Also asserts the control on J-1 is written before the one on J-2. |
| 3 Rounds used | HOLDS. `TestYvPamRoundsUsedMovesBuilderNotReview`: red on base `:134: J-1 is submitted, want accepted`; PASS on the lane. J-1 accepted; J-1.1 has 1 status control before and 1 after. |
| 4 Send-back | HOLDS. `TestYvPamSendBackRejectsBuilder`: red on base `:150: J-1 is submitted, want rejected`; PASS on the lane. J-1 rejected, reason and close_reason "sent back by dana through J-2", no `send_back` or `sent_back_hash`; J-2 rejected "sent back by dana". |
| 5 No second move | HOLDS. `TestYvPamBuilderNotSubmittedGetsNoSecondMove` (accepted before the answer, cancelled, rejected): the count of status controls on J-1 does not change and J-2 still closes. `TestYvPamNoBuilderToMove`: discovered_from null and discovered_from a Job the Feed lacks (J-9) leave J-1 submitted with no new control. These pass on base too, by nature: they guard the lane against over-moving. |
| Extra: two person Jobs, one builder | HOLDS. `TestYvPamTwoPersonJobsOneMove`: red on base `:238: J-1 has 2 status controls, want 3`; PASS on the lane (one control added). |
| 6 `go test -count=1 ./internal/orchestrator/` | HOLDS, exit 0: `ok .../internal/orchestrator 198.629s`. |
| 6 `go test ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0: `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`. |
| 6 `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0: `ok .../internal/job/lint 0.090s`. |
| 6 `make lint` | HOLDS, exit 0 (`go vet ./...`, no output). |
| 6 `make cross` | HOLDS, exit 0: darwin/arm64 and windows/amd64 vet and test compile of every package. |
| 6 after `git merge main` | HOLDS: `Already up to date.` (the lane base is main's head). |
| 6 `git diff main --stat` | HOLDS: person.go (58 insertions) and yv_person_answer_moves_test.go (240 insertions), before this doc was committed. |
| Importers of internal/orchestrator, tests run | HOLDS. `go list` (Imports and TestImports): internal/cli, internal/mcp, internal/migrate, internal/job/jobcmd, plus orchestrator itself. `go test -count=1` on cli (47.478s), jobcmd (1.990s), mcp (0.191s), migrate (4.060s): all `ok`. |
| Walk tests | HOLDS. Plugin walks: TestHsIpWalksEveryCommand and TestIlGuestSiteList PASS; `internal/job/lint` ok. The app walks (`make webui-test` and the rest) belong to the app repo and do not apply here. |
| RealPlugin tests | NOT APPLICABLE: no row names one. |
| No guard, gate, cap or hold on agents | HOLDS: nothing in the diff refuses, delays or limits an agent; the visited set only ends a discovered_from cycle. |
| New test file named in Touches, no shared table row | HOLDS. |
| Names unused before adding | HOLDS: `git grep -n -w 'yvPam\|TestYvPam'` found nothing on base. |
| Commit messages | One sentence, no attribution lines. |

## Not verified

- No real run: no app Judge, real plugin or live Feed. The brief says round 4 shows it end to end.
- sendsBack is true only for kind approve and the Judge posts kind ask, so the send-back path is covered by the test only; today the accept path is the live one.
- The leased path writes the control on b with the lease writer; an unleased person never writes it, only the holder's pass does.
