# C-js-guest-settings-opener-code

The settings store opener, `grid claude settings set` and `project repo` now return the guest sentinel `ilGuestRefused`, so `--json` prints the `guest_refused` envelope. The stderr text and exit codes are the same as before.

Lane commits on top of 07cc9052 (the lane start, which already has C-il-guest-refusal-code merged in):

- bf2d2d50: the three production sites, plus the `ilGuestOpenSites` shrink.
- ed9cb9e2: the new tests.
- f1e722d5: moves `project repo` out of the parity test's known-open pin (a deviation; see "Rules" below).

## Proof table

| Row | State | Proof |
|---|---|---|
| Base check: 013a54ce is an ancestor | HOLDS | `git merge-base --is-ancestor 013a54ce HEAD` printed BASE_OK, exit 0 |
| Base check: `errors.New(guestSettingsRefusal)` sites | HOLDS (5 lines, not 4) | `git grep` printed budget.go:137, project_repo.go:76, settings.go:48, signal.go:65 **and cli.go:27**, which is the sentinel's own `var` declaration and not a call site. After the change, only cli.go:27 and signal.go:65 remain |
| Base check: no `jsGo`/`TestJsGo` names yet | HOLDS | `git grep -n -E 'jsGo\|TestJsGo'` printed nothing |
| 1. TestJsGoOpenerCallers | HOLDS, with one answer that differs (`budget apply`) | `go test -count=1 -v -run 'TestJsGo\|TestIl\|TestGr' ./internal/cli/` printed `--- PASS: TestJsGoOpenerCallers`, then `ok`, exit 0. For guest and external, `agent set/unset`, `skill enable/disable` and `hook level` with `--project alpha` all go through settings.go:104. Each run, plain and `--json`, gives exit 1, empty stdout, the exact stderr `<kind>: guests cannot change project settings`, the `guest_refused` envelope, and unchanged HOME and GRID_HOME trees. See the two notes below this table |
| 1. fails on base | HOLDS | On 07cc9052 with only the tests added: `--- FAIL: TestJsGoOpenerCallers/{guest,external}/{agent_set,agent_unset,skill_enable,skill_disable,hook_level}`, each with `js_guest_opener_code_test.go:77: error JSON: unexpected end of JSON input:` (no code printed). `setup_project` and `budget_apply` passed on base |
| 2. TestJsGoClaudeSet | HOLDS | `--- PASS: TestJsGoClaudeSet`, exit 0. The real argv is `claude settings set autoCompactEnabled true [--json]`; the brief's shorthand `grid claude <key> <value>` exits 2 with usage (budget.go:90). For guest and external: exit 1, stderr `claude: guests cannot change project settings`, `guest_refused`, trees unchanged. The `member_writes` control runs the same argv as a member: exit 0 and the HOME tree changes, so the "writes nothing" check is not vacuous. Fails on base: `--- FAIL: TestJsGoClaudeSet/guest`, `/external` with `js_guest_opener_code_test.go:89: error JSON: unexpected end of JSON input:` |
| 3. TestJsGoProjectRepo | HOLDS | `--- PASS: TestJsGoProjectRepo`, exit 0. For guest and external, `project repo add\|edit\|remove alpha …`: exit 1, stderr `grid: guests cannot change project settings`, `guest_refused`, trees unchanged. The `member_writes` control writes under GRID_HOME. Fails on base: `--- FAIL: TestJsGoProjectRepo/{guest,external}/{add,edit,remove}` with `js_guest_opener_code_test.go:105: error JSON: unexpected end of JSON input:` |
| 3. TestGr still passes | HOLDS | same run: `--- PASS: TestGrProjectRepoGuestIsRefused` (gr_project_repo_test.go:200-222, stderr unchanged), with every `TestGr*` passing, exit 0 |
| 4. TestIl with `ilGuestOpenSites = {"signal.go": 1}` | HOLDS | il_guest_refusal_code_test.go:182-184. `--- PASS: TestIlGuestSiteList`, exit 0. On base (only the list shrunk) it failed: `il_guest_refusal_code_test.go:238: settings.go: 1 errors.New(guestSettingsRefusal) site(s) not in the known-open list; return the guest sentinel`, the same line for budget.go and project_repo.go, and `:250: the walk finds 4 sites, want the 1 known open` |
| 5. `go test -count=1 ./internal/cli/` | HOLDS | `ok github.com/gridconsole/grid-plugins/internal/cli 39.438s`, exit 0, at f1e722d5 |
| 5. `make lint` | HOLDS | `make lint > /dev/null 2>&1 && echo LINT_EXIT_0` printed LINT_EXIT_0 (exit 0) |
| 5. `make cross` | HOLDS | `make cross > /dev/null 2>&1 && echo CROSS_EXIT_0` printed CROSS_EXIT_0 (exit 0) |
| 5. diff touches only Touches + this doc | HOLDS, with one extra file | `git diff --stat 013a54ce..HEAD` already listed 38 files from main before my first edit, because 07cc9052 merges main into the C-il lane. The lane's own diff is `git diff --stat 07cc9052..HEAD`: budget.go, project_repo.go, settings.go, il_guest_refusal_code_test.go, js_guest_opener_code_test.go, **guest_parity_test.go** (see Rules), and this doc |
| Opener callers, `%w` | HOLDS, no change needed | Lane line numbers (one less than base after the import removal): settings.go:105 `return commandError(env, kind, err)`; settings.go:217 `return commandError(env, "setup", err)`; budget.go:60 `return commandError(env, "agent", err)`; worker.go:217 `return "", "", err` (listing=true at :212/:214, never refused). None of them wraps the error; commandError calls `env.recordError(err)` directly (artifact.go:59-60) |
| Rule: importers tested | HOLDS | Packages importing internal/cli (`go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...`): cmd/grid, lib. Packages whose tests import it (TestImports/XTestImports): contract/formats, internal/account, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings. `go test -count=1` on all of these: every one `ok` (cmd/grid has no test files), exit 0 |
| Rule: new tests fail on base | HOLDS | rows 1-4 above |
| Rule: no existing test edited to pass | DEVIATION, reported | guest_parity_test.go:418-422. `guestParityEarlyAnswer` pinned `project repo` as "Known open (TestIlGuestSiteList): no code, so no stdout". After the change, `TestGuestProjectParity/project_repo/{operand,operand-project-id}/json/invited` failed with `guest_parity_test.go:391: invited guest still gets unknown answer: {Code:1 Stdout:{"error":{"code":"guest_refused",…}}`. That pin hangs off the same known-open list the brief tells me to shrink. I moved `project repo` into the `guestRefused` group at :418 (exit 1, same stderr, and it now *requires* the envelope), and left `signal add` known-open at :422. The file is outside Touches. The alternative I rejected was reverting project_repo.go, which would leave row 3 NOT DONE |
| Rule: no guard, gate, cap or hold added | HOLDS | Only the error value changes at three existing refusals; no new check |
| Rule: no attribution lines, no personal paths | HOLDS | `git log 07cc9052..HEAD` shows one-sentence messages with no trailers |
| Rule: RealPlugin / UI | n/a | no RealPlugin test and no UI in this brief |

