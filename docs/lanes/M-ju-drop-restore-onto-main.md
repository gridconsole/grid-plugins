# M-ju-drop-restore-onto-main

Carries C-hy-plugin-drop-restore (8fb394c5) onto plugin main (fc3b76aa, which
holds the brain role-refusal JSON work of 99c11b7a). Proofs ran on d144a3a5;
the commit after it adds only this file.

## What conflicted

`git merge origin/lane-C-hy-plugin-drop-restore` conflicted in one file,
internal/cli/guest_parity_test.go, one hunk under `case "brain revert":`:

- main: the `if mode == "json" { want.Stdout = <person_required object> }` branch for revert;
- lane: two new arms `case "brain drop":` and `case "brain restore":`, exit 3 and the stderr line, no JSON branch.

Every other file auto-merged (contract/README.md, contract/commands.json,
internal/cli/cli.go, internal/cli/contract_inventory_test.go,
testdata/golden/help.golden).

## How it was resolved

- internal/cli/guest_parity_test.go:456-470: main's revert JSON branch first,
  then the lane's drop and restore arms, each with the same JSON branch as
  revert (hint `caller role is not allowed to drop|restore brain entries`).
- internal/braincmd/hy_drop.go:98-100: the lane's Fprintf plus `return 3` is now
  `return roleRefused(env, op, op+" brain entries", l.asJSON)`. Same stderr line,
  same exit 3, same place (after the usage check, before `levelStore`); only
  `--json` adds the person_required object on stdout.
  This production change is in the merge commit 4700bc91, with the resolved
  hunk, because the resolved guest-parity JSON rows fail without it.
- internal/braincmd/ia_brain_role_refusal_test.go:25-26: drop and restore rows in
  `iaBrVerbs`. `TestIaBrHiddenProjectSame` still walks `iaBrVerbs[:2]`.
- contract/README.md:692-695: "each of the five refusals (changes, revert,
  supersede, drop, restore)" and `drop brain entries|restore brain entries` in
  the hint alternatives. The lane's drop/restore section is unchanged.

## Proof table

