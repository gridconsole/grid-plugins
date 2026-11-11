# Lane Q-skill-run-job-brain

Base: branch point 3ccec5cb (brief names main 9eea4651). Test commit 68ee8e02, skill text commit 3085640d.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. Test first, fails on main | HOLDS | `go test -count=1 ./internal/contract/ -run 'TestSkill\|TestRunJob'` on the base skills, exit 1: `skill_examples_test.go:74: skills/run-job/SKILL.md: no fenced json block` (also review-job, and 5 `run-job skill lacks "..."` lines) |
| 2. run-job and review-job blocks pass `Validate("result", ...)`, review block has non-empty `must_fix` | HOLDS | `go test -count=1 ./internal/contract/ -run TestSkillExamplesValidate -v` exit 0: run-job, review-job and `review-job_has_must_fix` PASS. The body is wrapped in the envelope `grid job result` adds, since the `result` schema is the whole post (internal/contract/skill_examples_test.go:40) |
| 3. Broken copy (state `ok`) fails Validate | HOLDS | `TestSkillExampleBrokenCopyFails` PASS (skill_examples_test.go:93); it also fails if the replace changed nothing |
| 4. Every `grid <verb> <sub>` and `--flag` in the two skills and two agents exists in `contractdata.Commands` | HOLDS | `TestSkillAndAgentCommandsExist` PASS (skill_examples_test.go:110). It reads backticked spans, checks group+sub and each flag of that span, and bare `--flag` spans against the commands the file names. Verified by hand against braincmd usage (internal/braincmd/command.go:33-58): `brain show` (--task), `brain propose` (--name --topic --description --summary --source --body --task --link) |
| 5. run-job holds the post-when-blocked line, `grid brain show`, `grid brain propose` with `--source job:`, `supersedes`; front matter intact | HOLDS | `TestRunJobSkillHoldsTheRules` PASS (skill_examples_test.go:170); the line is skills/run-job/SKILL.md:15 (the test compares lower-case, the skill starts the sentence with a capital) |
| 6. `go test -count=1 ./internal/contract/` 0 fail; `go vet ./internal/contract/` exit 0 | HOLDS | both exit 0 |
| Packages that import internal/contract, plus packaging/settings/eval/golden, which read the skill and agent text | HOLDS | `go test -count=1` exit 0 on: artifact brain braincmd cli doctor eval feed golden guard hook intent job job/... jobgraph land lease mcp orchestrator packaging packaging/cmd people question routine session settings signal task usage where workflow. No non-test Go code changed |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0; `gofmt -l internal/contract` empty |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| New test in a new file, no existing test edited or deleted | HOLDS | only internal/contract/skill_examples_test.go added; `git diff --stat` shows no other test file |
| No guard, gate, cap or hold added | HOLDS | text only says to post a result and how to read and propose brain notes |
| Only the named files touched | HOLDS | skills/run-job/SKILL.md, skills/review-job/SKILL.md, agents/builder.md, agents/reviewer.md, internal/contract/skill_examples_test.go, this report |

## What changed

- run-job: one fenced `json` body with every field the schema requires, a line to post a result before ending also when blocked or when nothing changed, a note on `state` and `decision`, and a Brain section (read with `grid brain show`, propose with `--source job:<task>/<job>`, replace a wrong note with a `supersedes` link given by `--link`, never edit the note).
- review-job: one rejecting reviewer body with a `must_fix` row and the verdict text in `outcome`.
- agents/builder.md and agents/reviewer.md: one sentence each pointing to those rules.
- internal/contract/skill_examples_test.go: the tests above.

## Open items

- The example `verified` is `null`; a Job that needs an L1/L2/L3 record has to write its own, which the skill does not show.
- `grid job result` was not run (no binary run, per the brief); the example is checked against the schema, not against the CLI.

## NOT VERIFIED on a live screen

- That an agent given the new text posts a valid body on a real run, and that it reads and proposes brain notes. That needs a live eval run.
