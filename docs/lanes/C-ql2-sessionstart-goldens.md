# C-ql2-sessionstart-goldens

Lane: lane-C-ql2-sessionstart-goldens, built on lane-C-ql-sessionstart-read-step (its report: docs/lanes/C-ql-sessionstart-read-step.md). Touches add the four golden files.

## Proof table

| Row | Result |
|---|---|
| 1. `git merge origin/main` | HOLDS. "Already up to date", exit 0; no file resolved. |
| 2. Reds of the lane report | HOLDS. Its only red was row 5 (four golden files outside Touches). They are in Touches now; no code change was needed. |
| 3. Golden files hold only the closing line | HOLDS. `git diff main -- testdata/golden`: `brain-session-index.golden` +1 line `Read a note: grid brain search --topic <topic> <words>, then grid brain show <id>.`; the three `hook-sessionstart-*.golden` change one line each, adding only `Read a note: ... <topic> <words> ... <id>.\n` at the end of additionalContext. Nothing else differs. |
| QL 1. `go test -count=1 -v -run 'TestQl\|TestSessionStartText\|TestCustomTopic\|TestIndexGoldenAndGenerations' ./internal/brain/ ./internal/braincmd/` | HOLDS, exit 0: PASS `TestQlIndexNamesReadStep` (3 subtests), `TestSessionStartTextElidesOverflowWithinBudget`, `TestQlBrainKeeperClaims`, `TestIndexGoldenAndGenerations`, `TestQlIndexCommandsRun`. Red on base was shown in the earlier lane report row 1. |
| QL 2, 3 | HOLDS, same run (`TestQlIndexCommandsRun`, `TestQlBrainKeeperClaims`). |
| QL 4. package tests | HOLDS. `go test -count=1 ./internal/brain/... ./internal/braincmd/ ./internal/cli/ ./internal/hook/ ./internal/job/... ./internal/people/ ./internal/projectenv/ ./internal/routine/ ./internal/task/ ./internal/workflow/ ./internal/golden/ ./internal/contract/ ./internal/packaging/` exit 0, all ok (cli includes `TestHookSessionStartGolden`). Importers of internal/brain (go list, from the earlier report): braincmd, cli, hook, job, job/jobcmd, job/jobtest, people, projectenv, routine, task, workflow. `-v -run 'TestHeMvMessagesNameRealSubs\|TestSkillCommandsImplemented' ./internal/contract/ ./internal/packaging/` exit 0, both PASS. |
| QL 5. `make lint` | HOLDS, exit 0. |
| QL 5. `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| QL 5. diff names only Touches | HOLDS. `git diff main --stat`: the ql Touches (index.go, index_topics_test.go, two new test files, brain-keeper SKILL.md, the lane doc) plus the four golden files, now in Touches. |
| Walk tests (plugin) | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList\|TestHookSessionStartGolden'` ok; `internal/job/lint` ok (in the package run above). |
| App walk tests | Not part of this repo. |
| RealPlugin tests | No row names one. |
| Guards on agents | None added. |

## What changed

Nothing in code or goldens beyond the ql lane; this lane adds only this report, and confirms the four goldens match the code's closing line.

## NOT VERIFIED on a live screen

Nothing needs a screen.

Verdict: DONE.
