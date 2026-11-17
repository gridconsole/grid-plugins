# Lane P-usage-direct-read

Grid reads every Claude and Codex account's 5h and 7d windows itself, with no status line and no session first (owner decision 3a, 17 Nov).

Base: `origin/main` at 82a26411 (merged last). Branch: `lane-P-usage-direct-read`.
All commands below were run in the lane clone on the final tree, after the last merge of `origin/main`.

## Proof table

### Acceptance rows

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | Claude windows read from `get_usage`, no status line, no session; test with the recorded shape | HOLDS | `TestUdClaudeWindowsFromGetUsageWithoutStatusLine` (`internal/account/ud_direct_claude_test.go:186`) runs a fake `claude` on PATH that answers the recorded `control_response` (signed in: `five_hour` and `seven_day` with `utilization` and ISO `resets_at`, many unknown keys) and checks the argv, the one stdin line, the empty cwd, no status-line file and `source:"direct"` with 5h 53 and 7d 21.5. Code: `direct_claude.go:14` (argv), `:18` (request), `:31` (`claudeUsage`), `:104` (`parseClaudeUsage`). Signed out, error reply, garbage, exit 1 and unknown or missing keys: `:226`, `:255`. `go -C . test -count=1 ./internal/account/` exit 0 |
| 2 | Codex windows read from `app-server`; retry on a first error; test | HOLDS | `TestUdCodexWindowsFromAppServer` (`ud_direct_codex_test.go:74`), `TestUdCodexMapsWindowsByDuration` (`:100`, 300 is 5h, 10080 is 7d, whichever of primary or secondary), `TestUdCodexRetriesOneErrorReply` (`:112`: error, then success, one result), `TestUdCodexTwoErrorRepliesGiveAReason` (`:123`). Code: `direct_codex.go:45` (`codexUsage`, two tries with `retryWait` between), `:197` (`reading`). Same `go test` exit 0 |
| 3 | Source order and cache as in Do 3; direct beats an older note, a newer note beats direct, failure keeps the last good with `asOf` | HOLDS | Order: `windows.go:156-176` (`windowsFor`), `:188` (`sessionRecord`). Tests: `TestUdOrderDirectBeatsAnOlderNote` (`ud_direct_claude_test.go:293`), `TestUdOrderNewerNoteBeatsDirect` (`:305`), `TestUdOrderNoteThenNoneWhenTheDirectReadFailed` (`:325`), `TestUdCacheGapBackOffAndLastGood` (`:342`: one probe per 2 minutes, 5 minutes back-off, last good kept with its `asOf`), `TestUdResetPassedZeroesADirectWindow` (`:380`), `TestUdOneProbePerFolderAtATime` (`:451`). Codex: `ud_codex_rollout_test.go:22`, `:35`, `:54`. Constants: `direct.go:26-30`. Same `go test` exit 0 |
| 4 | A hanging probe is killed at the timeout and does not block other folders; test | HOLDS | `TestUdHangingProbeIsKilledAndDoesNotBlockOtherFolders` (`ud_direct_claude_test.go:393`: two hung folders cost one timeout, the folder that answers is not held up, and the `sleep` each hung fake started is gone afterwards, so the whole process group was killed), `TestUdCodexHangIsKilledAndKeepsTheLastGood` (`ud_direct_codex_test.go:142`), `TestUdCodexHangWithNoLastGoodSaysTimedOut` (`:163`). Kill: `direct_proc.go:118` (`stop`), process group on Unix (`direct_proc_unix.go:14`), job object through `proctree` on Windows (`direct_proc_windows.go:13`). Folders are read side by side in `registry.go` (`Windows`, `maxProbes`). Same `go test` exit 0; `go test -race ./internal/account/` exit 0 |
| 5 | One folder or one account id gives one row; test with the two names seen | HOLDS, with a deviation on the names | `TestUdTwoRecordsOfOneLoginGiveOneRow` (`ud_fold_test.go:43`), `TestUdTheSignedInFolderRepresentsTheLogin` (`:71`), `TestUdUnregisteredFolderOfARegisteredLoginAddsNoRow` (`:85`), `TestUdTwoRecordsOfOneFolderGiveOneRow` (`:100`), `TestUdDifferentLoginsAndKindsStayApart` (`:115`). Code: `registry.go:216` (`foldRecords`: same kind and same id, same folder, or same login email). The names: the brief names two real ids from the Mac; this repository is public, so the tests keep their shape with an example login (`claude-pat-lee` and `claude-pat-lee-example-com`, `pat.lee@example.com`). The real strings are in history only (see open items). Same `go test` exit 0 |
| 6 | No token read or printed (grep the diff); diff limited to the allowed places; `make lint` and `make cross` exit 0; `make dist` builds | HOLDS, one file outside the list named below | Token grep on the final diff: see "Token grep". `make -C . lint` exit 0 (`go vet ./...`). `make -C . cross` exit 0 (vet and test compile for darwin/arm64 and windows/amd64, run alone). `make -C . dist` exit 0 (darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64 built). Changed files: `git diff --name-status origin/main...HEAD` lists `internal/account/*` (code and tests), `contract/schemas/cmd-account-windows.json` and this doc; nothing else |

