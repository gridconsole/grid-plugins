# C-jz-feed-member-from-call

A lib Feed write with no GRID_MEMBER and no GRID_ACTOR in the Call finds its member from the
Call's git config and then the Call's USER and USERNAME, not the host process's.

Base: dc301fce757d6374de26d8d1b68de26575341b32 (lane-C-jz-feed-member-from-call at start).
Head: the tip of lane-C-jz-feed-member-from-call (the sha is in the hand-off message).

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. Call GIT_CONFIG_GLOBAL names `call@example.com`, host names `host@example.com`, no member in the Call, Dir outside a repo: author is `call@example.com` | HOLDS | `go test -count=1 -v -run 'TestJzMb' ./lib/` exit 0: `--- PASS: TestJzMbEmailFromCallGitConfig`. On base the same test fails: `t2_jz_member_test.go:137: author ["host@example.com"], want [call@example.com]` |
| 2. Call's git config has no user.email, Call has USER=ana: author is `ana` | HOLDS | `--- PASS: TestJzMbUserFromCall` with `/USER`, `/USERNAME` (bob), `/USER_wins_over_USERNAME` (ana). On base: `author ["host@example.com"], want [ana]` (and bob, ana) |
| 3. No email, no USER, no USERNAME in the Call: non-zero exit, Stderr holds `cannot find member identity; set GRID_MEMBER`, Feed byte-for-byte unchanged | HOLDS | `--- PASS: TestJzMbNoIdentityRefused` with both host cases (host git config has an email; host git config has none, so the host account is at stake). Every file under the task's `feed/` folder is compared before and after. On base: `exit 0, want non-zero`, `stderr "" does not hold the refusal`, and a record is written by `host@example.com@...0.jsonl` in the first case and by `grid@...0.jsonl` (the host account; it is `grid` in this sandbox, not root) in the second |
| 4a. `go test -count=1 ./lib/` | HOLDS | exit 0 on the final head (`ok github.com/gridconsole/grid-plugins/lib 30.563s`) |
| 4b. `go test -count=1 -run 'Feed' ./internal/cli/` | HOLDS | exit 0 (`ok .../internal/cli 74.477s`) |
| 5a. `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| 5b. `make cross` | HOLDS | exit 0 (darwin/arm64 and windows/amd64 vet and test compile; reported by the harness as completed, exit code 0) |
| Tests of the changed packages and of every importer | HOLDS | Importers from `go list -test -f '{{.ImportPath}} {{join .Deps " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...` filtered on internal/cli: cmd/grid, contract/formats, internal/account, internal/braincmd, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test -count=1 ./internal/cli/ ./cmd/grid/ ./contract/formats/ ./internal/account/ ./internal/braincmd/ ./internal/golden/ ./internal/jobgraph/ ./internal/mcp/ ./internal/packaging/ ./internal/question/ ./internal/settings/ ./lib/` exit 0: all `ok` (cmd/grid has no test files), internal/cli 146.774s |
| Plugin walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok 1.884s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok 7.794s`). The app walk tests (`make webui-test`, TestFvArgvCollects, TestEveryWiredMount, slices Imports) are for the app repo and do not apply here |
| New tests fail on base, pass on the lane | HOLDS | Base run with `git stash push internal/cli/feed.go`: the failing lines are in rows 1 to 3 above, plus `TestJzMbEmptyEnvironIsNotTheHosts` (`the host's identity "host@example.com" was written (exit 0, )`) and `TestJzMbTwoCallsTwoMembers` (`author ["host@example.com"], want [ana@example.com]` and bob). `TestJzMbBinaryKeepsItsReads` passes on base and on the lane: it is the proof that Getenv nil changes nothing |
| No existing test edited or deleted | HOLDS | `git diff origin/main..HEAD --stat` shows internal/cli/feed.go, lib/lib.go (comment), lib/t2_jz_member_test.go (new) and this report only |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change only moves where a missing member is looked up; the refusal text and its condition are the ones at base |
| `*RealPlugin*` tests | NOT NEEDED | No row names one, and every test here runs the lib or the binary built from this clone |
| Public repo: no personal paths, hosts or emails | HOLDS | Only `example.com` addresses and `ana`, `bob`, `hostuser` in the test |

## What changed

- `internal/cli/feed.go`, `feedWriter` only (feedStore at :213 is untouched). When `env.Getenv` is set (a lib call), the
  `git config user.email` child gets `cmd.Env` made from the call's getenv over HOME, USERPROFILE, XDG_CONFIG_HOME,
  GIT_CONFIG_GLOBAL, GIT_CONFIG_SYSTEM, GIT_CONFIG_NOSYSTEM, PATH, SystemRoot and APPDATA, each when non-empty.
  The slice is made with `make([]string, 0, 9)`, so a call with none of them gives git an empty environment and not
  the host's (a nil `cmd.Env` would inherit the host's). `cmd.Dir` stays `env.Cwd`. The account step reads the call's
  USER, then USERNAME, and does not call `user.Current`. With Getenv nil (the binary) both steps are as before. The
  refusal text is unchanged.
- `lib/lib.go`, doc comment only (not in the brief's Touches; its own commit, drop it if the file is not to change):
  the comment at the old lines 24-28 listed the Feed member as a host read, which is no longer true, so it now says the
  member comes from the Call's own git config, USER and USERNAME.
- `lib/t2_jz_member_test.go` (new): `TestJzMbEmailFromCallGitConfig`, `TestJzMbUserFromCall`, `TestJzMbNoIdentityRefused`,
  `TestJzMbEmptyEnvironIsNotTheHosts`, `TestJzMbTwoCallsTwoMembers` (two homes, two calls at once behind a start
  barrier), `TestJzMbBinaryKeepsItsReads` (Getenv nil: git reads the process's environment, the account is the system's).

## Open items

- The git child still reads the machine's system git config unless the call sets GIT_CONFIG_NOSYSTEM (the brief's
  list passes it through; system config is a file of the machine, not of the host process's environment).
  `TestJzMbEmptyEnvironIsNotTheHosts` therefore asserts that the host's identity is not written and, when it refuses,
  that the refusal text is there; it does not insist on a refusal if the machine's system config names an email.
- The Windows behaviour is built and vetted (`make cross`) but not run.

## NOT VERIFIED on a live screen

Nothing here is UI. A real app run of a person call with no GRID_MEMBER was not done; the brief records that the app's
product callers always set GRID_MEMBER and run the binary.
