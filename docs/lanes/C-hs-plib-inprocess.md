# C-hs-plib-inprocess: per-verb in-process answer

Base: 91fadeff87525f5859978740e0b66ce51f375e48 (origin/main). Code head: 09cb01fd. On the base, lib answered nothing per verb: lib/lib.go has only Call, Run, env, environment, key and tempRoots, and lib.go:31-35 only describes host-tied behaviour in prose.

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1 TestHsIpWalksEveryCommand | HOLDS | `go test -count=1 -v -run TestHsIp ./lib/ ./internal/cli/` exit 0: `walked 38 commands, 38 table entries`, plus PASS for the sub-tests `deleted_entry_is_reported` (a copy of the table without each verb in turn reports exactly that verb), `stray_entry_is_reported` and `an_argv_of_a_deleted_verb_fails_closed`. There is no expected-fail test. The test checks that the table and commands() hold the same set of names; it does not hardcode a count. |
| 2 TestHsIpBinaryOnly | HOLDS | Same run, PASS. All 16 argv from the brief answer false with a why. Also false: the same calls behind global flags, `-hold`, `--hold=true`, and `account note --then`. |
| 3 TestHsIpSafe | HOLDS | Same run, PASS. The 8 argv from the brief answer true, and so do `--member m feed state --json`, `lease take task --task T-01` and `help`. |
| 4 TestHsIpFailClosed | HOLDS | Same run, PASS. `[]`, `["nonsense"]` and `["task","nonsense"]` answer false, and so do `task` with no subverb, `--member` with no value, and `--json version`. |
| 5 TestHsIpLibExport | HOLDS | Same run: `ok .../lib`. It covers 3 binary-only, 3 safe and 3 fail-closed argv. `git diff origin/main --stat` before this doc was written: `internal/cli/hs_inprocess_test.go`, `internal/cli/inprocess.go`, `lib/hs_inprocess_test.go`, `lib/inprocess.go`, 4 files changed, 451 insertions. This doc is the fifth file; the builder runbook requires it. |
| 6 tests/lint/windows vet | HOLDS | `go test -count=1 ./lib/ ./internal/cli/` exit 0 (lib 8.5s, internal/cli 91.8s). `make lint` exit 0 after a gofmt commit. `GOOS=windows go vet ./lib/ ./internal/cli/` exit 0. |
| Fails on base | HOLDS | With the two production files stashed: `lib/hs_inprocess_test.go:24:18: undefined: lib.InProcess`, `internal/cli/hs_inprocess_test.go:12:32: undefined: hsIpVerb`, `... undefined: hsIpTable`, `... undefined: hsIpAnswer`, `FAIL [build failed]` for both packages. |
| Importers tested | HOLDS | Packages that are or import lib or internal/cli (from `go list -f '{{.ImportPath}} {{join .Imports " "}} ...' ./...`): cmd/grid, contract/formats, internal/account, internal/cli, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test -count=1` on all of them: exit 0 (cmd/grid has no test files). |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./cmd/grid ./lib/` exit 0; `GOOS=darwin go build ./cmd/grid ./lib/` exit 0; `GOOS=darwin go vet ./lib/ ./internal/cli/` exit 0. |
| `"watch"` grep | HOLDS | `git grep -n '"watch"' -- internal/cli` on the lane: no hits, so no watch entry was added. |
| lib.go and cli.go not edited; lib.Run unchanged | HOLDS | The diff stat above names only new files. lib.InProcess (lib/inprocess.go:12) calls cli.InProcess (internal/cli/inprocess.go:169), which reads the unexported hsIpTable. |
| Table unexported | HOLDS | hsIpTable, hsIpVerb, hsIpSub and hsIpAnswer are unexported. The only new export in internal/cli is the one-line accessor cli.InProcess, which lib needs because lib cannot reach an unexported symbol. |
| No guard, gate, cap or hold on agents | HOLDS | InProcess only answers a question. Nothing calls it on the run path, and lib.Run still runs any argv it is given. |
| Tests: no clock, sleep or home | HOLDS | The tests are pure functions over argv. They read no clock and no home, so they need no t.TempDir home. |
| Commits, attribution, public repo | HOLDS | 2 commits, each with a one-sentence message and no attribution lines. The repo files hold no personal paths, hosts or emails. |
| RealPlugin and UI rows | n/a | The brief has no RealPlugin or UI row. |

## What changed

- `internal/cli/inprocess.go`: hsIpTable has one entry per verb of commands(), and the walk keeps it complete. Verbs where it matters list every subverb, and an unlisted subverb fails closed: task, job, lease, feed, env, routine, worker, account, eval. Some subverbs are keyed by a flag: `lease take --hold` and `account note --then`. Each binary-only entry carries its why and the path:line it rests on.
- Global flags (`--member`, `--role`, `--lease-epoch`, `--grid-home`) before the verb are skipped the same way applyGlobalFlags skips them. A global flag with no value answers false.
- `help`, `-h` and `--help` as the verb, or as a listed verb's first argument, answer true because they only print usage.
- `task new` and `lease take --hold` stay binary-only even if C-hs-plib-call lands first and makes them owner-aware. Changing that answer needs a later brief.

## Answers beyond the brief's list (reviewer: please confirm)

The brief's list came from an exec, signal and pid grep over internal/. This lane found more host-tied sites and marked them binary-only, each with a cited line:
- `job verify` (jobcmd/verify.go:31) and `eval run` (evalcmd/command.go:155) are in the brief.
- **New:** the whole `agent` verb, not just `agent sync`. `agent check` waits on signals at agent.go:36 and probes host children (agent/runner.go:32). `agent sync` finds the plugin from the host's executable (agent_sync.go:17). The remaining agent settings subverbs go through runAgent and have no site; they are false only because the verb is whole.
- **New:** `account list`, `account status` and `account scan`. Each probes `claude auth status` / `codex login status` with os.Environ, the host environment (account/account.go:183, called from :214 and :270).
- **New:** `account note --then`, which runs a shell command as a child (account.go:200, account/statusline.go:125). Plain `account note` stays true.
- Left true: `env down` (it stops a recorded server and is not tied to the caller). Also left true: git child calls that read the repo (orchestrator/events.go:57, feed.go:306), which lib.go:27-28 already documents.

## NOT VERIFIED

Nothing in this brief needs a live screen. The app's in-process path (C-hs-run) does not read this answer yet.

DONE
