# C-gj-account-registry-two-processes

Two processes registering accounts at once lose no account and double none. Repo: plugin. Base: `b8324c53a776fefe1faadb5b7094d3a4e6f7598a` (lane-Pint92 merge). Test commit: `39b331fc`.

Base check: read `internal/account/registry.go:85-275` and `account_test.go:1-111` on that sha. No test on the base starts two processes on one registry (`grep -rln 'os.Args\[0\]' internal/account/` printed nothing). The lock holds across processes on the base, so no production file changed.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. TestGjRegistryTwoProcessesOneEmail | HOLDS | `go test -count=1 -race -v -run TestGjRegistry ./internal/account/` exit 0: `--- PASS: TestGjRegistryTwoProcessesOneEmail (1.48s)`. Asserts: both children exit 0, one record (ann@example.com, `claude-ann`, dir `~/.claude-ann`), `~/.claude-ann` exists, `~/.claude-ann-2` does not, both children printed `[claude-ann]`. gj_registry_processes_test.go:267 |
| 2. TestGjRegistryManyProcessesManyEmails | HOLDS | `--- PASS: TestGjRegistryManyProcessesManyEmails (3.39s)`. 6 children at once; 4 records, 4 distinct emails (case-folded), ids, dirs; ids are exactly claude-ann, claude-dan, codex-bob, codex-cat; every child printed those four; no `-2` folder; a 7th run leaves registry.json byte-equal. :291 |
| 3. TestGjRegistryReaderWhileWriting | HOLDS | `--- PASS: TestGjRegistryReaderWhileWriting (8.19s)`, subtests round1..round5 PASS, fresh world each. 4 register=true + 4 register=false interleaved: all exit 0, no child error, no "is not readable JSON", each reader printed nothing or the whole list (never a part), no `.registry.json.*` left in accounts. :344 |
| 4. Home rule | HOLDS | `gjRegNewWorld` (:60, Setenv from :73) t.Setenv's HOME, CLAUDE_CONFIG_DIR, CODEX_HOME, GRID_HOME (and FAKE_HOME) under `t.TempDir()` and t.Fatal's when HOME or GRID_HOME is not under that root; the child checks the same and refuses otherwise (:126). `gjRegWorld.env` (:87) gives every child the same keys with PATH = the fake bin, built from scratch (nothing taken from the parent's environment). `grep -n 'Sleep' internal/account/gj_registry_processes_test.go` exit 1, prints nothing. Barrier: each child blocks on stdin; `gjRegRun` closes every pipe only after every Start returned (:182). No log prints an email or a token. |
| 5a. 3 runs of the set | HOLDS | `go test -count=1 -race -run 'TestGjRegistry\|TestStatus\|TestTwoEmails' ./internal/account/` three times: `ok .../internal/account 15.061s`, `ok .../internal/account 14.512s`, `ok .../internal/account 14.382s` (exit 0 each) |
| 5b. make lint | HOLDS | `make lint && echo LINT_EXIT=0` printed `go vet ./...` then `LINT_EXIT=0` (vet + gofmt -l clean) |
| 5c. diff stat | HOLDS | `git diff main --stat` on the test commit: `internal/account/gj_registry_processes_test.go | 419 +` and nothing else. This report is a second commit (`docs/lanes/...md`) as the runbook asks; it is the only other file. |
| Windows + macOS builds | HOLDS | `make cross && echo CROSS_EXIT=0` printed `CROSS_EXIT=0`: vet and test-compile of every package for darwin/arm64 and windows/amd64. The tests skip on Windows (the fake provider is a shell script). |
| Importers' tests | HOLDS | `go list` of importers (imports and test imports of `internal/account`): only `github.com/gridconsole/grid-plugins/internal/cli`. `go test -count=1 ./internal/account/ ./internal/cli/` exit 0: `ok .../internal/account 7.189s`, `ok .../internal/cli 60.696s` |
| New tests fail on base | PARTLY, see below | Base is correct, so the three brief rows pass on base; they cannot fail there. One added test fails when the lock is removed (below). |
| No existing test edited, none deleted | HOLDS | `git diff main --stat` shows one added file. |
| No guard, gate, cap or hold on agents | HOLDS | test file only. |
| Clean tree | HOLDS | `git status --short` after the mutation was reverted showed only the new file (untracked, then committed). |
| `*RealPlugin*` tests | n/a | the brief names none; the plugin build was not needed. |

## What changed

One new file, `internal/account/gj_registry_processes_test.go` (tests and helpers; helpers `gjReg*`, tests `TestGjRegistry*`):

- `TestGjRegistryChild`: the helper process. Skips unless `GJ_REGISTRY_CHILD=1`. Builds an `Env` from HOME and GRID_HOME, waits for stdin to close, calls `Status(env, register)` once (register from `GJ_REGISTRY_REGISTER`), prints the ids as one JSON line, exits non-zero on an error.
- The three brief tests (rows 1-3).
- One added test, `TestGjRegistryEachProcessBringsItsOwnEmail`: six children, each with its own `CLAUDE_CONFIG_DIR` folder carrying its own email, beside `~/.claude` as ann. Expect 7 records, none lost. Added because the brief's static fixtures cannot tell a locked write from an unlocked one (see below).

No production file changed.

## New tests on base: what fails and what cannot

The brief rows 1-3 use one fixed set of folders, so every process reads the same disk and, locked or not, writes the same list. They hold on the correct base and are regression guards. To see whether the set can fail at all, I replaced the body of `lockRegistry` (registry.go) with a no-op, ran the set, and restored the file with `git checkout -- internal/account/registry.go` (`git status --short` clean after). This edit is outside the mismatch-proof allowance in the rules; I did it only to show that the tests can fail, and it never reached a commit.

With the lock removed, `go test -count=1 -race -run TestGjRegistry ./internal/account/` (one run):

```
--- FAIL: TestGjRegistryEachProcessBringsItsOwnEmail (1.46s)
    gj_registry_processes_test.go:407: an account was lost or doubled: registry ids [claude-ann claude-p1 claude-p2 claude-p3], want [claude-ann claude-p0 claude-p1 claude-p2 claude-p3 claude-p4 claude-p5]
FAIL
```

Rows 1-3 still passed on that mutation. With the lock restored the same test passes (three runs above).

## NOT VERIFIED

- Nothing here needs a live screen; this lane has no UI.
- The lost-account failure above is probabilistic: it showed up on the one mutated run I made; I did not repeat it.
- `TestGjRegistryChild` shows as SKIP in a `-v` run of the parent. It is the helper, not a row; the rows' children run it with the marker set and print their result.
- The skill `go-conventions` named in the brief is not in my skill list; I followed the repo's own style (gofmt, `go vet`).
