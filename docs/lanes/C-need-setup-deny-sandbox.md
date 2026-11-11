# C-need-setup-deny-sandbox: grid setup writes Claude Code deny rules for the host files and can switch on Claude Code's own sandbox

Base: plugin main 55c46cbdeab67b5afcfcb42806c49caf2a12386e. Redone on main from the brief and the accepted lane S8 (not merged, not rebased).

## Proof table

| # | Row / rule | Result | Proof |
|---|---|---|---|
| 1 | TestSetupWritesDenyRulesForTheHostGuardFiles fails on base, passes on the lane | HOLDS | Base 55c46cbd, new test file only: `go test -count=1 ./internal/cli -run '^TestSetupWritesDenyRulesForTheHostGuardFiles$'` FAIL: `setup_claude_host_test.go:101: no host rules written` and `setup_claude_host_test.go:185: home settings.json: deny rules match=false want true ([])` (and the same for every host path, both layouts). On the lane: ok. It pins host-guard and only-writer to block with `settings.Store.SetHook`, runs `grid setup --yes` in the default layout and with GRID_PROJECTS outside the home, and checks 40+ paths per layout (3 names x 5 places x lower and upper case, mixed case, `.host` and `.HOST` in the home and beside the projects folder, task files approval*.json, feed/, questions/, 12 paths neither guard refuses). Each path must be refused by EvaluateHost or EvaluateOnlyWriter exactly when a rule matches, and each host rule must be the only match for at least one path. |
| 2 | TestSetupSandboxFlag | HOLDS | Base: `[setup --sandbox --yes]: code=2 want 0: ... flag provided but not defined: -sandbox`. Lane: ok. Covers plain setup (no key), `--sandbox --yes` (`{"enabled":true}`), second run no change, plain setup keeps it, `--remove` drops only `enabled`, other sandbox keys and a pre-enabled sandbox stay, empty object dropped, `--sandbox --remove` exit 2 and file unchanged, malformed `sandbox` exit 2 only with `--sandbox`. |
| 3 | TestClaudeDenyRulesEqualWriterGuard still pins the writer rules | HOLDS | internal/cli/setup_claude_test.go:182-186: host rules filtered as additional through `hostDeny`, like the ~/.grid-person and setup rules. No test deleted. Passes (in the run of row 6). |
| 4 | Goldens in step | HOLDS | fresh.json and second.txt written from the real output by a throwaway test, deleted again (`git status` shows only the intended files). `go test ./internal/settings` TestClaudeFreshAndSecondRun, `./internal/cli` TestSetupClaudeCLI and TestContractCommandsParserInventory all pass. `UPDATE_CONTRACT_COMMANDS=1 go test ...` was refused by the sandbox, so the one `--sandbox` flag object was added by hand to the `setup` entry at the line the test named (contract/commands.json:4617); the passing inventory test proves the file equals what the parsers produce. `git diff --stat main -- contract/commands.json`: 5 insertions, one entry. |
| 5 | No Grid hook, check or level; remove leaves none | HOLDS | `git diff --stat main -- internal/guard internal/hook internal/settings/catalog.go hooks` prints nothing (exit 0). TestSetupSandboxFlag: after `--sandbox --yes` and `--remove --yes`, `sandbox` is absent and `permissions.deny` is empty. |
| 6 | Tests, builds, lint | HOLDS | `go test -count=1` on settings, cli, golden, packaging, doctor, contract, plus importers guard, hook, mcp, question, cmd/grid, contract/formats (list from `go list`: cmd/grid, contract/formats, internal/cli, doctor, golden, guard, hook, mcp, packaging, question, settings): all ok, exit 0. `GOOS=windows go build ./...` exit 0. `GOOS=darwin go build ./...` exit 0. `make lint` exit 0, `gofmt -l internal cmd` empty. |
| R1 | Small commits, one sentence, no attribution, no personal data | HOLDS | `git log origin/main..HEAD`. |
| R2 | New tests fail on base | HOLDS | Rows 1 and 2. |
| R3 | Existing tests not edited to pass | HOLDS | Two minimal edits the brief names: the usage string (setup_claude_test.go:121) and the host-rule filter (:182-186). |
| R4 | No guard, gate, cap or blocking hook on agents | HOLDS | Only Claude Code settings entries are written; internal/guard, internal/hook, catalog.go, hooks untouched (row 5). |
| R5 | Budget 60 calls | NOT MET | About 62 calls: the three refused commands (`rm`, the env-prefixed update, a failed Grep) and the golden regeneration cost extra. The work is complete. |

## What changed
- internal/settings/claude.go: `claudeHostPaths` adds `Edit(...)` deny rules for settings.json, .settings.lock and workflows.md at `<home>/`, `<home>/*/`, `<home>/projects/*/`, `<projects>/`, `<projects>/*/` (no rule twice; `<projects>/` is skipped when the projects folder sits directly in the Grid home, where `<home>/*/` already covers it), and `.host/**` in the Grid home and beside the projects folder. Case-folded with `claudeFold`. `ClaudeSetupOptions.Sandbox`; `sandbox.enabled` is a boolean with its own read/write paths, recorded in the setup record so `--remove` takes out only what setup wrote. Only `sandbox.enabled` is written, never allowUnsandboxedCommands or failIfUnavailable.
- internal/cli/settings.go: `--sandbox` flag ("switch on Claude Code's own sandbox for this user"), usage line, `--sandbox --remove` exit 2.
- Goldens fresh.json, second.txt; contract/commands.json (`--sandbox` on `setup`); README.md (setup paragraph, two sentences); tests in internal/cli/setup_claude_host_test.go.

## Not covered (the guard refuses these; no deny rule is written for them)
- The legacy .host two levels above the projects folder (in the default layout `~/.host`, outside the Grid home; a rule for it breaks TestClaudeCustomGridHomeAndInvalidRules).
- settings.json, .settings.lock and workflows.md directly in, or one folder below, the parent of a projects folder that sits outside the Grid home.
- The same three at `<projects>/projects/<name>/` (host.go:50 with the projects folder as the root).
- Existing installs get the new rules only by running `grid setup` again.

## NOT VERIFIED on a live screen
- That real Claude Code applies the Edit patterns to Write, MultiEdit and NotebookEdit and to case variants, and starts its sandbox with `sandbox.enabled` (the tests apply the patterns with the repo's own matcher). The sandbox does not run on native Windows; the key is still written.
