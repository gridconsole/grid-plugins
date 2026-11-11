# C-sk-questions-ask-job

The questions skill asks through `grid question ask` (a person Job when the caller is a person or holds the task lease) and the contract lists the verb.

Base: main at 0e60a23a (the clone's main; the brief named d0400870).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Skill test fails on main | HOLDS | New test `TestSkillQuestionsAskText`, run on base 0e60a23a before the edits, exit 1: `skills/questions/SKILL.md does not contain "grid question ask"`, `... does not contain "gate:Q-"`, `... still contains "Not built"`, `... still contains "only way work waits"`. Passes on the lane: `go test ./internal/contract -count=1` ok. |
| 2. Contract test fails on main | HOLDS | `TestSkillQuestionsAskContract` on base 0e60a23a, exit 1: `skill_questions_ask_test.go:67: contract/commands.json lists no `question ask``. It also compares the flag lists of `ask` and `add` (name, takes, required); on the lane they match. |
| 3. `make generate` adds only `ask` | HOLDS | `make generate` ok. `git diff --stat contract/commands.json`: 75 insertions, 0 deletions, all inside the new subcommand. `git grep -n '"ask"' contract/commands.json` shows `:2382` (under `question`) and `:2540` (the existing top-level `ask` stub). `git status --short` after generate listed only Touches files. |
| 4. Usage names `ask`, question tests pass | HOLDS | `go run ./cmd/grid question --help` prints `Usage: grid question add\|ask\|withdraw\|list\|tick\|sync ...`. `go test ./internal/question -count=1` ok with `testdata/golden/question-help-help.golden` updated (see note). |
| 5. Roles | HOLDS | `go test ./internal/cli -run 'Guest\|TestQuestionAsk' -count=1 -v`: all PASS, including `TestQuestionAskIsPersonJobAnswerClosesIt`, `TestQuestionAskByAgentPostsNoJob` and the guest parity suite with the new `question ask` row (guest_parity_test.go:89). Role code not touched. |
| 6. Packaging | HOLDS | `go test ./internal/packaging -run TestShippedAgentsAndSkillsAreWellFormed -count=1` ok. |
| 7. No leftover text | HOLDS | `git grep -n "question add --blocking\|only way work waits\|Not built" -- skills/` prints nothing, exit 1. |
| 8. Suites and lint | HOLDS | Package list below, all ok, exit 0. `make lint` (go vet ./...) exit 0. |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Small commits, no attribution | HOLDS | `git log origin/main..HEAD`: one commit, one sentence, no trailers. |
| No guard, gate, cap or hold on agents | HOLDS | Only text, usage string, test tables and generated contract changed. |
| Existing tests not edited to pass | HOLDS | Existing tests only gained table rows the brief names (contract_inventory_test.go:273 and one annotation, guest_parity_test.go:89). No test deleted. |

Packages tested (run with `go test ... -count=1`, all ok): contract/formats, internal/artifact, brain, braincmd, cli, feed, guard, hook, intent, job (and job/frontmatter, job/jobcmd, job/lint), jobgraph, land, lease, mcp, orchestrator, ports, question, session, settings, signal, task, usage, where, workflow, contract, packaging (and packaging/cmd). This is every package `go list` shows importing internal/question or internal/contract, plus the two themselves.

## What changed

- skills/questions/SKILL.md: line 10 says a Job waits only by naming the person Job (or `gate:Q-<n>`) in its own `after`, and that `--blocking` is a priority label that holds no Job. Step 2 asks with `grid question ask` and the `add` flag set (minus `--blocking`), says a person or the task-lease holder gets a person Job and any other agent gets none, and keeps `grid question add` only as the form with no Job. The "Not built" paragraph is gone. The Done line names no verb and stays as it was.
- internal/question/command.go: both usage lines name `ask`.
- testdata/golden/question-help-help.golden: the one usage line.
- contract/commands.json: `question ask` added by `make generate`.
- internal/cli/contract_inventory_test.go: `ask` in the question paths and one `question ask` annotation.
- internal/cli/guest_parity_test.go: one `question ask` row.
- internal/contract/skill_questions_ask_test.go: new, two tests.

Notes:
- The golden was edited by hand to the single changed line instead of with `UPDATE_GOLDEN=1 go test ...`, because the sandbox refuses an env-var prefix other than GOOS and CGO_ENABLED. `TestLeaseHelp` compares against it and passes, so the file matches the program's output.
- The base commit in the clone is 0e60a23a, not d0400870 as the brief says; `make generate` ran on that base.

Left as they are, per the brief: the `question` summary at internal/cli/cli.go:115 (F-cli-drop-stubs), agent-templates/grid-planner.md:27 and :53 (still say `grid question add`), contract/README.md, the top-level `ask` stub at internal/cli/cli.go:116.

Landing order: this lane lands before C-sk-setup-answers-contract; if F-cli-drop-stubs lands first, rebase and rerun `make generate`, never merge commands.json by hand.

## NOT VERIFIED on a live screen

Nothing here is a UI change. Not run: a real `grid question ask` against a live Feed as a person and as the lease holder (covered by TestQuestionAskIsPersonJobAnswerClosesIt in the test harness only).
