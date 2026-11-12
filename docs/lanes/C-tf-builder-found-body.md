# C-tf-builder-found-body

Base: c7c10553 (lane-Pint135 merge head; the brief quoted main 2b47f621). Base check: `git grep -n "found" HEAD -- skills/run-job/SKILL.md` shows :40 (`"found": []` in the result example) and :53 with no body; `grid job show --json` printed no record id (the report struct had no such field), so the lane went ahead.

Code head (before this report commit): b4018231.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. `TestTfFoundShowRecord`: `job show <task> J-2 --json` prints `record` equal to `kbRoleJobRecord(t, a, "J-2")` | HOLDS | `go test -count=1 -v -run 'TestTfFound' ./internal/cli/` exit 0, `--- PASS: TestTfFoundShowRecord (0.89s)`. Red on base (the two production files from c7c10553 put back): `tf_found_body_test.go:109: job show J-2 --json: record "", want the job record "01M76KSM88WVK0RK6JEAGXJW9H"` and `--- FAIL: TestTfFoundShowRecord`. The same test also pins the text output: it holds no record id and still ends `state: Started`. |
| 2. `TestTfFoundBuilderPosts`: the builder holding J-2's lease posts `--type found` with the run-job body, `re` from row 1's field, kind security: exit 0, one found record whose body `re` is that id | HOLDS | Same command, `--- PASS: TestTfFoundBuilderPosts (0.62s)`. The rig assigns J-2 with `job next`, sets GRID_LEASE=J-2 and its epoch, acks, then posts the body read from SKILL.md with job/re/at/text filled; it asserts exit 0, one found record, body `re` equal to the show field and to `kbRoleJobRecord`, kind security, and J-2 NeedsLead. Red on base: `the line that names --type found gives no body` and `--- FAIL: TestTfFoundBuilderPosts`. |
| 3. `TestTfFoundSkillBody`: the `--type found` line of run-job SKILL.md gives a body that decodes to the six keys found.json requires, `re` naming `grid job show --json` | HOLDS | Same command, `--- PASS: TestTfFoundSkillBody (0.00s)`. The test reads the required list from contract/schemas/found.json, not a copy. Body is at skills/run-job/SKILL.md:53. Red on base: `tf_found_body_test.go:153: the line that names --type found gives no body: A security problem you meet, ...` and `--- FAIL: TestTfFoundSkillBody`. |
| 4. `go test -count=1 ./internal/job/... ./internal/cli/...` exits 0; `git diff main --stat -- contract/` empty | HOLDS | Exit 0: `ok internal/job 4.723s`, `ok internal/job/frontmatter`, `ok internal/job/jobcmd 6.055s`, `ok internal/job/lint`, `ok internal/cli 103.125s` (kb_found_role_test.go and the jobcmd tests are in these). `git diff main --stat -- contract/` printed nothing, exit 0. |
| 5. `make lint` exits 0 and `make cross` exits 0 | HOLDS | `make lint` (`go vet ./...`) exit 0, no output. `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test compile (tests are compiled and run with `/bin/true`, not executed). `gofmt -l internal/job/jobcmd internal/cli/tf_found_body_test.go` prints nothing. |
| Rule: tests of changed packages and of every importer | HOLDS | `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... \| grep internal/job/jobcmd` names: cmd/grid, internal/cli, internal/job/jobcmd, lib. `go test -count=1 ./internal/job/... ./internal/cli/...` exit 0 (row 4); `go test -count=1 ./cmd/grid ./lib` exit 0 (`cmd/grid [no test files]`, `ok lib 38.483s`). |
| Rule: plugin walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0, `ok internal/cli 0.265s`. `go test -count=1 ./internal/job/lint/` exit 0, `ok internal/job/lint 0.442s`. (The app walk tests in the runbook are for the app repo.) |
| Rule: new tests fail on base, pass on the lane | HOLDS | Rows 1-3 above: all three red with the base `command.go` and `SKILL.md` checked out, green on the lane. `git status --short` was clean after `git checkout HEAD -- internal/job/jobcmd/command.go skills/run-job/SKILL.md`. |
| Rule: golden | HOLDS | `grep -rn "job show" golden/ -l` printed nothing, so no golden pins the show output. I did not run `make golden` and did not run the golden package's tests on their own (`make cross` only compiles tests), so no golden moved is read from that grep alone. |
| Rule: helper names | HOLDS | tf_found_body_test.go: helpers `tfFoundBuilder`, `tfFoundShow`, `tfFoundRecord`, `tfFoundSkillBody`; tests `TestTfFoundShowRecord`, `TestTfFoundBuilderPosts`, `TestTfFoundSkillBody`. |
| Rule: no guard, gate, cap or hold on agents; no personal paths, hosts or emails; no attribution lines | HOLDS | The change adds one output field, one SKILL.md sentence and tests. Commit messages are one sentence each with no attribution line. |
| Rule: touches only the brief's list | HOLDS | `git diff --stat c7c10553..HEAD`: internal/job/jobcmd/command.go, skills/run-job/SKILL.md, internal/cli/tf_found_body_test.go, plus this report. contract/ unchanged. |

## What changed

- internal/job/jobcmd/command.go: `report.Record` (`json:"record,omitempty"`). In `show`, when a task was resolved and its Feed was read, `jobRecordID` gives the id of the newest non-stale `job` record of the Job (the same choice as `pickJob`, using `identity`). The text output is unchanged because `Record` is only in the JSON.
- skills/run-job/SKILL.md:53: the full found body on the `--type found` line: `{"job": "<your job>", "re": "<the record field of grid job show --json>", "at": "<path:line>", "text": "<one line>", "kind": "security", "class": null}`, and the command `grid job show <task> <job> --json` that prints the record field.
- internal/cli/tf_found_body_test.go: the three tests.

## Open items and NOT VERIFIED

- Not verified on a live screen or with the built plugin: no row names a RealPlugin test, so none was run. Nothing here needs a live screen.
- A Job posted more than once has more than one `job` record; `record` is the newest non-stale one, as `grid job render` chooses. No test posts a Job twice (the brief allows one new file with these three rows), so that choice is read from the code only.
- When the Job was never posted to the Feed (a brief saved from plan.md only) or the task has no stable id, `record` is left out. A builder with no record has no `re` to give; the brief does not say what then.
- The text rendered into every Job brief (Go code in internal/job, outside Touches) still says "`--type found --body <json>`, kind `security`" with no body and points to the `run-job` skill for the detail. That line is now covered by SKILL.md:53; adding the body there is a follow-up brief.
- The two app readers named in the brief (agentbuild.go:273, taskcode/build.go:92) were not run here; the brief says neither decodes with DisallowUnknownFields.

Verdict: DONE
