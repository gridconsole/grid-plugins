# How Grid reads each Claude and Codex account's usage directly

Lane R-usage-direct-read, 17 Nov 2026. Research only, no code change. Owner decision 17 Nov (mac findings, section 3a): usage must not depend on the status line.

## What this lane could and could not read

The lane ran with these permissions: read access to the build area and /tmp only, no `claude` or `codex` command, `curl` denied, WebSearch and WebFetch denied. Bash calls naming a path in the lane user's home were refused, among them `ls ~/.local/bin` and `grep five_hour ~/.local/bin/claude`. As a result:

- the installed Claude Code and codex-cli on this VM were **not read** by this lane (Q1, Q2, Q4, Q5);
- the live call on the lane user's own account was **not run** (Q3);
- no changelog or doc URL was read (Q4).

I did not work around these refusals with `node` or `go run`. Every claim below carries one of three tags:

- **[lane]**: read in this lane, with `path:line`.
- **[notes]**: quoted from the earlier research notes of the GridV2 task (5-7 Nov 2026, on the owner's Mac, not this VM), with the Claude Code or Codex version those notes name. They live in the hand folder under `realtasks/gridv2/tasks/T-01-grid-on-claude/`:
  - `app-steps/gaps-5nov/account-usage-all.md`, cited as AUA;
  - `brain-proposals/claude-get-usage-without-a-credential.md`, cited as GU;
  - `subagents/721-account-usage-core.md`, cited as 721;
  - `subagents/731-review-account-usage.md`, cited as 731;
  - `app-steps/18x-codex.md`, cited as 18X.
- **UNVERIFIED**: neither of the above. The section "Orchestrator runs" lists the command that settles each one.

## Q1. How Claude Code's /usage gets the 5-hour and weekly windows

**Answer:** Claude Code reads the windows from Anthropic's OAuth usage endpoint `/api/oauth/usage`. It also exposes the same data without a session turn through its `get_usage` stream-json control request. This lane could confirm neither the exact HTTP request (host, method, headers, user agent) nor the installed version on this VM.

### The HTTP call (Claude Code reading it itself)

- [notes] The path is `/api/oauth/usage` and it needs the account's OAuth token (AUA:44). Those notes give no host, method, header or user agent.
- UNVERIFIED: the host (`api.anthropic.com` is expected), the method (GET is expected), the auth header (expected to be `Authorization: Bearer <OAuth access token>`), any `anthropic-beta` header and the user agent. Settle them with "Orchestrator runs" 1-2.

### The same data through Claude Code, with no turn and no credential read by Grid

[notes] Claude Code 2.1.318 strings, run on 2.1.319 (AUA:45-49, GU:14-19):

- **Command:**

  ```
  claude -p --verbose --setting-sources local --input-format stream-json --output-format stream-json --no-session-persistence
  ```

  - cwd is a fresh empty temp folder.
  - stdin gets one line and is then closed:

    ```
    {"type":"control_request","request_id":"usage","request":{"subtype":"get_usage","skip_behaviors":true}}
    ```

- **Flags that matter:**
  - `--verbose` is required with `-p --output-format stream-json`; without it the command exits 1 (GU:15, 721:62).
  - `--setting-sources local` keeps the account's and the cwd's SessionStart hooks from running. Without it they ran, checked with a touch-file hook (GU:16).
- **Reply:** a `control_response` line with the same `request_id`. `system` lines may come before it. `response.response` holds:
  - `subscription_type`
  - `rate_limits_available` (bool)
  - `rate_limits`: null or an object with
    - `five_hour`, `seven_day`, `seven_day_opus`, `seven_day_sonnet`: each `{utilization, resets_at}`;
    - `model_scoped`: a list of `{display_name, utilization, resets_at}`;
    - `extra_usage`.
  - `utilization` is a percent from 0 to 100 (schema text "Percentage of the window used, 0-100").
  - `resets_at` is an ISO-8601 string or null.
- **Status:** the binary marks the request "Experimental, shape may change" (AUA:49).
- **Observed so far:** only the signed-out reply: `subscription_type:null, rate_limits_available:false, rate_limits:null`, exit 0 in about 2 s (AUA:49). The signed-in shape comes from the schema and was never observed (721:72).

### Units Grid already handles

[lane] Three sources, three conventions:

| Source | Use | Reset time |
|---|---|---|
| Status line (`windows.go:17-18, 104-129`) | `used_percentage`, 0-100 | `resets_at`, Unix seconds |
| Stream `rate_limit_event` (`yb_stream.go:83-94`) | `utilization` as a fraction 0-1, multiplied by 100 at `yb_stream.go:94` | `resetsAt`, Unix seconds |
| `get_usage` ([notes]) | 0-100 | ISO string, which must be parsed into `Reading.ResetsAt` (Unix seconds) |

GU:18 says the status line uses fractions 0-1. That is wrong for the status line; it holds for the stream event.

**Version read:** none on this VM. The notes name Claude Code 2.1.312 (18X:10), 2.1.318 (AUA:49) and 2.1.319 (GU:14).

## Q2. Where the OAuth token lives per account folder, and refresh

- [notes] **macOS:** the Keychain service is `Claude Code-credentials` for the default folder, and `Claude Code-credentials-<first 8 hex of sha256(config dir)>` when `CLAUDE_CONFIG_DIR` is set (AUA:44). Setting `CLAUDE_CONFIG_DIR` for the machine's own sign-in changes the service name, so that sign-in is not found (AUA:50, GU:17).
  - UNVERIFIED: whether the hash input is the path as given or the resolved path, and which value is used as the Keychain account. Settle them with "Orchestrator runs" 2.
- [notes] **Linux and other non-Keychain systems:** the token is in `<config dir>/.credentials.json` (AUA:44).
  - [lane] Grid's tests use the shape `{"claudeAiOauth":{"accessToken":...}}` (`internal/account/account_test.go:315`).
  - UNVERIFIED: the field names for expiry and refresh token.
- [notes] **Refresh rotates the token.** If Grid refreshed it, the stored refresh token would be replaced and Claude Code signed out of that folder. The endpoint also returns 429 hard, and that budget is shared with the account's own sessions (AUA:44). The rotation claim is from the notes, not read in a bundle here.
- [lane] **Package rule:** `internal/account` says it "never copies, writes or logs a secret" (`internal/account/account.go:1-5`). `Scan` "never opens .credentials.json or the Keychain item; `claude auth status` may, which is Claude Code reading its own store" (`internal/account/account.go:151-153`).
  - A direct OAuth call from Grid would break that rule.
  - The `get_usage` probe keeps it: Claude Code reads and refreshes its own token.

## Q3. Live check on the lane user's own account

**NOT DONE.** No `claude` or `curl` command is allowed, and Bash refused every path in the home folder, so neither the HTTP call nor the `get_usage` probe could run. Per the brief, the orchestrator runs it: "Orchestrator runs" 3 (the probe) and 4 (the HTTP call). Record keys and value types only, with every token, id and email redacted.

## Q4. Stability

- [notes] `get_usage` is marked experimental in the binary (AUA:49). `/api/oauth/usage` is an internal endpoint with no public documentation (AUA:44).
- UNVERIFIED: what changed across recent Claude Code versions. The changelog could not be fetched; WebFetch was refused.
- [notes] **Rate:** the endpoint returns 429 hard (AUA:44). The design in the notes reads each account at most once per 2 minutes, backs off 5 minutes after any failure, and keeps the last good windows with their time (AUA:52, 721:54). The review found the cache, back-off and one-probe-per-account rules met (731:48).
- **On failure:**
  - [notes] Parse tolerantly. An unusable answer gives the reason "usage cannot be read from this Claude Code version" (AUA:79, 721:69).
  - Then use the status-line note when it is fresher (`readSnapshot`, [lane] `windows.go:131-144`).
  - Then use source `none` with a one-line note ([lane] `windows.go:149, 177`), never 0.

## Q5. Codex

- [lane] **What P-codex-windows-read covers.** It reads `<CODEX_HOME>/sessions/**/rollout-*.jsonl`:
  - the newest `token_count` line with `rate_limits.primary/secondary {used_percent, window_minutes, resets_at}`;
  - windows mapped by length (360 minutes or less is 5h, 1440 or more is 7d);
  - "premium" lines with null windows are skipped;
  - source `codex` (`P-codex-windows-read/internal/account/codex_read.go:23-43, 137-202`).

  Its facts are from codex-cli 0.167.0 on this VM (`briefs/P-codex-windows-read.md:8-14`). It needs at least one earlier Codex session on that folder, and the reading is only as fresh as the last turn.
- [notes] **Direct read without a session record:** `codex app-server` (JSON-RPC over stdio; `initialize`, then `account/rateLimits/read`, with `CODEX_HOME=<folder>`) returns `rateLimits.primary/secondary {usedPercent, windowDurationMins, resetsAt (unix s)}` (AUA:51, codex-cli 0.162.0).
  - It was used live on a real account and read 0% before and after two paid runs (18X:303).
  - `CODEX_HOME` scopes the sign-in (18X:309).
  - app-server is labelled experimental (18X:41).
- **What a direct read adds:** a reading for a Codex account with no session yet, and a fresh one after the windows reset. Like `get_usage`, Codex reads its own token; Grid opens no `auth.json` token.
  - [lane] Today `readCodex` reads only the email and plan claims of `id_token` (`account.go:281-300`).
- UNVERIFIED:
  - whether 0.167.0 (installed here) still answers `account/rateLimits/read` with the same field names;
  - which HTTP endpoint codex-cli calls behind it.

  Settle them with "Orchestrator runs" 5.

## Q6. Proposed design for P-usage-direct-read

### Route: the owner question

The owner's text says Grid reads the "OAuth usage call". The earlier research rejected calling it from Grid (token rotation, 429, credential handling) and chose the `get_usage` control request (AUA:44-45, 73). This doc recommends `get_usage`.

| | `get_usage` probe (recommended) | Direct HTTP call |
|---|---|---|
| No setup by the person | yes | yes |
| No Claude session first | yes | yes |
| Grid reads a secret | no | yes |
| Grid can sign Claude Code out (token rotation) | no | yes, if Grid refreshes the token |
| Experimental | yes | internal, undocumented |
| Needs `claude` found on PATH | yes (Mac finding 2: the GUI PATH misses `~/.local/bin`, `account.go:205` uses `exec.LookPath`) | no |

**Owner question for the build brief:** "Read Claude usage through Claude Code's own `get_usage` request (recommended: Grid never touches a token), or call the OAuth endpoint with the token Grid reads from the Keychain or `.credentials.json`?"

### Placement

- **Where:** `internal/account`, a new `direct.go` beside `windows.go`.
- **Reader seam:** a `usageReader` field on `Env` (or a package var) so tests can swap it.
- **Claude reader:** `runProvider(env, "claude", args, "CLAUDE_CONFIG_DIR", dir, stdin, ...)`.
  - It reuses `providerEnv`: `CLAUDE_CONFIG_DIR` is set for an added folder and unset for the home folder (`account.go:185-199, 204-226`).
  - It needs:
    - a fresh empty temp cwd: `runProvider` sets no `cmd.Dir`, so add a parameter;
    - a 30 s timeout even with stdin: today the timeout applies only when `in == nil` (`account.go:210-214`);
    - no `ANTHROPIC_API_KEY` passed to the child.
- **Codex reader:** `codex app-server` with `CODEX_HOME`, `initialize`, then `account/rateLimits/read`, then stdin closed.

### Order of sources in `windowsFor`

For each account:

1. The direct read, cached.
2. The status-line note or stream snapshot when its `At` is newer than the direct read's `AsOf`.
3. For Codex, the rollout reading.
4. Source `none` with a one-line reason.

Source values: `direct` (new), `statusline`, `stream`, `codex`, `none`.

### Cache

- One file per folder under `<GridHome>/accounts/windows/`, beside the snapshot (`windows.go:55-58`), e.g. `<hash>.direct.json`, holding numbers and `at` only.
- A real read happens at most every 2 minutes per folder; after a failure, back off 5 minutes. The last good reading is kept with its `asOf`.
- At most one probe per folder at a time, using a lock file in the same folder.

### Trigger

- `grid account windows` already answers per account (`registry.go:277-326`).
  - It reads through the cache.
  - It starts a probe only when the cache is older than the gap, so the CLI answers stays bounded.
  - Add `--refresh` to force one read, still subject to the back-off.
- UNVERIFIED (the app repo was not read in this lane): the desktop and server reach the windows through `grid account windows --json` for the usage modal and Settings, so no new route would be needed. The mac findings (section 3) show the command's output in that modal. A page open triggers a read through the gap rule.

### Platforms

- **macOS:** Grid touches no Keychain item; Claude Code finds its own. For the home folder, never set `CLAUDE_CONFIG_DIR` (AUA:50).
- **Linux:** the same; Claude Code reads `.credentials.json` itself.
- **Windows:** the same probe; `providerEnv` already folds variable-name case (`account.go:182-190`).

### Tests (fake binary, never a real account, `~/.claude`, the Keychain or `~/.grid`)

- A fake `claude` script on a temp PATH. It:
  - checks the exact args;
  - checks the stdin request line;
  - checks that `CLAUDE_CONFIG_DIR` is set for an added folder and unset for home;
  - checks that the cwd is empty;
  - answers with recorded replies.
- Parser tests:
  - signed in, with `system` lines before the reply;
  - signed out (`rate_limits_available:false`);
  - `rate_limits:null`;
  - an unknown subtype or garbage, which gives the version reason;
  - an ISO `resets_at` converted to Unix seconds;
  - utilization 0-100 kept as is.
- Order tests:
  - direct newer than the note: direct wins;
  - note newer: statusline wins;
  - direct failed: the note, then `none` with the reason.
- Cache tests: gap, back-off, last good kept, a lock held by another reader.
- Timeout test: a fake that hangs is killed, and the answer is the reason line.
- Codex: a fake `codex` app-server answering `initialize` and `account/rateLimits/read`.

## Orchestrator runs (to settle the UNVERIFIED rows)

Each run uses the lane user's own account only.

1. Q1, Q4: `claude --version`, then `codex --version`.
2. Q1, Q2: run `grep -a -o` on the resolved `claude` binary for `oauth/usage`, `anthropic-beta`, `Claude Code-credentials` and `get_usage` with about 200 bytes of context. Record the host, method, headers, user agent, Keychain service and account derivation, and whether `get_usage` is still there.
3. Q3: the `get_usage` probe from Q1 in an empty temp folder on `~/.claude`. Keep `CLAUDE_CONFIG_DIR` unset. Record the reply's keys and value types only.
4. Q3: one GET of the usage endpoint with the account's token, headers taken from run 2. Record the status code and the keys and types of the body only; never print the token.
5. Q5: `CODEX_HOME=~/.codex codex app-server`, then `initialize` and `account/rateLimits/read`. Record the keys and types.

## Build brief inputs

- **Read:**
  - Claude through `claude -p --verbose --setting-sources local --input-format stream-json --output-format stream-json --no-session-persistence` plus a `get_usage` control request, in an empty temp cwd, with `CLAUDE_CONFIG_DIR` only for added folders;
  - Codex through `codex app-server` and `account/rateLimits/read` with `CODEX_HOME`.
- **Parse:** `utilization` 0-100; `resets_at` ISO to Unix seconds; `five_hour` to 5h and `seven_day` to 7d; tolerate missing keys; `rate_limits_available:false` gives the reason line.
- **Order:** direct, then a fresher status-line or stream note, then the Codex rollout, then `none` with a reason.
- **Cache:** 2-minute gap, 5-minute back-off, last good kept with `asOf`, one probe per folder at a time, a 30 s timeout that kills the child.
- **Code changes:** `runProvider` gains a cwd and a timeout with stdin. New source value `direct`. Grid never reads a token.
- **Open owner question:** `get_usage` probe or direct OAuth call (Q6).
- **Before building:** settle "Orchestrator runs" 1-5. The signed-in `get_usage` shape and the installed versions are UNVERIFIED.

NOT DONE