| # | Row | Status | Proof |
|---|---|---|---|
| 1 | Main and the carried lane are in the branch | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0; `git merge-base --is-ancestor origin/lane-C-hy-plugin-drop-restore HEAD` exit 0; `git grep -n -e '^<<<<<<<' -e '^>>>>>>>' -- internal contract` prints nothing. Step 6: `git fetch origin && git merge origin/main` says "Already up to date." (main fc3b76aa) |
| 2 | Lane rows 1-4 on the merged tree | HOLDS | `go test -count=1 -v -run 'TestHyDr' ./internal/brain/ ./internal/braincmd/ ./internal/job/` exit 0, no SKIP (lines below) |
| 3 | Guest gets main's refusal shape on all five brain role checks | HOLDS | `go test -count=1 -v -run 'TestIaBr\|TestGuestProjectParity' ./internal/braincmd/ ./internal/cli/` exit 0; mismatch run fails, then `git checkout -- internal/braincmd/hy_drop.go`, pass (lines below) |
| 4 | Package tests of every touched file | HOLDS | `go test -count=1 ./internal/brain/ ./internal/braincmd/ ./internal/job/ ./internal/cli/` exit 0 |
| 5 | Generated files and goldens | HOLDS | `make generate && git diff --exit-code --stat contract/commands.json testdata/golden` exit 0; `make golden` exit 0, `git status --short` empty after it |
| 6 | Lint and cross, nothing else changed | HOLDS | `make lint` exit 0; `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile); `git diff --stat origin/main HEAD` lists the lane's 19 files plus internal/braincmd/ia_brain_role_refusal_test.go, and with this commit docs/lanes/M-ju-drop-restore-onto-main.md |
| R1 | Small commits, one sentence, no attribution | HOLDS | `git log --format=%B origin/main..HEAD \| grep -c -i -E 'co-authored\|claude-session\|@'` prints 0 |
| R2 | New tests fail on base, pass on lane | HOLDS | the new `iaBrVerbs` rows fail with the lane's role check (row 3 mismatch run) and pass on HEAD |
| R3 | No test deleted, skipped or weakened | HOLDS | `git diff --stat origin/lane-C-hy-plugin-drop-restore HEAD -- internal/braincmd/hy_drop_test.go internal/brain/hy_drop_test.go internal/job/hy_drop_source_test.go` is empty (TestHyDrRoles unchanged, still wants empty stdout without `--json`); guest_parity_test.go +12/-0 and ia_brain_role_refusal_test.go +2/-0 against main |
| R4 | No guard, gate, cap, hold or blocking hook | HOLDS | the role check was already on the lane at the same place (hy_drop.go:98); only its `--json` output changed |
| R5 | Tests of changed packages and of importers | HOLDS | importers from `go list` (below); `go test -count=1 ./cmd/grid ./contract/formats ./internal/account ./internal/golden ./internal/jobgraph ./internal/mcp ./internal/packaging ./internal/question ./internal/settings ./lib` exit 0 |
| R6 | gofmt | HOLDS | `gofmt -l internal/braincmd internal/cli internal/brain internal/job` prints nothing, exit 0 |
| R7 | UI rows | N/A | no UI change |
| R9 | Bundle | HOLDS | `git bundle create <hand>/M-ju-drop-restore-onto-main.bundle origin/main..lane-M-ju-drop-restore-onto-main` exit 0; its prerequisites fc3b76aa and 91fadeff are both on origin/main (`git merge-base --is-ancestor 91fadeff origin/main` exit 0); `git bundle verify` exit 0 |
| R8 | RealPlugin rows | N/A | no row names a *RealPlugin* test |

### Row 2 lines

```
--- PASS: TestHyDrDropAndRestore (0.03s)
--- PASS: TestHyDrRefusals (0.00s)
--- PASS: TestHyDrDropTwiceIsRefused (0.01s)
--- PASS: TestHyDrConcurrent (0.00s)
ok  	github.com/gridconsole/grid-plugins/internal/brain	0.049s
--- PASS: TestHyDrDropAndRestore (0.28s)
--- PASS: TestHyDrRoles (0.05s)
--- PASS: TestHyDrRefusals (0.06s)
ok  	github.com/gridconsole/grid-plugins/internal/braincmd	0.402s
--- PASS: TestHyDrJobSource (0.11s)
ok  	github.com/gridconsole/grid-plugins/internal/job	0.124s
```

### Row 3 lines

On HEAD:

```
--- PASS: TestIaBrRoleRefusal (0.00s)
    --- PASS: TestIaBrRoleRefusal/guest/drop (0.00s)
    --- PASS: TestIaBrRoleRefusal/guest/restore (0.00s)
    --- PASS: TestIaBrRoleRefusal/other/drop (0.00s)
    --- PASS: TestIaBrRoleRefusal/other/restore (0.00s)
--- PASS: TestIaBrHiddenProjectSame (0.00s)
--- PASS: TestIaBrReadme (0.00s)
ok  	github.com/gridconsole/grid-plugins/internal/braincmd	0.013s
    --- PASS: TestGuestProjectParity/brain_drop/project/json (0.00s)
        --- PASS: TestGuestProjectParity/brain_drop/project/json/invited (0.00s)
        --- PASS: TestGuestProjectParity/brain_drop/project/json/parity (0.00s)
    --- PASS: TestGuestProjectParity/brain_restore/project/json (0.00s)
        --- PASS: TestGuestProjectParity/brain_restore/project/json/invited (0.00s)
        --- PASS: TestGuestProjectParity/brain_restore/project/json/parity (0.00s)
    --- PASS: TestGuestProjectParity/brain_drop/project-project-id/json (0.00s)
    --- PASS: TestGuestProjectParity/brain_restore/project-project-id/json (0.01s)