### Notes on row 1

- **`grid setup <project>` (settings.go:216):** the case passes, both on base and on the lane, but it does not go through the opener. Role `guest` is refused at the dispatch gate (cli.go:198, stderr `grid: …`). Role `external` is refused at runSetup (settings.go:175 on the lane, stderr `setup: …`). Both already returned `ilGuestRefused`, so no refused caller reaches the opener call at :216.
- **`agent budget apply` (budget.go:59): the answer differs from the brief.**
  - Request: `agent budget apply balanced` and `agent budget apply balanced --json`, as guest or external, with GRID_MEMBER=gus invited to alpha.
  - Response: exit 1, empty plain stdout, stderr `agent: project "personal" is not registered`, envelope `{"error":{"code":"no_such_project","message":"agent: project \"personal\" is not registered"}}`, trees unchanged.
  - Why: budget.go:58 on the lane (:59 on base) opens the Personal store, and `Resolve` refuses Personal for any guest (`GuestProjectAllowed`, where/guest.go:44). The opener therefore returns the lookup error at settings.go:38 before its guest check at :46-47 runs.
  - The test pins this as seen. I did not reorder the opener, because the brief says the stderr stays as today.

## What changed

- internal/cli/settings.go:46-47: the opener returns `ilGuestRefused`. The unused `errors` import is removed.
- internal/cli/budget.go:135-136: `claude settings set` returns `ilGuestRefused`. The unused `errors` import is removed.
- internal/cli/project_repo.go:75-76: `project repo` returns `ilGuestRefused`.
- internal/cli/il_guest_refusal_code_test.go:182-184: `ilGuestOpenSites` is now `{"signal.go": 1}`, and the comment names C-iy-signal-refusal-codes.
- internal/cli/js_guest_opener_code_test.go (new): TestJsGoOpenerCallers, TestJsGoClaudeSet, TestJsGoProjectRepo.
- internal/cli/guest_parity_test.go:418-422: `project repo` moved from the known-open pin to the `guest_refused` refusals.

## NOT VERIFIED on a live screen

Nothing. The change is CLI-only, and every row was proved with go test and make.

## Open items

- `agent budget apply` and personal `agent set` / `skill` / `hook` without `--project` give a guest `no_such_project`, not `guest_refused`, because the Personal resolve fails before the guest check. This is for the owner to decide. A fix would change stderr, which this brief rules out.
- signal.go:65 stays known open, waiting for C-iy-signal-refusal-codes (il_guest_refusal_code_test.go:183 and guest_parity_test.go:422).

DONE
