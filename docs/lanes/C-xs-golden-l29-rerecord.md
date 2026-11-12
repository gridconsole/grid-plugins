# C-xs-golden-l29-rerecord: L29 golden cases re-recorded from the Go hooks or deleted

Base: f09dda3a (plugin main, `git rev-parse HEAD` before the first commit).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Base and groups | HOLDS, one premise differs | `git show HEAD:golden/known-differences.txt`: 50 cases; in scope agent-routing 2, brain-read 2, brief-lint 6, grid-env 3, grid-home-allow 3, heartbeat 2, history-log 8, lib-parseFrontmatter 2, stop-check 7, usage-log 3, worker-exit 3 = 41; host-guard 2 and plan-guard 7 left out. No case was already gone. |
| 2. `make golden` skips | HOLDS, before-count differs | Before: `make golden` -v holds 44 `known difference:` skips, not 50 (TOTAL 131: pass 58 / known 44 / fail 0 / skip 29). The six brief-lint cases never reach corpus_test.go:125: `brief-lint` left `legacyHooks` (docs/lanes/E-verdict-2-brief.md:29), so Replay skips them at Resolve with "command not implemented". After: `known difference: 9` (TOTAL 126: pass 91 / known 9 / fail 0 / skip 26): host-guard 2, plan-guard 7 (incl. bash-recall), no kept line other than these. `make golden` exit 0. |
| 3. TestXsRerecord | HOLDS | `go test -count=1 -v -run TestXsRerecord ./internal/golden/` prints `--- PASS: TestXsRerecord` with subtests stop-check/mark-done (case folder) and agent-routing/session (go-expected). It stales stdout, shows the replay differs, shows a flag naming another case rewrites nothing, re-records with `-xs-rerecord=`, then replays with no differences and no skip. The test lives in a new file, so it cannot fail on an unchanged base (the flag and the file do not exist there). |
| 4. Deleted cases | HOLDS | Commit a35de0af names each: heartbeat/post, heartbeat/missing-task: docs/lanes/L29.md:63 (D2); covered by internal/hook/events_test.go:65 `TestHookHeartbeatCannotRenewOrEndOnStop` (fatal at :93). brief-lint/complete, missing-file, no-brief: docs/lanes/L4.md:58, and the hook itself was removed by docs/lanes/E-verdict-2-brief.md:9,:29; replacement is Job lint, internal/job/lint/json_test.go:26 `TestCheckJSONValidRecordIsClean`. Deletions are only under golden/heartbeat/ and those three brief-lint folders. |
| 5. grid-home-allow decisions | HOLDS | Old (Node recording) vs new (Go), per path. inside-read: Read of `$GRID_HOME/projects/demo/tasks/T-1-demo/subagents/01-build.md`: allow, allow (stdout identical; files.diff gains the home-allow counter line `{"event":"PreToolUse","tool":"Read"}`, L4.md:71). own-script: Bash `node $OLD_PLUGIN/skills/plan/scripts/plan-hash.mjs ...`: allow, no grant (stdout now empty, Claude Code's own prompt decides; tighter). permission: PermissionRequest Write of the same task-folder file: allow (addDirectories `$GRID_HOME`), allow (stdout identical; counter line `{"event":"PermissionRequest","tool":"Write"}` added). The old recording denied none of the three paths, so no old denial is allowed by the new one. The 7 other grid-home-allow cases (broad-home, glob-traversal, outside, relative, script-outside, shell-chain, symlink-escape) already passed and are untouched. |
| 6. Tests, lint, cross | HOLDS | `make lint && make cross && go test -count=1 ./internal/golden/ && go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' && go test -count=1 ./internal/job/lint/` exit 0 (darwin/arm64 and windows/amd64 vet and test compile both ran). |
| Changed packages and importers | HOLDS | Only the test package `internal/golden` changed (test files and golden/ data, no production code). Its importers use `golden.Assert` on their own testdata and are not affected by corpus cases; `make cross` compiled every package's tests for both OSes. |
| Walk tests (plugin) | HOLDS | `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` ok; `internal/job/lint` ok (in the chain above, exit 0). |
| New test fails on base | HOLDS by construction | `xs_rerecord_test.go` is new, so it does not exist on f09dda3a. |
| No guard, gate, cap, hold on agents | HOLDS | No production code changed. |
| docs/lanes proof table | HOLDS | this file |

## Not finished (NOT DONE rows)

- brief-lint/missing-field, missing-section, no-frontmatter (3 of the 41): NOT DONE. The brief says to re-record them, but no Go hook exists to record from: the hook was removed (docs/lanes/E-verdict-2-brief.md:9,:29), and Replay skips them before it produces any output. Deleting them is outside the deletions row 4 allows, so their three known-differences lines stay. Suggested follow-up: delete them like the other three, covered by internal/job/lint. brief-lint/invalid-json is not listed and also skips; left alone.

## What changed

- `internal/golden/xs_rerecord_test.go` (new): `-xs-rerecord=<case,case>` test flag and `TestXsRerecord`; all helpers start with `xsRr`.
- `internal/golden/corpus_test.go`: one added line calling `xsRrApply` after Replay (the hook the flag needs).
- 30 cases re-recorded: agent-routing 2, grid-env 3 (written into go-expected, next to their go-diff.md), history-log 8, brain-read 2, stop-check 7, usage-log 3, worker-exit 3, lib-parseFrontmatter 2; plus grid-home-allow 3. Their 33 lines are gone from golden/known-differences.txt. Five cases deleted (2 heartbeat, 3 brief-lint) with their lines.

## For the reviewer: what the re-recordings pin

- stop-check: the cited reason (L29.md:80, D6) describes a wording change, but what landed is docs/lanes/E-verdict-5-stop.md:10, which replaced the old task.md and brief checks with one check, "the Job's result is posted". The five cases with no GRID_JOB (empty-next, empty-summary, finished-no-outcome, reviewer, subagent-missing) now record exit 0 and empty stderr (the silent no-Job path, covered by internal/hook/stop_test.go:94 `TestStopWithNoJobStopsSilently`); mark-done and mark-blocked lose their files.diff because status marking is gone. Their setup.json `proves` text ("blocks Stop with exit 2") is now stale; setup.json is outside Touches, so I left it. A reviewer may prefer deleting these seven instead.
- agent-routing/* and grid-env/*: go-expected/stdout now holds the brain topic index and no longer the agent routing line (the routing line was removed by docs/lanes/E-verdict-2-brief.md:29). go-expected/stderr pins `grid hook: agent sync: open $OLD_PLUGIN/agent-templates: no such file or directory`, which is the Go hook's real output with the recorded CLAUDE_PLUGIN_ROOT pointing at the inert token folder, not a recorded bug. The go-diff.md prose of these cases still describes the old script and was not edited.
- worker-exit/missing-task stderr `grid hook: worker exit: task T-999 not found in project demo` is D8 "missing tasks log and allow".
- The grid-home-allow/permission known-difference reason ("the only-writer rule denies the recorded write") was stale: only-writer is at warn by default, and Go allows through the home grant exactly as the old script did. Decision unchanged, so the re-record does not loosen anything.

## NOT VERIFIED on a live screen

Nothing here is a screen.
