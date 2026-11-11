# C-he-message-verb-walk: proof table

Base: 91fadeff87525f5859978740e0b66ce51f375e48 (main). Head (code commit): ff292f49.
One new file: `internal/contract/he_mv_message_verbs_test.go` (344 lines, no build tag, no production change).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. `TestHeMvMessagesNameRealSubs` walks all non-test Go files under cmd/ and internal/, passes, logs the pair count, includes `feed state` | HOLDS | `go test -count=1 -v -run 'TestHeMv\|TestGoReadmeSubverbsInCommandsJSON' ./internal/contract/` exit 0. Log: `he_mv_message_verbs_test.go:213: 221 pairs found (158 distinct file:pair) in the non-test Go files under cmd/ and internal/; 27 verbs with subs checked; 12 pairs are listed prose`. The test fails if `internal/cli/feed.go` names no `grid feed state` (it does, feed.go:23 and :28) or if `state` is not a sub of `feed` in commands.json. |
| 1b. Literals naming a sub commands.json lacks go in `heMvProse`, messages not edited | HOLDS | 12 entries at `he_mv_message_verbs_test.go:41-57`, named below. `git diff main --stat` shows the test file only. |
| 2. `TestHeMvAlternation` | HOLDS | `--- PASS: TestHeMvAlternation`. "grid feed post\|read\|fold" gives 3 pairs (feed post, feed read, feed fold); "grid plan <task>" gives none. Also covers `|`-tail, trailing punctuation, `<a\|b>`, `mygrid`, `/bin/grid`, line offset. |
| 3. `TestHeMvFinder` | HOLDS | `--- PASS: TestHeMvFinder`. t.TempDir with `pkg/a.go` (a real pair, an unknown pair, a verb with no subs, a comment naming a pair, a raw string) and `pkg/a_test.go` (unknown pair `feed zork`). The walk sees only the three pairs from a.go (lines 4, 5, 8); the `_test.go` pair is never seen; `heMvUnknown` returns `frobnicate` and `rawunknown` from `pkg/a.go` only. |
| 4. `TestHeMvProseListBothWays` | HOLDS | `--- PASS: TestHeMvProseListBothWays`. Mismatch run with two temporary bad entries (since reverted, file clean): `heMvProse entry "internal/cli/feed.go:feed gone" matches no string literal; remove it` and `heMvProse entry "internal/cli/feed.go:feed state" is a real sub of `grid feed` now; remove it`, exit 1. |
| 5a. `go test -count=1 -run 'TestHeMv\|TestGoReadmeSubverbsInCommandsJSON' ./internal/contract/` 0 fail | HOLDS | exit 0, `ok github.com/gridconsole/grid-plugins/internal/contract 0.829s` (5 tests PASS, listed above). |
| 5b. `git diff main --stat` names only the new file | HOLDS | At the code commit ff292f49: ` internal/contract/he_mv_message_verbs_test.go \| 344 +++`, 1 file changed. The head adds a second commit with this report, so the diff names the test file and `docs/lanes/C-he-message-verb-walk.md` (the runbook's proof table, committed the way earlier lanes do, for example 5bdc20c8); no other file. |
| 5c. `make lint` exit 0 | HOLDS | `go vet ./...` exit 0; `gofmt -l internal/contract` empty. |
| New tests fail on base, pass on lane | HOLDS | No production change, so "base" is the walk with an empty `heMvProse`. That run (first run, same file) failed with 12 unlisted pairs, for example: `internal/cli/agent.go:25: grid agent budget: "budget" is not a sub of grid agent in contract/commands.json`; `internal/cli/task.go:253: grid lease take-over: "take-over" is not a sub of grid lease`; `internal/braincmd/command.go:136: grid brain help: "help" is not a sub of grid brain`. After listing them the test passes. |
| No existing test edited or deleted | HOLDS | `git diff main --stat` shows one added file. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | Test file only. |
| Tests of changed package and importers | HOLDS | `go test -count=1` on the 26 packages that import internal/contract (artifact, brain, braincmd, cli, contract, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, question, session, settings, signal, task, usage, where, workflow): all `ok`, exit 0. |
| Windows and macOS builds | HOLDS | `make cross` (darwin/arm64 and windows/amd64: `go vet ./...` and a test compile of every package) exit 0. The new test uses `filepath.ToSlash` for paths. |
| No clock, no sleeps, temp home, no expected-fail tests | HOLDS | The test reads files only; the finder test uses `t.TempDir`; no `Skip`, no expected failures. |
| `*RealPlugin*` tests | n/a | None in this brief. |

## Pairs listed in `heMvProse` (the literals are not edited)

Named subs commands.json does not have:
- `internal/cli/task.go:253` `grid lease take-over` (no dispatcher has it).
- `internal/cli/agent.go:25`, `internal/cli/budget.go:24` `grid agent budget` (dispatched at agent.go:21, not in commands.json).
- `internal/cli/budget.go:91` `grid claude settings` (dispatched at budget.go:90, not in commands.json).
- `internal/braincmd/{changes,command,supersede}.go` `grid brain help` (answered at braincmd/command.go:112, not in commands.json).
- `internal/job/render.go:237` `grid brain topic` (no dispatcher has it; `--topic` is a flag of other subs).

Plain English after a verb name:
- `internal/guard/host.go:17` `grid task or`, `grid question command`; `host.go:21` `grid feed or`; `internal/guard/writer.go:40` `grid project to`.

## What changed

Only the new test file. It parses each non-test .go file under cmd/ and internal/ with go/parser, takes each string literal (comments are not read), finds `grid <verb> <sub>`, expands `a|b|c`, and checks each pair against contract/commands.json. Verbs with no named sub, and verbs not in commands.json, are skipped. Helpers are `heMv*`, tests are `TestHeMv*`, the commands.json reader is its own `heMvDoc` (no dependency on the names in `go_readme_subverbs_flags_test.go`).

Limits (not covered): a call split across string concatenation (`"grid feed " + sub`) or across lines is not joined; only the word right after the verb is checked, not a third word; a verb that commands.json lacks altogether is left to the top-level checks.

Two notes for a reviewer counting: the walk skips directories named `testdata` and `vendor` (the Go tool ignores testdata too; there is no vendor folder to read). The row 3 fixture is a superset of the row: it holds two unknown pairs (`frobnicate` and one in a raw string) plus a no-sub verb and a comment, so the check is that nothing comes from `a_test.go` and everything unknown comes from `a.go`.

## NOT VERIFIED on a live screen

Nothing here is UI. No row needs a live screen.
