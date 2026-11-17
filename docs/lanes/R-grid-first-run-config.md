# R-grid-first-run-config: proof table

Research only. The one deliverable is docs/research/first-run-config.md. No Go file is changed.

| Row | Result | Evidence |
| --- | --- | --- |
| Q1 first-run step in a Claude Code plugin | HOLDS | docs/research/first-run-config.md section 1. Claude Code 2.1.325, from this session's own record; `claude --version` was refused by the allow list. The hooks, plugins-reference, cli-reference and install docs were read 17 Nov 2026. No install or enable hook exists. `userConfig` shows a dialog on enable. SessionStart gives `systemMessage` and `additionalContext` only. Today the plugin uses `additionalContext` (`internal/hook/hook.go:152-155`) and `grid setup` |
| Q2 app install path and how it runs setup | HOLDS | section 2, from `app:internal/slices/setup/plain.go:77-96`, `agentreads/plugin_hosts.go:14,47-118`, `plugincli/client.go:139,176-179`, onboarding `logic.mjs:9-20,380-433` |
| Q3 home, brain, projects and member resolution on both sides, plus the files each side writes | HOLDS | section 3. The writer lists are partial where the doc says so. The plugin's `hookevents` and `feed` write paths were not traced |
| Q4 proposed config, precedence, older file, several workspaces | HOLDS | section 4 |
| Q5 question set and defaults | HOLDS | section 5 |
| Q6 lane plan, U-mac-first-open-setup, C-mac-desktop-plugin-home | HOLDS | section 6. `dd7eefdbd` is in the C lane clone but not in the app ref repo (`git show` exits 128 there) |
| Research doc committed on the lane, last line DONE | HOLDS | `git log -1 -- docs/research/first-run-config.md` |
| Copy to /srv/gridwork/tmp/hand/R-grid-first-run-config.md | HOLDS | `cp` exit 0 |
| Tests of changed packages and their importers | not run, no Go package changed | `git diff --stat origin/main` lists only docs/ |
| Walk test `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `ok internal/cli`, exit 0 |
| Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | `ok internal/job/lint`, exit 0 |
| `go test ./internal/packaging/` (a repo walk that the brief does not name) | HOLDS on a clean clone of the lane | In the build folder it fails on `app/...` "getsentry" lines. Those come from the untracked app clone that the brief asked for at `app/`, and they are not in the bundle. A clean clone of the lane head gives `ok internal/packaging`, exit 0 |
| `make lint` | HOLDS | clean clone, `go vet ./...` exit 0 |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` and `GOOS=darwin go build ./...`, exit 0 |
| `git fetch origin && git merge origin/main` before the last run | HOLDS | "Already up to date." |
| Whole-module test, cross-repo set | not run, not asked | the orchestrator runs these at landing |
| No guard, gate or block proposed | HOLDS | sections 1 and 6 say a missing config is an offer only |
| No attribution lines, no personal paths or emails in the repo | HOLDS | commit messages are plain. The doc writes `~/.grid/<name>` for the owner's folder |

What changed: added docs/research/first-run-config.md and this file.
Untracked and not committed: `app/` (the app clone the brief asked for) and `check/` (a clean clone used for the
packaging walk, lint and cross builds). The orchestrator may delete both. The allow list has no `rm`.

NOT VERIFIED (needs an interactive Claude Code or a Mac): whether the `userConfig` dialog appears on enable from a shell
install or from the app, and the exact content of the owner's `~/.grid/config.json`.

DONE