### Do items without an acceptance row

| Do | Result | Proof |
|----|--------|-------|
| 4, cause of the two names | HOLDS | Before this lane `describe` and `Windows` (`registry.go`) listed every registry record and every scanned folder as a row, with no merge. The registry can hold twins of one login: ids come from `slugOf` (`registry.go:139`, the part before the `@`), and `newID` (`:165`) adds a suffix on a clash, so an older record and a later one made for the same login by another path (a person's `account add`, or another spelling of the email) both stay. `registerNew` (`:183`) skips an email it already holds, so it does not make a twin by itself. I did not have the Mac's registry file: the cause is read from the code and reproduced with two records and two folders of one login (red below), not seen on the Mac. The fold happens on read; the registry file is left as it is (`ud_fold_test.go:60` checks the bytes are unchanged) |
| 5, fake `claude` and `codex` scripts | HOLDS | Signed in, signed out, error then success, hang past the timeout, unknown keys: the tests above, all with shell-script stand-ins on a temp PATH (`udFakeClaude`, `udFakeCodex`) |

### Rules

| Rule | Result | Proof |
|------|--------|-------|
| No sudo, no root | HOLDS | No command in the run used `sudo` or a root shell |
| Never push | HOLDS | No `git push`. The work leaves as a bundle only. `git fetch origin` was run (read only) |
| Never touch `/srv/gmf`, `/root/.grid`, other lanes' clones | HOLDS | Paths used: this clone, `/srv/gridwork/tmp/hand` (the bundle and the report), a scratch worktree of this clone in `/tmp` (removed). `origin` is read by fetch |
| No `pkill -f` | HOLDS | Never run. The tests kill a probe by its own pid and process group |
| Never remove a feature | HOLDS | `git diff --numstat origin/main...HEAD`: 59 lines deleted in all, the old bodies of `windowsFor` (`windows.go`) and of `describe` and `Windows` (`registry.go`), each reworked in place. The status line note, the stream note, the Codex rollout record and the per-id read all remain, as the second and later sources |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | No hook, no check before or after an agent's tool call, nothing limits, delays or blocks a session. The per-folder gap, back-off, one-probe-at-a-time lock, `maxProbes = 8` and the 30 s timeout limit Grid's own usage probes, which Do 3 asks for; a probe is not an agent |
| One command per Bash call, no `;` chains, no redirects or tee, no `sed -i`, no `cd x && ...` | HOLDS | Process rule; commands used `git -C`, `go -C`, `make -C`; `&&` was not needed and pipes went only into `grep` |
| Small commits, one-sentence messages | HOLDS | `git log --format=%s origin/main..HEAD`: eleven commits with the one that adds this doc, each one sentence (three are merges of `origin/main`, the last with git's own one-line merge message) |
| No attribution lines | HOLDS | `git log --format=%B origin/main..HEAD` piped into a grep for `co-authored`, `claude-session`, `generated with`, `claude code`, `noreply`: no line |
| Public repo: no personal path, host or email | HOLDS for the diff, one item in history | See "Public repo grep" and the open items |
| New tests fail on base, pass on the lane | HOLDS | See "Red on base" |
| Tests for changed packages and every importer | HOLDS | Changed: `internal/account` (exit 0, 6.8 s), `contract` (exit 0, schema file). Importer: `go list` shows only `internal/cli` imports `internal/account`; `go test -count=1 ./internal/cli/` exit 0 (134 s) |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0. Both rerun after the last `git fetch origin` and `git merge origin/main` |
| Windows and macOS builds | HOLDS | `make cross` and `make dist`, above |
| Whole-module test, cross-repo set | not run, not asked | The orchestrator runs the whole module and the cross-repo set |
| `make test`, RealPlugin tests | not run, not asked | Not named in the brief for this lane |

## Red on base

The 24 tests of commit a8d5c495 (tests only, no code) run at that commit with `go test -count=1 ./internal/account -run TestUd`: 22 fail, 2 pass. Of the 2, `TestUdDifferentLoginsAndKindsStayApart` and `TestUdOrderNoteThenNoneWhenTheDirectReadFailed` hold on base by design: they guard what must not change. Failing lines (the real login in the fold lines is shown as `<login>`):

```
--- FAIL: TestUdClaudeWindowsFromGetUsageWithoutStatusLine
    ud_direct_claude_test.go:197: want source direct with 2 windows: {ID:claude Kind:claude Windows:[] AsOf: Source:none Note:No use reported for this folder yet}
--- FAIL: TestUdClaudeFailuresGiveAReasonNotZero/signed_out
    ud_direct_claude_test.go:242: want none with a note naming "signed out": {... Source:none Note:No use reported for this folder yet}
--- FAIL: TestUdOrderDirectBeatsAnOlderNote
    ud_direct_claude_test.go:302: want source direct with 2 windows: {ID:claude Kind:claude Windows:[{Label:5h Used:77 ...}] AsOf:2026-11-11T11:00:00Z Source:statusline Note:}
--- FAIL: TestUdHangingProbeIsKilledAndDoesNotBlockOtherFolders
    ud_direct_claude_test.go:405: two hung folders cost one timeout when read side by side, not two: 235.204627ms
--- FAIL: TestUdOneProbePerFolderAtATime
    ud_direct_claude_test.go:473: two readers, one probe: 0
--- FAIL: TestUdCodexWindowsFromAppServer
    ud_direct_codex_test.go:80: want source direct with 2 windows: {ID:codex Kind:codex Windows:[] AsOf: Source:none Note:Codex use is not read yet}
--- FAIL: TestUdCodexRetriesOneErrorReply
    ud_direct_codex_test.go:117: want source direct with 2 windows: {ID:codex ... Source:none Note:Codex use is not read yet}
--- FAIL: TestUdCodexHangIsKilledAndKeepsTheLastGood
    ud_direct_codex_test.go:146: want source direct with 2 windows: {ID:codex ... Source:none Note:Codex use is not read yet}
--- FAIL: TestUdTwoRecordsOfOneLoginGiveOneRow
    ud_fold_test.go:50: one login is one account row: [{ID:claude-<short> ... Email:<login> ...} {ID:claude-<short>-<domain> ... Email:<login> ...}]
--- FAIL: TestUdTwoRecordsOfOneFolderGiveOneRow
    ud_fold_test.go:104: one folder is one row: [{ID:claude-<short> ...} {ID:claude-<short>-<domain> ... Dir:<the same folder>}]
FAIL	github.com/gridconsole/grid-plugins/internal/account	1.537s
```

All 22 failing tests: the 12 Claude direct and order tests, the 6 Codex direct tests, the 4 fold tests (the list above shows one line of each kind).

The 3 Codex rollout tests (`ud_codex_rollout_test.go`, commit c231710c) were written after the merge with P-codex-windows-read, which put the rollout record into the code. Their base is the lane's `windows.go` as it was before that merge (7afc2e64), put into the merged tree. Two fail, one passes in both (an older rollout loses to the direct reading, which the old order already did):

```
--- FAIL: TestUdCodexNewerRolloutBeatsDirect
    ud_codex_rollout_test.go:48: want source codex with 2 windows: {ID:codex Kind:codex Windows:[{Label:5h Used:12.5 ...} {Label:7d Used:40 ...}] AsOf:2026-11-11T12:00:00Z Source:direct Note:}
--- FAIL: TestUdCodexRolloutAnswersWhenTheDirectReadFailed
    ud_codex_rollout_test.go:61: want source codex with 2 windows: {ID:codex Kind:codex Windows:[] AsOf: Source:none Note:Codex answered the usage request with an error}
```

On the lane head all 27 `TestUd` tests pass (`go test -count=1 ./internal/account/` exit 0; with `-race` exit 0, 26 s). I did not edit any existing test.

## Token grep

`git diff -U0 origin/main...HEAD -- internal contract` piped into a grep for `token|secret|bearer|credential|authorization|api_key|apikey|oauth|auth.json`: 18 hits.

- Code (2): a comment in `direct.go` saying the cache keeps no token, and `direct_claude.go:42`, which names `ANTHROPIC_API_KEY` and `ANTHROPIC_AUTH_TOKEN` in the list of variables dropped from the child's environment. Nothing reads or prints a token; no auth file is opened. stderr of the child is discarded and every reason is a fixed text line.
- Tests (16): fake values on purpose (`SECRET-ERROR-TEXT`, `SECRET-IN-REPLY`, `SECRET-CODEX-ERROR`, `sk-test-must-not-reach-the-child`, an `auth.json` of the fake id-token helper) to prove the reverse: an error text, a token in a reply or an API key in the environment never reaches a note, the cache file or the child.

## Public repo grep

The same diff piped into a grep for `/home/|/Users/|/srv/|/root|C:\|@host|abdu|gmail`: only `example.com` addresses (`ann@`, `pat.lee@`, `other@`, `a@`, `b@`). No personal path or host.

## What changed

- `internal/account/direct.go`, `direct_claude.go`, `direct_codex.go`, `direct_proc*.go` (new): the direct read of one folder. Claude: `claude -p --verbose --setting-sources local --input-format stream-json --output-format stream-json --no-session-persistence` in an empty temp cwd, `CLAUDE_CONFIG_DIR` only for a folder other than `~/.claude`, one `get_usage` control request on stdin, then stdin closed; `five_hour` is 5h, `seven_day` is 7d, `utilization` 0-100, `resets_at` to Unix seconds; unknown and missing keys tolerated. Codex: `codex app-server`, `initialize`, `initialized`, `account/rateLimits/read`, windows by `windowDurationMins`, one retry of an error reply after a short wait. Cache per folder in the Grid home (`accounts/windows/<hash>.direct.json` and a lock): at most one probe per 2 minutes, 5 minutes back-off after a failure, last good kept with `asOf`, one probe per folder at a time, 30 s timeout that kills the child (its process group on Unix, a job object on Windows).
- `internal/account/windows.go`: the source order: direct, then a status line, stream or Codex rollout record only when it is newer than the direct reading, then none with the reason. New source value `direct`.
- `internal/account/registry.go`: `foldRecords`, `groupAccounts`: one row per folder, per account id or per login (same kind); `Windows` reads the folders side by side (at most 8 at once), so a hang costs one timeout.
- `internal/account/account.go`: `Env.ProbeTimeout` and `Env.ProbeRetryWait`, test overrides.
- `contract/schemas/cmd-account-windows.json`: `"direct"` added to the `source` enum. **The one file outside the brief's list.** It is the schema the command's output is held to (`internal/cli/contract_inventory_test.go:439` names it for `account windows`), and the command now prints `direct`, so the enum has to list it. The schema package tests pass with it (`go test ./contract/... ./internal/contract/` exit 0). I did not run the base without that line to show a red.
- Tests: `ud_direct_claude_test.go`, `ud_direct_codex_test.go`, `ud_fold_test.go`, `ud_codex_rollout_test.go`, `ud_main_test.go`.
- `ud_main_test.go` is a `TestMain` guard: the legacy stream tests (`yb_stream_test.go`) build their own `Env` and used to start the installed Claude Code, because `windowsFor` now runs the direct read; a test with no PATH of its own now gets stand-in `claude` and `codex` that exit at once. The helper children of the `TestGjRegistry` tests are left alone (`GJ_REGISTRY_CHILD`). No existing test was edited.

Decisions the brief did not settle:

- **Codex order.** Do 3 says: direct read, then a status-line or stream note only when newer, then the Codex rollout record, then none. I read it as: the rollout record, like the note, wins over the direct reading only when it is newer, and is the answer when the direct read has nothing (`ud_codex_rollout_test.go`). With no `GridHome` the direct read is an empty cache, so the per-kind default notes of P-codex-windows-read (`Codex use is not read yet`) still come out (`cx_codex_readings_test.go`, not edited).
- **Unreadable time.** A record time that cannot be read counts as the oldest there is (`madeAfter`, `windows.go:229`).
- **Env.** The Claude probe drops `ANTHROPIC_API_KEY` and `ANTHROPIC_AUTH_TOKEN` from the child's environment, since a key makes Claude Code report no subscription windows.

## NOT VERIFIED

- The real `claude` and the real `codex` were not run by this lane. The shapes in the fakes are the ones recorded in the orchestrator's run of 17 Nov (`R-usage-direct-read.md`, Q1, Q5, Q6). A change in the providers' replies after that is not covered.
- Nothing was run on Windows or macOS: `make cross` vets and compiles both, and `make dist` builds both. The Windows kill (job object through `proctree`) and the `CREATE_NEW_PROCESS_GROUP` start are not executed.
- Nothing UI-visible: the app that shows the windows was not run; the schema accepts `direct`, but whether the app does is not checked here.
- The cause of the two names on the Mac is read from the code and reproduced, not seen in the Mac's registry file.

## Open items

1. **The app must accept source `direct`** (the schema enum is updated here; the app's own check is not part of this repository).
2. **History carries the owner's real login.** Commit a8d5c495 (the first, tests only) added the real address and the two real ids in `ud_fold_test.go`; 3b62eeb7 replaced them with example names. The final diff has none, but the history of the branch does. This repository is public: squash the lane at landing (or rewrite a8d5c495), or the address reaches public history. The commit messages hold no address.
3. `maxProbes = 8` and the 2 minute, 5 minute and 30 s values are my choice inside the brief's numbers; the orchestrator may want them set elsewhere.

DONE
