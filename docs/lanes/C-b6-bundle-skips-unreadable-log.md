# C-b6-bundle-skips-unreadable-log: proof table

Base: origin/main a6c5cb02 (the brief cites 414347a3; the cited lines are the same on a6c5cb02). The lane is up to date with origin/main (`git merge origin/main`: "Already up to date.", origin/main a6c5cb02). All proof runs below ran on the code commit 474b7df0; the later commits change only this doc.

| Row | Status | Proof |
|---|---|---|
| 1 Base check | HOLDS | At a6c5cb02, internal/doctor/bundle.go:138 `if err = add("skipped.txt", strings.Join(skipped, "\n")+"\n"); err != nil {` comes before the file loop at :142, and :143-145 `content, err := readBundleTail(file.path)` / `if err != nil {` / `return "", fmt.Errorf("collect %s: %w", file.name, err)`. The loop did not skip on base. |
| 2a Unreadable Grid home log is skipped and named | HOLDS | `go test -count=1 -v -run 'TestB6Bs' ./internal/doctor/` exit 0: `--- PASS: TestB6BsBundleSkipsAnUnreadableHomeLog`. Names are `doctor.txt,logs/ok.log,skipped.txt,versions.txt`; skipped.txt is one line naming `logs/locked.log` and `permission denied`. Also `--- PASS: TestB6BsBundleSkipsAnUnreadableLauncherLog` covers a mode-000 launcher log (`logs/machine/launcher/server.log`) in the same skipped.txt. |
| 2a fails on base | HOLDS | The new test file was run on base a6c5cb02's bundle.go, before either step was made (step 2 was absent too). The failing `collect …` return is the line that step 1 replaces. Exit 1: `b6_bundle_skip_log_test.go:43: an unreadable log must not stop the bundle: collect locked.log: open /tmp/TestB6BsBundleSkipsAnUnreadableHomeLog3183292430/001/logs/locked.log: permission denied` / `--- FAIL: TestB6BsBundleSkipsAnUnreadableHomeLog`. Same run: `--- FAIL: TestB6BsBundleSkipsAnUnreadableLauncherLog` (`collect machine/launcher/server.log: open …/launcher/alice/server.log: permission denied`) and `--- FAIL: TestB6BsSkippedLineIsRedacted` (`collect sk-b6bsabcdefghijklmnop.log: … permission denied`). No file is written: Bundle returns before archive.Close. |
| 2b Readable log as written, no marker | HOLDS | Same run, TestB6BsBundleSkipsAnUnreadableHomeLog: `logs/ok.log` equals `readable b6 body\n`; no entry holds `B6BS-LOCKED-LOG-MARKER`. |
| 2c Single-link refusal stays | HOLDS | Same run: `--- PASS: TestB6BsReadBundleTailStillRefusesAHardLink`, calls readBundleTail directly on a log hard-linked after it was written and gets exactly `log is not a single-link regular file`. This passes on base too by design (the refusal is unchanged; readBundleTail is untouched, bundle.go:290). |
| 2d Skipped line is redacted | HOLDS | Same run: `--- PASS: TestB6BsSkippedLineIsRedacted`. A mode-000 log named `sk-b6bsabcdefghijklmnop.log` under Home: skipped.txt holds `[REDACTED]` and `permission denied`, and no entry name or body holds the secret or the home path. skipped.txt goes through `add` (bundle.go:154). |
| 2 Skips on Windows and as root | HOLDS | b6_bundle_skip_log_test.go `b6BsSkipUnlessUnreadable` and the hard-link test skip on `runtime.GOOS == "windows" \|\| os.Geteuid() == 0`, as pc_bundle_skip_test.go:42. The run here is as a normal login: all four print `--- PASS`, none `--- SKIP`. |
| 3 Bundle pins | HOLDS | `go test -count=1 ./internal/doctor/ ./internal/cli/ -run 'Bundle'` exit 0 (`ok …/internal/doctor`, `ok …/internal/cli`). `-v` run of TestBundleContentsAndMode, TestBundleSkipsHardLinks (still 3 entries, no skipped.txt: the hard link is left out at listing, bundle.go:237 on the lane head, unchanged), TestBundleRefusals, TestBundleLeavesOutUnsafeNamesAndOrdersEqualTimesByName, TestBundleSkipsAnUnreadableLauncherFolderAndSaysSo, TestBundleSkipsALauncherThatHasNoPlainLoginName: all `--- PASS`. The pin files are unedited (`git diff --stat origin/main..HEAD` lists only bundle.go, the new test and this doc). |
| 4 make lint, make cross | HOLDS | `make lint` exit 0 (go vet ./...). `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile, every package ok). |
| 5 Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`). `go test -count=1 ./internal/job/lint/` exit 0. Run after the merge of origin/main. |
| Changed packages and importers | HOLDS | Importers of internal/doctor (`go list -f '{{.ImportPath}} {{join .Imports " "}}' ./... \| grep 'internal/doctor' \| grep -v '^github.com/gridconsole/grid-plugins/internal/doctor ' \| cut -d' ' -f1`; `.Imports` does not list test-only importers): `github.com/gridconsole/grid-plugins/internal/cli`. `go test -count=1 ./internal/doctor/ ./internal/cli/` exit 0 (`ok …/internal/doctor 5.116s`, `ok …/internal/cli 160.992s`). |
| New tests fail on base, pass on lane | HOLDS | Rows 2a-2d. TestB6BsReadBundleTailStillRefusesAHardLink is a pin that holds on both, by the brief's row 2c. |
| No existing test edited or deleted | HOLDS | `git diff --stat origin/main..HEAD`: only internal/doctor/bundle.go and the new test file changed, plus this doc. |
| No guard, gate, cap, hold or hook | HOLDS | The change removes a hard stop from Bundle and adds none. |
| Touches only | HOLDS | bundle.go (`Bundle` only), b6_bundle_skip_log_test.go, docs/lanes/C-b6-bundle-skips-unreadable-log.md. |
| No attribution lines | HOLDS | `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` prints 0. |
| Whole-module test, cross-repo set | not run, not asked | Run by the orchestrator at landing. |
| RealPlugin tests | not run, not asked | The brief names none. |

## What changed
- internal/doctor/bundle.go `Bundle`: in the file loop, a `readBundleTail` error no longer fails the bundle. The log is left out and `skipped %s: %v` (archive name `logs/…`, then the error text) is added to `skipped`. skipped.txt is written after the loop through `add`, only when `skipped` is not empty, so launcher-folder skips and log skips share one redacted file. readBundleTail, the listing step and the hard-link check are unchanged. A refused log's content never reaches the zip.
- A bundle with something skipped now has skipped.txt as its last entry, not before the logs. No test pins the raw order with a skip present (pc_bundle_skip_test.go:59 sorts).
- New internal/doctor/b6_bundle_skip_log_test.go with four TestB6Bs tests.

## NOT VERIFIED on a live screen
Nothing: the lane has no UI.

## Open items
- A log that becomes hard-linked between listing and reading is now left out and named in skipped.txt (name and `log is not a single-link regular file`), where before it failed the whole bundle. Its content still never reaches the zip.
- A skipped line carries the error text, as the brief asks, and that text holds the log's absolute path. `add` shortens only a path under `Home` to `~`. A path outside it, such as a machine-folder launcher log or a GRID_HOME elsewhere, lands in skipped.txt in full. The launcher-folder line at bundle.go:95 already behaves this way.

DONE
