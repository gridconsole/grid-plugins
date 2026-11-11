# C-ks-host-rules

`grid setup --yes` writes one more Claude Code deny rule: `Edit(<home>/*/tasks/[Tt]-[0-9]*/**)` (with "tasks" folded), for gate files in a task-like folder one level below the Grid home. The host guard's other outside forms are named "not carried, by decision" in the `claudeHostPaths` comment and get no rule.

Base: `cf5b259f` (origin/main, lane-Pint121 merge; the brief's line numbers held: `Not covered` was at claude.go:80). Code head: `461b30b9`; the report commit sits on top of it.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. New TestKsHostRulesGateFilesBelowHome, both layouts, positive half and not-carried half | HOLDS | internal/cli/ks_host_rules_test.go. Runs `setup --yes` through claudeHarness in the default and the outside-home layout, reads rules with claudeDenyRules and matches them with claudeRuleMatches. Finds a rule for `<home>/notes/tasks/T-01-x/approval.json` and `.../questions/q1.md` in both layouts. In the outside layout it finds no rule for `<parent>/notes/tasks/T-01-x/approval.json`, `<parent>/settings.json`, `<parent>/x/workflows.md` or `<Dir(Dir(projects))>/.host/hold.json`. It fails on base (below) and passes on the lane. |
| 2. The three tests pass; the two new table rows fail on base | HOLDS | `go test -count=1 -v ./internal/cli -run 'TestSetupWritesDenyRulesForTheHostGuardFiles\|TestClaudeDenyRulesEqualWriterGuard\|TestKsHostRules'` exit 0; prints `--- PASS: TestKsHostRulesGateFilesBelowHome`, `--- PASS: TestSetupWritesDenyRulesForTheHostGuardFiles`, `--- PASS: TestClaudeDenyRulesEqualWriterGuard`. Failing lines on base (claude.go stashed, same tests, exit 1) are below. |
| 3. `Not covered` gone, `Not carried, by decision` once | HOLDS | `git grep -n 'Not covered' -- internal/settings/claude.go` prints nothing. `git grep -n 'Not carried, by decision' -- internal/settings/claude.go` prints one line: `internal/settings/claude.go:84`. |
| 4. `go test -count=1 ./internal/settings/ ./internal/cli/ ./internal/doctor/ ./internal/guard/` | HOLDS | exit 0: settings ok 0.441s, cli ok 40.238s, doctor ok 1.834s, guard ok 1.107s. |
| 5. Walk tests after `git merge main` | HOLDS | `git merge main` printed `Already up to date.` `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok internal/cli 0.110s`). `go test -count=1 ./internal/job/lint/` exit 0 (`ok 0.243s`). |
| 6. `make lint`, `make cross` | HOLDS | `make lint` (go vet ./...) exit 0. `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test-compile of every package, no errors. `gofmt -l internal/cli internal/settings` prints nothing. |

## Rules

| Rule | Result | Proof |
|---|---|---|
| Base sha quoted; EvaluateHost listed and the guard untouched | HOLDS | Base `cf5b259f`. `git grep -n -w EvaluateHost -- '*.go'` lists internal/guard/host.go:97,103; `git diff origin/main..HEAD --stat` touches nothing under internal/guard. |
| Small commits, one-sentence messages, no attribution | HOLDS | Two commits: `d33613ec` (rule, comment, goldens) and `461b30b9` (tests). No Co-Authored-By or Claude-Session line; no personal path, host or email in the diff. |
| New tests fail on base and pass on the lane | HOLDS | Failing lines on base are below. |
| No existing test edited, none deleted | HOLDS | setup_claude_host_test.go gained two `add(...)` rows after the `PROJECTS folder` row, as the brief allows; no other line of it changed. setup_claude_test.go is untouched. The goldens fresh.json and second.txt gained the new rule only, as the brief allows. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change is one Claude Code `permissions.deny` entry inside the Grid home plus a comment. No hook or guard code changed. |
| Tests of changed packages and every package that imports them | HOLDS | Changed: internal/settings, internal/cli. Direct importers of internal/settings from `go list`: account, cli, doctor, guard, hook, updates. Importers of internal/cli: cmd/grid, lib. `go test -count=1 ./internal/account/ ./internal/hook/ ./internal/updates/ ./cmd/grid/ ./lib/` exit 0 (account ok, hook ok 10.035s, lib ok 11.142s; updates and cmd/grid have no test files). settings, cli, doctor, guard: row 4. |
| Walk tests (plugin) | HOLDS | Row 5. The app walk tests (webui-test, TestFvArgvCollects and so on) belong to the app repo and do not apply here. |
| *RealPlugin* tests | N/A | No row names one and no RealPlugin test touches setup rules; none run, none claimed. |

### Failing lines on base (claude.go stashed, new tests kept)

```
--- FAIL: TestKsHostRulesGateFilesBelowHome (0.01s)
    ks_host_rules_test.go:45: no written rule matches .../home/.grid/notes/tasks/T-01-x/approval.json
    ks_host_rules_test.go:45: no written rule matches .../home/.grid/notes/tasks/T-01-x/questions/q1.md
--- FAIL: TestSetupWritesDenyRulesForTheHostGuardFiles (1.39s)
    setup_claude_host_test.go:190: home/notes/tasks approval: deny rules match=false want true ([])
    setup_claude_host_test.go:190: home/notes/tasks question: deny rules match=false want true ([])
FAIL	github.com/gridconsole/grid-plugins/internal/cli
```

Both layouts (`default`, `outside_home`) fail the same way. The guard oracle at setup_claude_host_test.go:178-179 did not fire for the new rows, so EvaluateHost refuses both today, as the brief said.

## What changed

- internal/settings/claude.go: `claudeBlock` has the new entry `anchor + "/*/" + claudeFold("tasks") + "/[Tt]-[0-9]*/**"` after the personal-tasks entry, so the written order is project tasks, personal tasks, this rule, knowledge. The `claudeHostPaths` comment says the rule carries the gate files one level below the Grid home. The "Not covered" paragraph is now one "Not carried, by decision" paragraph naming the four forms and the reason (Grid reads nothing there; they are the person's own folders). It still names EvaluateHost. Both claims in it were checked: only internal/guard, internal/ports/ports.go and this comment name `.host`; internal/workflow/gates.go:68-78 reads the Grid home layout only.
- internal/settings/testdata/claude/fresh.json and second.txt: the new rule line at its place in the order, written by hand, in both the `+`/`=` list and the printed JSON.
- internal/cli/setup_claude_host_test.go: two want=true rows after the `PROJECTS folder` row.
- internal/cli/ks_host_rules_test.go (new): helper `ksHrMatching`, test `TestKsHostRulesGateFilesBelowHome`.

## Notes for the reviewer

- The new rule also matches `<home>/personal/tasks/T-n/**`, which the personal-tasks entry covers already. The overlap is harmless and stays, as the brief says.
- `*` spans one segment, so `<home>/projects/<p>/tasks/T-n` (two segments) is not matched by the new rule; the project-tasks rule covers it.
- No rule is written on a folder outside the Grid home.

## NOT VERIFIED on a live screen

Claude Code's own matching of the new deny pattern is not exercised: the tests use the repo's `claudePatternMatches` model of the pattern (the same one the existing parity test uses). No UI row.
