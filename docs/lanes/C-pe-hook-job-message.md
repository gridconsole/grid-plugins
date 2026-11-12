# C-pe-hook-job-message: a person's message to a running Job reaches its agent through the PostToolUse hook, once

Head: see the bundle (code commit c40a9f10; this file is the commit after it). Base: main 00413d82.

Base check: `git grep -n '"message"' -- internal/hook/` on 00413d82 gives stop.go:54 (`json:"message"` of a transcript line) and test files subagent_brief_name_test.go:28, :89 and worker_job_tooluse_test.go:83. No hook on base returns additionalContext for a Feed message.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 Red on base, green on lane | HOLDS | Base (events.go call reverted with `git stash push internal/hook/events.go`): `go test -count=1 ./internal/hook/ -run TestPeMsg` gives `--- FAIL: TestPeMsgDeliversAPersonsMessage` / `pe_job_message_test.go:66: stdout is not hook output: unexpected end of JSON input: ""` (empty stdout). Lane: `--- PASS: TestPeMsgDeliversAPersonsMessage`; stdout parses to hookEventName PostToolUse and additionalContext "Message from owner: To J-1: use the staging bucket". |
| 2 Once | HOLDS | `--- PASS: TestPeMsgDeliversOnce`: second call empty; two new messages are delivered alone on the next call, oldest first, then empty again. `--- PASS: TestPeMsgByJobFieldAndSession`: a record with `job: J-1` and no prefix is delivered; the same record is told again to a different session id. |
| 3 Not delivered (table) | HOLDS | `--- PASS: TestPeMsgNotDelivered/message_to_another_Job`, `/message_to_J-10_is_not_for_J-1`, `/post_of_kind_close`, `/note_under_the_task_lease`, `/GRID_JOB_unset` (all empty stdout); `/guest_message_to_J-1` delivered. The lease case has teeth: with the Lease check disabled for a moment it failed with the note on stdout, then the check was restored. |
| 4 Never blocks | HOLDS | `--- PASS: TestPeMsgNeverBlocksOnAnUnreadableFeed`: a file where `feed` should be gives exit 0, empty stdout, log `grid hook: job message: ...`. The call sits inside `o.record` (panic safety, v-15). |
| 5 Test sets | HOLDS | `go test -count=1 ./internal/hook/ -run 'TestPeMsg\|TestJobAgentToolCallsThroughTheHook'` ok (0 fail); `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. |
| 6 lint, cross, diff | HOLDS | `make lint` exit 0 (go vet ./...); `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile); `gofmt -l internal/hook/` empty. `git diff main --stat` at the code commit names only internal/hook/events.go (+3), pe_job_message.go (+115), pe_job_message_test.go (+174). This file docs/lanes/C-pe-hook-job-message.md is the runbook's proof table, in its own commit. |
| Packages that import hook | HOLDS | `go list` importers of internal/hook: internal/cli, internal/packaging. `go test -count=1 ./internal/hook/ ./internal/cli/ ./internal/packaging/` ok, ok, ok. |
| Walk tests (plugin) | HOLDS | `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` ok; `internal/job/lint` ok (row 5). The app walk tests do not apply to this repo. |
| RealPlugin tests | n/a | No *RealPlugin* test in the brief; none run. |
| Windows and macOS builds | HOLDS | `make cross` exit 0. |
| No guard, gate, cap or hold on agents | HOLDS | The hook only reads the Feed and adds context; it blocks nothing, exits 0 on every error. No command, hooks.json, commands.json or Feed record shape changed. |
| New tests fail on base, pass on lane | HOLDS | Row 1; on base TestPeMsgDeliversAPersonsMessage, TestPeMsgDeliversOnce, TestPeMsgByJobFieldAndSession, TestPeMsgNotDelivered/guest_message_to_J-1 and TestPeMsgNeverBlocksOnAnUnreadableFeed fail (nothing delivered, no log line); the other not-delivered cases pass on base by design, as they expect empty stdout. No existing test edited or deleted. |

## What changed

- `internal/hook/pe_job_message.go`: `peMsgDeliver` reads `Filter{Type: "post"}` of the task Feed on PostToolUse when GRID_JOB is a valid Job id and the task resolves. It keeps the posts with `body.kind == "message"`, an author tier of PersonTeam or PersonGuest (`contract.TierFor`), a Lease other than "task", and either `job == GRID_JOB` or text starting `To <GRID_JOB>:`. Each not yet seen is marked in the session store (`Seen["pemsg:<job>:<record id>"]`, under the session lock) and written as one `hookSpecificOutput` with `additionalContext` "Message from <author id>: <text>", one line per message, bounded by `boundContext`.
- `internal/hook/events.go`: three lines in the `default:` branch of `remaining`, for PostToolUse only, inside `o.record`.
- `internal/hook/pe_job_message_test.go`: the TestPeMsg tests.

Notes: a hook call with no session id delivers nothing (no way to mark it seen). A message is marked seen before it is written, so a failed stdout write loses it rather than repeating it.

## NOT VERIFIED on a live screen

- A real Claude Code session receiving the PostToolUse additionalContext from this hook, and a real post from the app's Agents composer (no claude run or app in this sandbox). The Feed record is built with `feed.Store.Post` in the shape the brief gives.
