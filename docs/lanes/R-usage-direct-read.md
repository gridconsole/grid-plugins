# R-usage-direct-read: lane report

Research only. One file is added: `docs/research/usage-direct-read.md`. No code changed.

## The sandbox

This lane could read only `/srv/gridwork/**` and `/tmp/**`. Several things were refused (exact calls below):

- Bash on any path in the home folder:
  - `ls -la ~/.local/bin/ ...`
  - `ls /home/<lane user>/.local/bin`
  - `grep -c five_hour /home/<lane user>/.local/bin/claude`
- `claude` and `curl`: not on the allow list, and `curl` is denied outright.
- WebSearch and WebFetch: refused in dontAsk mode.

So the installed Claude Code and codex-cli on this VM, the live call (Q3) and the changelog (Q4) could not be read or run. The doc instead:

- quotes the 5-7 Nov GridV2 research notes from the hand folder, with the versions those notes name;
- cites lane code by `path:line`;
- marks everything else UNVERIFIED;
- lists the exact "Orchestrator runs" that settle each open item.

## Proof table

| Row | State | Proof |
|---|---|---|
| Q1 exact HTTP call (method, URL, headers, fields, version) | NOT DONE | Only the path `/api/oauth/usage` is known (5 Nov notes). The host, method, headers and installed version are UNVERIFIED because the binary is outside the readable paths. The no-credential route `get_usage` is given with its flags and reply schema (notes, 2.1.318/2.1.319). Doc section Q1. |
| Q2 credentials per folder, Keychain name, refresh | NOT DONE (partial) | Keychain service name and `.credentials.json` are from the notes; the hash input and the Keychain account are UNVERIFIED. The package's no-secret rule is at `internal/account/account.go:1-5,151-153`. Doc section Q2. |
| Q3 one live call on own account | NOT DONE | Bash refused home-folder paths, and `claude` and `curl` are not allowed. Per the brief, the orchestrator runs it: doc "Orchestrator runs" 3-4. |
| Q4 stability, changelog, failure handling | NOT DONE (partial) | Changelog not read (WebFetch refused). The experimental marking, 429, cache and back-off rules and the fallback order are given from the notes and lane code (`windows.go:131-179`). |
| Q5 Codex direct read vs rollouts | HOLDS for what P-codex covers, NOT DONE for the 0.167.0 check | P-codex coverage: `P-codex-windows-read/internal/account/codex_read.go:23-202`. `account/rateLimits/read` from the notes (0.162.0, used live at 18x-codex.md:303); not checked on the 0.167.0 installed here. |
| Q6 design for the build lane | HOLDS | Doc section Q6 and "Build brief inputs": placement, order, cache, trigger, macOS and Linux, fake-binary tests. It also raises one owner question: the `get_usage` probe or the direct OAuth call. |
| Doc committed, last line DONE or NOT DONE | HOLDS | `docs/research/usage-direct-read.md`; last line `NOT DONE`. Commit 351d60e2. |
| Copy to hand | HOLDS | `cp` to `/srv/gridwork/tmp/hand/R-usage-direct-read.md` |
| No code change, no guard added | HOLDS | `git diff --stat origin/main..HEAD` lists only the two docs files |
| No token, cookie, account id, personal path or email | HOLDS | `grep -n -e /Users -e /home/ -e @ -i -e abdu docs/research/usage-direct-read.md`: no output (exit 1) |
| Merge origin/main before last proof run | HOLDS | `git fetch origin` then `git merge origin/main`: already up to date at 2fdfe3eb |
| Walk test `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `ok` in 0.077s, exit 0 |
| Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | `ok` in 0.315s, exit 0 |
| Tests of changed packages and importers | not run, not asked | No package changed. |
| make lint, Windows and macOS builds | not run, not asked | No Go file changed. |
| Bundle | HOLDS | `git bundle create /srv/gridwork/tmp/hand/R-usage-direct-read.bundle origin/main..lane-R-usage-direct-read` |

## What changed

- `docs/research/usage-direct-read.md` (new)
- `docs/lanes/R-usage-direct-read.md` (new)

## Open items

1. The orchestrator runs "Orchestrator runs" 1-5 of the doc. These settle the UNVERIFIED items in Q1-Q5 and the signed-in `get_usage` shape.
2. The owner chooses between the `get_usage` probe (recommended) and the direct OAuth call.

NOT VERIFIED on a live screen: nothing; no UI changed.

NOT DONE