ok  	github.com/gridconsole/grid-plugins/internal/cli	3.451s
```

(the plain-mode drop and restore parity sub-tests pass too, so the guest
still gets exit 3, the same stderr line and empty stdout without `--json`;
TestIaBrRoleRefusal also checks the Grid home is never created.)

Mismatch: with hy_drop.go:98-100 set back to the lane's Fprintf and `return 3`, in the working tree only:

```
--- FAIL: TestIaBrRoleRefusal (0.00s)
    --- FAIL: TestIaBrRoleRefusal/guest/drop (0.00s)
        ia_brain_role_refusal_test.go:81: stdout "" is not one JSON object: unexpected end of JSON input
    --- FAIL: TestIaBrRoleRefusal/guest/restore (0.00s)
        ia_brain_role_refusal_test.go:81: stdout "" is not one JSON object: unexpected end of JSON input
    --- FAIL: TestIaBrRoleRefusal/other/drop (0.00s)
        ia_brain_role_refusal_test.go:81: stdout "" is not one JSON object: unexpected end of JSON input
    --- FAIL: TestIaBrRoleRefusal/other/restore (0.00s)
        ia_brain_role_refusal_test.go:81: stdout "" is not one JSON object: unexpected end of JSON input
FAIL	github.com/gridconsole/grid-plugins/internal/braincmd	0.007s
--- FAIL: TestGuestProjectParity (2.35s)
    --- FAIL: TestGuestProjectParity/brain_drop/project/json (0.00s)
    --- FAIL: TestGuestProjectParity/brain_restore/project/json (0.00s)
    --- FAIL: TestGuestProjectParity/brain_drop/project-project-id/json (0.00s)
    --- FAIL: TestGuestProjectParity/brain_restore/project-project-id/json (0.00s)
FAIL	github.com/gridconsole/grid-plugins/internal/cli	2.355s
```

After `git checkout -- internal/braincmd/hy_drop.go` (`git status --short` empty):

```
ok  	github.com/gridconsole/grid-plugins/internal/braincmd	0.027s
ok  	github.com/gridconsole/grid-plugins/internal/cli	4.348s
```

### Row 4 and importers

```
ok  	github.com/gridconsole/grid-plugins/internal/brain	2.826s
ok  	github.com/gridconsole/grid-plugins/internal/braincmd	1.381s
ok  	github.com/gridconsole/grid-plugins/internal/job	2.090s
ok  	github.com/gridconsole/grid-plugins/internal/cli	56.467s
```

Packages importing braincmd or cli (`go list -f '{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...`
filtered on `internal/(braincmd|cli)`): cmd/grid, contract/formats,
internal/account, internal/braincmd, internal/cli, internal/golden,
internal/jobgraph, internal/mcp, internal/packaging, internal/question,
internal/settings, lib.

```
?   	github.com/gridconsole/grid-plugins/cmd/grid	[no test files]
ok  	github.com/gridconsole/grid-plugins/contract/formats	1.154s
ok  	github.com/gridconsole/grid-plugins/internal/account	3.251s
ok  	github.com/gridconsole/grid-plugins/internal/golden	0.855s
ok  	github.com/gridconsole/grid-plugins/internal/jobgraph	0.689s
ok  	github.com/gridconsole/grid-plugins/internal/mcp	0.821s
ok  	github.com/gridconsole/grid-plugins/internal/packaging	6.530s
ok  	github.com/gridconsole/grid-plugins/internal/question	2.444s
ok  	github.com/gridconsole/grid-plugins/internal/settings	1.409s
ok  	github.com/gridconsole/grid-plugins/lib	14.034s
```

## What changed

`grid brain drop --json` and `grid brain restore --json` from a role that
`brain.RevertAllowed` refuses now print main's person_required object on stdout,
as changes, revert and supersede already do. Stderr, exit 3 and the empty stdout
without `--json` are unchanged.

## NOT VERIFIED

Nothing needs a live screen: this lane changes no UI.

## Open items

- Once this lane is accepted, put C-hy-plugin-drop-restore on landing-excludes.txt.
