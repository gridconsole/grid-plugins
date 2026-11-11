# C-im-job-skills-bash-lines-pin

Base: ee8a96c126781462cbe0067771db40d01911cd76 (main). Skills and agents read on this base: run-job/SKILL.md:55-61, plan-jobs/SKILL.md:26-32, review-job/SKILL.md:18-24, brain-keeper/SKILL.md whole (no Bash part), agents/{builder,curator,orchestrator,reviewer}.md `skills:` = run-job, brain-keeper, plan-jobs, review-job. `git grep -n -i "one command per\|dangerouslyDisable" -- '*_test.go'` on base finds only internal/routine/unschedule_remove_test.go:15, a comment about timer files, so no test read these lines.

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. `go test ./internal/contract/ -run TestImBash -count=1 -v` exits 0 and lists the four agents and their skills | HOLDS | exit 0; `-v` logs `agents/builder.md preloads skills [run-job]`, `agents/curator.md preloads skills [brain-keeper]`, `agents/orchestrator.md preloads skills [plan-jobs]`, `agents/reviewer.md preloads skills [review-job]`, with PASS subtests `builder.md/run-job`, `curator.md/brain-keeper`, `orchestrator.md/plan-jobs`, `reviewer.md/review-job` |
| 2. Mutation: run-job :57 deleted | HOLDS | FAIL: `im_job_bash_lines_test.go:138: agents/builder.md: skill run-job has no one-command Bash line: the closest line lacks one command per Bash call, `;`, `&&`, pipe, `$(` / `--- FAIL: .../builder.md/run-job`; restored with `git checkout -- skills/run-job/SKILL.md` |
| 3. Mutation: review-job :24 deleted | HOLDS | FAIL: `im_job_bash_lines_test.go:141: agents/reviewer.md: skill review-job has no line forbidding `dangerouslyDisableSandbox`: missing `dangerouslyDisableSandbox`` / `--- FAIL: .../reviewer.md/review-job`; restored with `git checkout -- skills/review-job/SKILL.md` |
| 4a. Mutation: agents/imtmp.md (builder copy, no `skills:`) | HOLDS | FAIL: `im_job_bash_lines_test.go:128: agents/imtmp.md: front matter has no `skills:` list, so no skill carries the Bash lines for this agent` / `--- FAIL: .../imtmp.md`; scratch file removed (`git clean -f agents/imtmp.md`) |
| 4b. Base brain-keeper (`git stash push -- skills/brain-keeper/SKILL.md`) | HOLDS | FAIL: `:138: agents/curator.md: skill brain-keeper has no one-command Bash line: the closest line lacks one command per Bash call, `;`, `&&`, `$(` and `:141: agents/curator.md: skill brain-keeper has no line forbidding `dangerouslyDisableSandbox`: missing `dangerouslyDisableSandbox`` / `--- FAIL: .../curator.md/brain-keeper`; `git stash pop`, then row 1 is green again |
| 5. Diff names only the test, brain-keeper and this table; brain-keeper diff adds only the heading and two bullets | HOLDS | `git diff main --stat`: docs/lanes/C-im-job-skills-bash-lines-pin.md, internal/contract/im_job_bash_lines_test.go, skills/brain-keeper/SKILL.md. `git diff main -- skills/brain-keeper/SKILL.md`: +`## Bash in a Job`, + two bullets byte-identical to run-job/SKILL.md:57 and :61, plus blank separator lines, placed before `## Done` |
| 6. `go test . ./internal/job/ ./internal/contract/ ./internal/job/lint/ ./internal/braincmd/ -count=1`; `go vet ./...` | HOLDS | all five `ok`, exit 0; `go vet ./...` exit 0 |
| Rule: new tests fail on base, pass on lane | HOLDS | row 4b is the base failure (curator/brain-keeper); row 1 is the lane pass |
| Rule: edited no existing test, deleted none | HOLDS | diff has one new test file only |
| Rule: no guard, gate, cap, hold or hook on agents | HOLDS | skill wording and one test only |
| Rule: tests of changed packages and of their importers | HOLDS | changed: root (embeds skills), internal/contract. Importers from `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}}' ./...`: artifact brain braincmd cli contract feed guard hook intent job job/jobcmd job/lint jobgraph land lease mcp migrate orchestrator ports question session settings signal task usage where workflow. `go test . <those 27> -count=1`: all 28 `ok`, exit 0 |
| Rule: lint | HOLDS | `make lint` (go vet ./...) exit 0; `gofmt -l internal/contract/` prints nothing |
| Rule: Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0; `GOOS=windows/darwin go vet ./internal/contract/` exit 0 |
| Rule: mutation edits reverted before commit | HOLDS | `git status --short` was clean of run-job, review-job and agents/imtmp.md before each commit |
| Rule: no attribution lines, no personal paths | HOLDS | commit messages are one sentence each; the test uses relative paths only |
| RealPlugin tests | N/A | none named by the brief |

## Deviation from the brief, for review

The brief names the word `` `$(` `` (backtick, `$(`, backtick). No skill line contains that: the lines write `` `$(...)` ``. Pinned the base line as it is, so the word is "`$(" (opening backtick plus `$(`), see internal/contract/im_job_bash_lines_test.go:18-20. With the brief's literal word, rows 1 and 6 failed on all four skills (first run: `the closest line lacks `$(``). It still pins the substitution word, and a later wording fix that keeps `` `$(...)` `` stays green.

## What changed

- internal/contract/im_job_bash_lines_test.go (new): TestImBashEveryJobAgentSkillCarriesTheLines reads agents/*.md (os.ReadDir ../../agents, `.md` only), parses each front matter `skills:` list (block or inline), fails on an agent with none, and for each skill (gridplugins.SkillFor) asserts one line holding `one command per Bash call`, `` `;` ``, `` `&&` ``, `pipe`, "`$(" and one line holding `` `dangerouslyDisableSandbox` `` with `Never` or `Do not`. The failure names the agent file, the skill and the missing words.
- skills/brain-keeper/SKILL.md: new `## Bash in a Job` before `## Done`, with the two bullets copied from skills/run-job/SKILL.md:57 and :61.

## NOT VERIFIED on a live screen

Nothing in the lane has a UI. Whether a curator model obeys the new lines in a real run is not proven here; the next real run's permission_denied count for curator Jobs shows it.

DONE
