# H-guest-plugin-role: a guest's close, reopen, pickup and ship posts and task writes are refused

Repo: plugin. Branch lane-H-guest-plugin-role from main. The owner's 11 Nov answer: guest = answer and comment only. This lane makes the plugin refuse what the app already refuses (app docs/lanes/D-guest-writes.md open item 5).

The D-guest-writes review (tmp/hand/review-D-guest-writes.md) has two must-fix rows; both are in the app (`task_intent` role check, jstest). Its only plugin-side item is note 4 ("the plugin may still accept close/reopen under GRID_ROLE=guest"), which is this lane.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. Guest `grid task close`, `reopen`, `stop`: non-zero, refusal names the guest role, task folder unchanged | HOLDS | `go test ./internal/cli -run TestGuest -count=1` exit 0 (`ok`). `TestGuestTaskMoveRoles/guest/{close,reopen,stop}` runs the CLI with `GRID_ROLE=guest` against an invited guest, expects exit 1 and `role "guest" may not <verb> a task` on stderr, and compares every file under the task folder (task.md and history.md included) before and after. `TestGuestTaskMoveRefusalIsEarly` shows the same refusal for a task that does not exist. Code: internal/cli/cli.go:247 (`guestWriteVerb`), :254-264 (helpers), :216-221 (the refusal). |
| 2. Guest `feed post --type post` with kind close, reopen, pickup, ask-pickup, ship: exit 3, no record appended | HOLDS | `TestGuestFeedPostKinds/guest/{close,reopen,pickup,ask-pickup,ship}`: exit 3, stderr names the guest role, `store.Read` record count equal before and after. `TestGuestFeedPostNotAMessage` adds no kind, a numeric kind, `Close`, ` close`, and a body that is only a body reference: all exit 3, no record. Code: internal/cli/feed.go:137 (call) and :253-266 (`refuseGuestPostKind`). |
| 3. Guest kind `message` posts (exit 0); a guest answer to a question addressed to it posts; GRID_LEASE=task still exempts a Grid-run session; `feed_test.go` and `answer_post_roles_test.go` pass unchanged | HOLDS for message, answer, and the lease exemption of the task verbs. See the note under "Base" for the feed post. | `TestGuestFeedPostKinds/guest/message` exit 0 and one record more; `TestGuestStillAnswersAndComments/{message,answer}` (answer closes the gate); `TestGuestStillAnswersAndComments/lease_task_task_close`: guest with `GRID_LEASE=task` runs `grid task close`, exit 0, folder changed (cli.go:204 condition is unchanged). `feed_test.go` and `answer_post_roles_test.go` are not in `git diff --stat origin/main..HEAD`, and `go test ./internal/cli/... -count=1` exit 0. |
| 4. Member and owner: every row of 1 and 2 still succeeds (one table test) | HOLDS | `TestGuestTaskMoveRoles/{member,owner}/{close,reopen,stop}` (role x verb table): exit 0 and the folder changes. `TestGuestFeedPostKinds/{member,owner}/<six kinds>` (role x kind table): exit 0 and one record more. Both pass in the run of row 1. |
| 5. `go test ./internal/cli/... ./internal/feed/... -count=1` and `go vet ./...` | HOLDS | `go test ./internal/cli/... ./internal/feed/... ./cmd/grid/... -count=1` exit 0: `ok internal/cli 44.036s`, `ok internal/feed 1.423s`, `cmd/grid [no test files]`. `go vet ./...` exit 0. |
| Importers of the changed package | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./... \| grep internal/cli`: only `github.com/gridconsole/grid-plugins/cmd/grid`; it has no test files, and is covered by the run above. |
| Lint, Windows and macOS builds | HOLDS | `gofmt -l .` empty (the second half of `make lint`; its `go vet ./...` is above). `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go vet ./...` exit 0. `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go vet ./...` exit 0 (vet compiles every package and test file). |
| New tests fail on base, pass on the lane | HOLDS | Base (tests added, source unchanged), `go test ./internal/cli -run 'TestGuestTaskMove\|TestGuestFeedPost\|TestGuestStill'`: <br>`--- FAIL: TestGuestTaskMoveRoles/guest/close: guest task close: exit 0, want 3` (the first run; the same for reopen and stop, and now exit 1 on the lane)<br>`--- FAIL: TestGuestTaskMoveRefusalIsEarly/close: exit 1: "grid task close: task not found\n"`<br>`--- FAIL: TestGuestFeedPostKinds/guest/close: refusal does not name the guest role: "grid feed: $.body.kind: const mismatch\n"` (the same for reopen, pickup, ask-pickup, ship)<br>`--- FAIL: TestGuestFeedPostNotAMessage/body_ref: exit 0, want 3:`<br>On the lane the same run is `ok`. |
| No existing test edited to pass | HOLDS, with one extension | One existing test file is touched: internal/cli/guest_parity_test.go (+2 lines, `guestParityEarlyAnswer`). That function is the test's own list of documented pre-resolution answers; it already holds task delete/restore and question withdraw. Because `guestWriteVerb` now answers `task close\|reopen\|stop` before any project is resolved, the invited answer equals the unknown answer, and the test requires each such command to be listed with its exact answer. I added the three rows with the literal text. No assertion was loosened. `TestPersonalBypassTaskCommands` (hard-codes exit 1 for a guest's `task stop`) needed no edit: that is why the task refusal exits 1, not 3. |
| No test deleted | HOLDS | `git diff --stat origin/main..HEAD` shows no deleted file and no removed test lines (1 deletion is the old `return len(rest) > 0 && (...)` line in cli.go). |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The refusal is on the guest person's role (`GRID_ROLE=guest`, as cli.go:204 already did for `setup`, `task delete` and `question withdraw`). An agent role (worker, lead, agent, cli) is not affected; GRID_LEASE=task still exempts a Grid-run session. No hook added. |
| Small commits, one-sentence messages, no attribution, no personal paths | HOLDS | `git log origin/main..HEAD`: 3 commits (two code, one report). |
| Not touched: contract schemas, internal/feed/store.go, answer handling, app repo, contract/commands.json | HOLDS | The diff stat lists only internal/cli/{cli.go, feed.go, guest_parity_test.go, guest_role_test.go} and this report. |

## Base: what was already true, and what was not

The brief says a guest's non-message post "goes through" on plugin main. For a post whose body has a kind, that is not so on this base: contract/schemas/post.json:113-139 (a guest's or external's body kind is `const message`) already refuses it with exit 3 (`grid feed: $.body.kind: const mismatch`) and nothing is appended. So row 2 held for those five kinds before the lane, but the refusal did not name the role. What did go through on base:

- A guest post whose body is only a body reference, `{"ref":"bodies/<ULID>.json"}` (the second `oneOf` branch of post.json), exit 0 on base; the schema's guest rule does not look at a body without a `kind`. Now exit 3 (`TestGuestFeedPostNotAMessage/body_ref`).
- `grid task close|reopen|stop` under `GRID_ROLE=guest`, for a guest invited to the project: exit 0 and the task changed (internal/task has no role check on `Change`). Now refused.

Because the schema refuses a guest's non-message post whatever the lease, `GRID_LEASE=task` does not exempt a feed post of kind close and the like (that was already so on base, and the schema is not in this brief). The exemption of cli.go:204 (the guest-verb gate: `task close|reopen|stop`, `task delete|restore`, `question withdraw`, `setup`) is unchanged and tested.

## What changed

- internal/cli/cli.go: `guestWriteVerb` lists `task close|reopen|stop` (new `guestTaskMove`, `guestTaskMoveRefusal`). The refusal is printed before any project is resolved, so an unknown task and an invited one give the same answer. Text: `grid task close: role "guest" may not close a task; a guest answers and comments`, exit 1.
- internal/cli/feed.go: `feed post --type post` refuses a guest (`contract.NormalizeRole(GRID_ROLE) == "guest"`) whose body `kind` is not the string `message`, as `*feed.InvalidRecordError` (exit 3), after the task is resolved and before anything is written. Text: `grid feed: role guest may post a message only, not kind close`.
- internal/cli/guest_role_test.go: new, the tests above.
- internal/cli/guest_parity_test.go: +2 lines, the three early answers (see above).

## Open items

1. MCP `feed_post` (internal/cli/mcp.go:25-31, the `Session` callback) does not call `refuseGuestPostKind`, so a guest session posting through MCP still has the body-reference gap above (the schema still refuses a kind other than message). The brief names feed.go only; the fix is one call: `refuseGuestPostKind(env.getenv("GRID_ROLE"), in.Type, in.Body)` in that callback.
2. cli.go:204 compares `GRID_ROLE` as `strings.ToLower(TrimSpace(...)) == "guest"`, so the spelling `grid-guest` (which `contract.NormalizeRole` and `where.Caller.IsGuest` accept) is not refused for any guest verb, old or new. Pre-existing; not in the brief.
3. A guest can still make other task moves: `grid task status`, `task attach`, `task move`, `task new`. The brief names close, reopen and stop only. Likewise the post.json role list lets a guest write `control` and `knowledge` type records (post.json is `x-roles` for `post`; `control` and `knowledge` list guest in the types output at contract_commands_test.go:389). Worth an owner's answer on whether those are "answer and comment".
4. `grid task close --help` for a guest is refused like `task delete --help` is today (`guestWriteVerb` exempts only a bare `--help`).

## NOT VERIFIED on a live screen

- The dev UI: that the app's close/reopen/ship controls are hidden for a guest and that the plugin refusal text shows up in the app when a guest session reaches the plugin (no session or server was run here).
- A real `claude` session running as a guest through the MCP server (open item 1).
