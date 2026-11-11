# C-em-dist-public-readme

Base: bf4b8e21 (lane base; the brief named d528178a). Base check: `grep -n README internal/packaging/dist.go docs/release.md` printed nothing, so stage() wrote no README.

| Row / rule | Result | Proof |
|---|---|---|
| 1. Staged plugin has README.md byte-equal to plugin-readme.md | HOLDS | `TestStagedPluginHasThePublicReadme`, `go test -count=1 ./internal/packaging/` exit 0. Stages from the real root (`../..`). Fails on base: `em_public_readme_test.go:26: no README.md at the plugin root: open .../plugin/README.md: no such file or directory` |
| 2. README names the five terms and none of the banned ones | HOLDS | `TestPublicReadmeSaysWhatAnInstallerNeeds`, exit 0. Fails on base: `em_public_readme_test.go:47: open .../plugin/README.md: no such file or directory` |
| 3. LICENSE starts `MIT License`; every manifest `license` key is `MIT` (encoding/json) | HOLDS | `TestStagedLicenseIsMITEverywhere`, exit 0; it fails if no manifest has a license key. It passes on base too: it guards behaviour that already held |
| 4. TestStageLeavesDeniedSourceFoldersBehind unchanged and passing | HOLDS | publish_set_test.go not edited; full package run exit 0. README comes from `//go:embed` (dist.go:68), not the root |
| 5. No `not built yet` / `It is public` in README.md; release.md names README.md | HOLDS | `grep -n 'not built yet\|It is public' README.md` prints nothing (exit 1). docs/release.md:25 (tree), :40-41 (What stays out), :78 (step 3 `ls` line) |
| 6. Diff touches only Touches files; tests; lint; Windows vet | HOLDS | `git diff bf4b8e21 --stat`: README.md, docs/release.md, dist.go, em_public_readme_test.go, plugin-readme.md (+ this proof file). `go test -count=1 ./internal/packaging/` exit 0. `make lint` exit 0. `GOOS=windows go vet ./internal/packaging/` exit 0 |
| Importers of the changed package | HOLDS | `go list` importers: `internal/packaging/cmd`. `go test -count=1 ./internal/packaging/ ./internal/packaging/cmd/` exit 0 for both |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0; `GOOS=darwin go vet ./internal/packaging/` exit 0 |
| No guard, gate, cap or hold on agents | HOLDS | none added |
| Public repo: no personal paths, hosts, emails | HOLDS | covered by banned-string test; README uses `~/.grid`, `~/.claude/` only |
| Existing tests unedited, none deleted | HOLDS | only new files and the four edited files above |
| Small commits, no attribution | HOLDS | one commit, one sentence, no trailer |

## What changed

- `internal/packaging/plugin-readme.md` (new, 46 lines): public README (what Grid is, install, first run and what `grid setup` writes, Grid home and files outside it, plugin contents, MIT).
- `internal/packaging/dist.go`: `//go:embed plugin-readme.md` into `pluginReadme`; stage() writes it to `<plugin>/README.md` after LICENSE. Not in `packaging.Files`, so `make generate` does not touch the source README.
- `README.md`: line 5 reworded as the brief gave; the "not built yet" sentence dropped, contract link kept.
- `docs/release.md`: README.md in the shipped tree, in the "What stays out" paragraph, and on the step 3 `ls` line.
- `internal/packaging/em_public_readme_test.go` (new): three tests above.

## Notes

- docs/privacy.md does not exist on this base, so the README names `~/.claude/settings.json` and `~/.claude/agents/` directly.
- Install commands match docs/release.md:9-10.
- The README says setup "asks before it writes" (internal/settings/claude.go:432, `Write these? [y/N]`; `--yes` skips it).

## NOT VERIFIED

- `claude plugin marketplace add` / `install` against the real public repo (not run: no `claude`, no publish).
- How the README renders on the public repo's page.
