# C-ql-sessionstart-read-step

Base: plugin main 44c7de5bacbac31560de11e84afd4ff15980c05d. Lane: lane-C-ql-sessionstart-read-step.

Base check, quoted. `git grep -n 'one line per note' -- skills/brain-keeper/SKILL.md` hit
`skills/brain-keeper/SKILL.md:32: ... the SessionStart index, which carries one line per note (internal/hook/hook.go) ...`.
On the base, `SessionStartText` of three rows holds no `grid brain show`
(the red run below prints the 3-row text, which ends at its last topic row).

## Proof table

| Row | Result |
|---|---|
| 1. `TestQlIndexNamesReadStep` red on base | HOLDS. With `internal/brain/index.go` reverted to base: `go test -count=1 -run TestQl ./internal/brain/` exit 1: `--- FAIL: TestQlIndexNamesReadStep` with `ql_index_read_step_test.go:34: text does not end with the read step: "Brain: project knowledge first, ... topic-02 | Short description. | 2 | - | 0\n"` for 3 rows, and the same for `12_rows_of_80_bytes` (ends `3 topics cut; grid brain topics lists them.\n`) and `40_rows_of_80_bytes` (ends `31 topics cut; ...`). |
| 1. green on the lane | HOLDS. `go test -count=1 -v -run 'TestQl|TestSessionStartText|TestCustomTopic' ./internal/brain/` prints `--- PASS: TestQlIndexNamesReadStep`, `--- PASS: TestQlBrainKeeperClaims`, `--- PASS: TestSessionStartTextElidesOverflowWithinBudget`. The test asserts the closing line is last, `len(text) <= 1200`, and for cut tables the banner directly before the closing line. |
| 2. `TestQlIndexCommandsRun` | HOLDS. `go test -count=1 -v -run TestQl ./internal/braincmd/` prints `--- PASS: TestQlIndexCommandsRun` (exit 0). It keeps and accepts a note, takes the two commands out of the closing line of `SessionStartText`, fills topic `decisions`, word `changing` and id `brain:home/checks`, runs both through `h.run`, asserts exit 0 and the id in each output. |
| 3. `TestQlBrainKeeperClaims` | HOLDS. Red on base (same run as row 1): `brain-keeper still says the index carries one line per note`, `does not name topics/<topic>/TOPIC.md`, `names the read fallback knowledge/<topic>/TOPIC.md as the definition`, `does not say an unregistered repo has no Grid record`. Green on the lane. `TopicFile("x")` is `topics/x/TOPIC.md`. |
| 4. package tests | HOLDS. `go test -count=1 ./internal/brain/ ./internal/braincmd/ ./internal/hook/` exit 0. `go test -count=1 -v -run 'TestHeMvMessagesNameRealSubs|TestSkillCommandsImplemented' ./internal/contract/ ./internal/packaging/` exit 0, prints `--- PASS: TestHeMvMessagesNameRealSubs` and `--- PASS: TestSkillCommandsImplemented`. |
| 5. `make lint` | HOLDS, exit 0 (`go vet ./...`). `gofmt -l internal/brain internal/braincmd` prints nothing. |
| 5. `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| 5. `git diff main --stat` names only Touches | NOT DONE. It also names four golden files that pin the whole index text: `testdata/golden/brain-session-index.golden`, `hook-sessionstart-git.golden`, `hook-sessionstart-personal.golden`, `hook-sessionstart-empty-input-uses-cwd.golden`. They are not in Touches. Any closing line turns them red (`TestIndexGoldenAndGenerations` in internal/brain, `TestHookSessionStartGolden` in internal/cli), so the brief cannot be done inside Touches as written. I added the closing line to each (the three hook files in the JSON-escaped form, `<`). Decision for the orchestrator: add those four files to Touches. |
| Importers' tests | HOLDS. Direct importers of internal/brain (go list): braincmd, cli, hook, job, job/jobcmd, job/jobtest, people, projectenv, routine, task, workflow. `go test -count=1 ./internal/brain/... ./internal/braincmd/ ./internal/cli/ ./internal/hook/ ./internal/job/... ./internal/people/ ./internal/projectenv/ ./internal/routine/ ./internal/task/ ./internal/workflow/ ./internal/golden/` exit 0, all `ok`. The Windows and macOS builds are `make cross` above. |
| Walk tests (plugin) | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok. `go test ./internal/job/lint/` ok (in the importer run). The app walk tests are not part of this repo. |
| App callers | HOLDS. `git -C /srv/gridwork/ref/app.git grep -n 'topics cut; grid brain topics' main` printed nothing on this base too; no app pin of the last line. |
| RealPlugin tests | No row names one. |
| Guards on agents | None added. |

## What changed

- `internal/brain/index.go`: `SessionStartText` ends with `Read a note: grid brain search --topic <topic> <words>, then grid brain show <id>.\n`, after the cut banner when there is one. The per-row reserve is now `60 + len(closing line)` (it was 80), so the banner and the closing line both fit; the text stays within 1,200 bytes. The index is still one row per topic and carries no note text.
- `skills/brain-keeper/SKILL.md` lines 20, 28, 32 only: a proposal without `--project` goes to the project of the folder it runs in, an unregistered repo refuses with `has no Grid record`, only a Personal folder gives home; custom topics are defined at `topics/<topic>/TOPIC.md`; the index carries one line per topic (internal/brain/index.go) and ends with the read step.
- Pins turned red by the closing line, before and after:
  - `TestSessionStartTextElidesOverflowWithinBudget` (index_topics_test.go): `HasSuffix(text, banner)` becomes `Contains(text, banner)` plus a check that the closing line is last. Passes before and after; its short-table "cut" check at the end is unchanged.
  - `TestSessionStartTextCutsLongDescription`: before, `line()` read the last line; it failed with `200 bytes: "Read a note: grid brain search ..."`. After, it reads the second-last line. The row-format `HasSuffix` became `Contains(... + "\nRead a note: ")`.
  - `TestSessionStartTextCutsMultibyteOnRuneBoundary` (three subtests): `HasSuffix(text, want)` failed; it is now `Contains(text, want+"Read a note: ")`.
  - `TestReadOnlyEmptyIndexAndBudget` (store_test.go:237-240) and `sessionstart_job_test.go`: stay green, unchanged.
  - Golden files: the four above.
- New: `internal/brain/ql_index_read_step_test.go`, `internal/braincmd/ql_index_commands_test.go`.

## NOT VERIFIED on a live screen

Nothing here needs a screen. A real session's SessionStart context was not read.

Verdict: NOT DONE (row 5 stat clause: four golden files outside Touches; everything else HOLDS).
