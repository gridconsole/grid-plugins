# C-ia-brain-role-refusal-json

Base: 91fadeff87525f5859978740e0b66ce51f375e48 (main = origin/main). Code head: e2f32b99 (this report is the commit after it).

Base check: on 91fadeff a guest `grid brain changes --json` printed nothing on stdout. `go test ./internal/braincmd -count=1 -run TestIaBr` on base, before any production edit:

```
--- FAIL: TestIaBrRoleRefusal/guest/changes (0.00s)
    ia_brain_role_refusal_test.go:79: stdout "" is not one JSON object: unexpected end of JSON input
--- FAIL: TestIaBrRoleRefusal/guest/revert (0.00s)
    ia_brain_role_refusal_test.go:79: stdout "" is not one JSON object: unexpected end of JSON input
--- FAIL: TestIaBrRoleRefusal/guest/supersede (0.00s)
    ia_brain_role_refusal_test.go:79: stdout "" is not one JSON object: unexpected end of JSON input
--- FAIL: TestIaBrRoleRefusal/other/changes|revert|supersede: the same line, 3 times
--- FAIL: TestIaBrHiddenProjectSame/changes (0.00s)
    ia_brain_role_refusal_test.go:109: stdout "" is not one JSON object: unexpected end of JSON input
--- FAIL: TestIaBrHiddenProjectSame/revert (0.00s)
    ia_brain_role_refusal_test.go:109: stdout "" is not one JSON object: unexpected end of JSON input
--- FAIL: TestIaBrReadme (0.00s)
    ia_brain_role_refusal_test.go:135: the brain role paragraph does not name person_required
    ia_brain_role_refusal_test.go:135: the brain role paragraph does not name role_not_allowed
    ia_brain_role_refusal_test.go:135: the brain role paragraph does not name supersede
```

The plain-mode checks and the "member cannot resolve beta" fixture check passed on base. The failures are only the missing `--json` object and the README words.

## Proof table

| Row / rule | Status | Proof |
|---|---|---|
| 1. TestIaBrRoleRefusal | HOLDS | `go test ./internal/braincmd -count=1 -v -run 'TestIaBr\|TestRoleRefusalComesBeforeAnyReadOrWrite\|TestRolesDecideBothVerbs'` exit 0: `--- PASS: TestIaBrRoleRefusal`. Roles `guest` and `other`; changes, revert `decisions/checks --to 1` and supersede `parity --by parity-v2 --reason "parity reason here"`. Each run checks: exit 3 in both modes; stderr equal to the quoted line and to the plain stderr; plain stdout empty; `--json` stdout decodes to exactly the five keys (reflect.DeepEqual on the map) and is byte-equal to `{"error":"person_required","reason":"role_not_allowed","hint":"<line after 'grid brain <verb>: '>","code":"person_required","message":"<line>"}\n`; no Grid home is created (ia_brain_role_refusal_test.go:64-86, object check :29-44). |
| 2. TestIaBrHiddenProjectSame | HOLDS | Same run, `--- PASS: TestIaBrHiddenProjectSame`. The Grid home has `alpha` (granted to `gus` in project-roles.json) and `beta` (registered, not granted). The resolver caller is guest `gus`. A `member` role first shows that beta really is hidden (exit 1, `"beta" is not registered`). Then the guest's changes and revert with `--project beta --json` and `--project absent --json` each give exit 3, the object, byte-identical stdout and identical stderr (ia_brain_role_refusal_test.go:90-117). |
| 3. TestIaBrReadme | HOLDS | Same run, `--- PASS: TestIaBrReadme`. The test finds the paragraph by its first words "Both verbs check `GRID_ROLE`" and stops at the next blank line. contract/README.md:655-660 adds supersede and the `--json` object and keeps :654 byte for byte. |
| 4. Pins move | HOLDS | changes_test.go:424-434: the ghost rows still need empty stdout; the two `--json` rows need the exact object. gr_brain_envelope_test.go:90 needs the object built from stderr. guest_parity_test.go:439-453: json mode sets `want.Stdout` to the object for the three verbs. `go test ./internal/braincmd -count=1` exit 0 (`ok ... 1.870s`); `go test ./internal/cli -count=1 -run 'TestGuestProjectParity\|TestGrBrain\|TestBrain\|TestStableErrorCodes'` exit 0 (`ok ... 5.445s`); `go test ./internal/contract -count=1` exit 0 (`ok ... 1.137s`; go_readme_subverbs_flags_test.go:32 still finds its span). Verbose PASS lines include `TestGuestProjectParity/brain_{changes,revert,supersede}/{project,project-project-id}/json/{invited,parity}` and `TestGrBrainEnvelopeGuestHiddenProject/{changes,revert}`. |
| 5a. make lint | HOLDS | `make lint && make cross` exit 0 (`go vet ./...`, no error). |
| 5b. make cross | HOLDS | Same call: `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile`, every package `ok` or `[no test files]`, exit 0. |
| 5c. diff stat names only Touches | HOLDS | `git diff main --stat` lists contract/README.md, internal/braincmd/{changes.go, changes_test.go, ia_brain_role_refusal_test.go, role_refusal.go, supersede.go}, internal/cli/{gr_brain_envelope_test.go, guest_parity_test.go}: 8 files, +196 -8. Every hunk is in the line ranges the brief allows (changes.go @@153, @@237; supersede.go @@61; changes_test.go @@424-426; gr_brain_envelope_test.go @@90; guest_parity_test.go @@440-444; README @@654). The runbook also requires this report, docs/lanes/C-ia-brain-role-refusal-json.md, as its own commit. |
| Rule: stderr byte for byte, exit 3, plain unchanged | HOLDS | internal/braincmd/role_refusal.go:14-30 builds the same lines. internal/cli/brain_test.go:67,78 (plain mode, untouched), changes_test.go:380-411 TestRolesDecideBothVerbs (untouched) and supersede_test.go:201 (untouched) pass. |
| Rule: which roles may act does not change | HOLDS | `brain.ChangesReadAllowed` / `brain.RevertAllowed` are untouched. The call sites only replace the print (changes.go:152-154, :235-237; supersede.go:60-62). |
| Rule: new tests fail on base, pass on lane | HOLDS | Failing lines above; PASS lines in rows 1-3. |
| Rule: no existing test edited to pass except the brief's pins | HOLDS | Only the three pins the brief names changed (row 4). brain_test.go, supersede_test.go and TestRolesDecideBothVerbs are untouched. |
| Rule: no guard, gate, cap, hold or hook added | HOLDS | The role check already existed; this lane only adds stdout output under `--json`. |
| Rule: tests of changed packages and their importers | HOLDS | `go list` importers of braincmd: `internal/braincmd`, `internal/cli`. Importers of cli/contract: `cmd/grid contract contract/formats internal/account internal/braincmd internal/cli internal/contract internal/golden internal/hook internal/job/lint internal/jobgraph internal/mcp internal/packaging internal/question internal/settings lib`. `go test -count=1` over all of them exit 0 (cli `ok 65.338s`, contract `ok 5.422s`, the rest `ok`). |
| Rule: tests use no clock, no sleep, temp home, no skip | HOLDS | ia_brain_role_refusal_test.go uses newHarness (t.TempDir). It has no time, sleep or Skip. |
| Rule: helpers prefixed iaBr, new tests in the new file | HOLDS | iaBrVerbs, iaBrObject, iaBrProject, TestIaBr* in internal/braincmd/ia_brain_role_refusal_test.go. |
| Rule: commits small, one sentence, no attribution | HOLDS | 418b23e1 (tests), ee2c50d7 (code + pins), e2f32b99 (README), then this report. None has a trailer. |
| Rule: RealPlugin / UI | N/A | The brief has no RealPlugin or UI row. |

## What changed

- New `internal/braincmd/role_refusal.go`: `roleRefused(env, verb, what, asJSON)` prints `grid brain <verb>: caller role is not allowed to <what>` on stderr. With `--json` it also writes one stdout line, `{"error":"person_required","reason":"role_not_allowed","hint":"caller role is not allowed to <what>","code":"person_required","message":"<line>"}`. The keys are in that order and the encoder uses SetEscapeHTML(false), the same as cli's writeJSON. It returns 3.
- changes, revert and supersede call it in place of their own Fprintln and `return 3`.
- The CLI layer adds no envelope of its own: `commandFailure.finish` returns once stdout has bytes (internal/cli/error_codes.go:76). TestGuestProjectParity pins the whole stdout, so it holds exactly one object.
- contract/README.md, the paragraph "Both verbs check `GRID_ROLE`": it now names supersede and the `--json` object, and says that without `--json` stdout stays empty.

## Open items / NOT VERIFIED

- Cross-repo, not run here: the app's plugin client (app internal/kernel/plugincli person.go / client.go) should decode this object as PersonRequired with exit code 3, so `brainchanges` keeps answering 403. The orchestrator's cross-repo set covers that at landing.
- No live screen is involved.

DONE
