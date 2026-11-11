# C-ln-plugin-env-entry

Plugin package `internal/projectenv` that saves, edits, removes and lists a project's environments as brain config entries, plus `brain.LooksSecret`.

Base: main `f6383e8c77b17a82718880a54f8330969a4b58c8`. Base check: `grep -rn 'knowledge/environments\|"environments"' --include=*.go .` finds only topics.go:29, check.go:44, internal/env/config.go:51 (the dev-server key, a different thing) and tests; nothing on the base writes `knowledge/environments`. Read before building: config.go:1-160, store.go:25-90, note.go:196-215, config_test.go (Store on a temp home).

## Proof table

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | Bytes are the contract: Render of the stage example equals the literal byte for byte; Parse gives the Entry back; full-field round trip equal | HOLDS | `go test -count=1 -v -run TestLnEnv ./internal/projectenv/` exit 0: `--- PASS: TestLnEnvExampleBytes`, `TestLnEnvRoundTripEveryField`, `TestLnEnvStringsAreStoredTrimmed`, plus every other TestLnEnv test and subtest; no `--- FAIL`. The literal is `lnEnvExample` at projectenv_test.go:17; the round trip has who x2, secrets x2 (one missing), steps x2 (one without text), approval false, and a step text with `"`, `<`, `>`, `&` (JSON written with HTML escaping off). |
| 2 | Stored as a config entry: Save writes projects/alpha/knowledge/environments/stage.md; ReadConfig same bytes; second Save changes only the test line; Index counts environments 1 entry, 0 invalid; log.md gains a line per change | HOLDS | `TestLnEnvSaveStoresAConfigEntry` PASS: file bytes == example, `ReadConfig("project","environments","stage")` bytes == file, line-by-line diff after Test "make smoke2" is only the `test:` line, `Index()` row environments Entries 1 Invalid 0, log.md audit lines naming environments and stage: 1 then 2. |
| 3 | Remove deletes, ReadConfig answers fs.ErrNotExist; List gives a then b; List with no knowledge folder is empty, no error | HOLDS | `TestLnEnvRemove` (file gone, ReadConfig `fs.ErrNotExist`), `TestLnEnvListIsInIdOrder` (saved b, a-b, a; got a, a-b, b: sorted by id, not by file name), `TestLnEnvListWithNoKnowledgeFolder` (no project folder, and project folder with no knowledge folder: empty non-nil list, nil error). EditConfig's no-log-line on remove is untouched. |
| 4 | No secret value written; refusals write nothing | HOLDS | `TestLnEnvRefusesSecretValues` (14 cases incl. the four named: secret name `API_TOKEN=abc123`, Test `curl -H "Authorization: Bearer abc123"`, Where `https://u:pw@host`, Runs `password: hunter2`): each errors, error text lacks `abc123` / `u:pw@` / `hunter2`, and the Grid home is still empty. `TestLnEnvRefusalLeavesTheSavedEntry`: a refused edit leaves the saved entry byte-equal and adds no audit line. `TestLnEnvRefusesBadFields` (23 cases incl. `Bad_Id`, Kind `moon`, empty Name, Name with newline): refused, Grid home empty. |
| 5 | Nothing else moved; brain and projectenv tests exit 0 | HOLDS | `git diff main --stat` (before this report was committed): `internal/brain/ln_secret.go 13 +`, `internal/projectenv/projectenv.go 353 +`, `internal/projectenv/projectenv_test.go 443 +`, 3 files. This report, `docs/lanes/C-ln-plugin-env-entry.md`, is the one extra file; the builder runbook requires it. `go test -count=1 ./internal/brain/ ./internal/projectenv/` exit 0. |
| 6 | Plugin checks | HOLDS | `make lint` exit 0 (go vet, gofmt -l empty). `make cross` exit 0 (darwin/arm64 and windows/amd64: vet and test compile of every package). `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok internal/cli 0.035s`). `go test -count=1 ./internal/job/lint/` exit 0. Run again on the final code head after the last code commit. |
| R1 | New tests fail on base | HOLDS | With ln_secret.go and projectenv.go stashed (the base): `go test -count=1 -run TestLnEnv ./internal/projectenv/` fails to build: `projectenv_test.go:34:19: undefined: Entry`, `:81:14: undefined: Render`, `:88:12: undefined: Parse`, `:101:15: undefined: Step`. |
| R2 | Packages that import the changed package pass | HOLDS | `go list` importers of internal/brain: braincmd, cli, hook, job, job/jobcmd, job/jobtest, people, projectenv, routine, task, workflow. `go test -count=1` on brain, projectenv and those: all `ok` (jobtest has no test files), exit 0. |
| R3 | Walk tests | HOLDS | Plugin set: `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` ok, `./internal/job/lint/` ok (rows 6). App-only walk tests do not apply to the plugin repo. |
| R4 | No guard, gate, cap or hold on agents; no existing test edited or deleted; no production code beyond Touches | HOLDS | Only three new files in the code diff; no existing file touched. |
| R5 | No attribution lines, no personal paths, hosts or emails | HOLDS | Commit messages are one sentence each; no `Co-Authored-By`. No path, host or email in the new files. |
| R6 | RealPlugin tests | n/a | No row names a *RealPlugin* test; nothing skipped. |

## What changed

- `internal/brain/ln_secret.go`: `LooksSecret(s string) bool` runs the note checker's `secretPatterns` over s. note.go is unchanged.
- `internal/projectenv/projectenv.go`: `Entry`, `Secret`, `Step`; `Render`, `Parse`, `Save`, `Remove`, `List`. Validation runs before `EditConfig`, so a refusal does not even create the project folder. Error text names the field, never the value. `Render` is `brain.EncodeConfig(map{"name": id}, body)` with the body in the fixed order. `Save` and `Remove` call `EditConfig("project","environments",id,...)`; `List` is `ConfigNames`, `ReadConfig`, `Parse`, sorted by id.
- `internal/projectenv/projectenv_test.go`: the TestLnEnv tests above.

Choices the brief did not fix (the CLI lane and the reviewer should know):
- `Remove` of an id with no entry returns an error wrapping `fs.ErrNotExist` (EditConfig itself would answer nil). Nothing is written or removed.
- `LooksSecret` is also run on the id, the secret names and the step workflow and step name, not only on scalars, who and step text; a secret name or id shaped like a token (`AKIA...`, `sk-...`) is refused.
- A step needs a non-empty `wf` and `step`; an empty who entry is refused; Kind may be empty (left out) but otherwise is local, remote or cloud.
- `Parse` checks the front matter name equals the id, the `### <id>` line, rejects unknown or repeated keys and malformed lines, and then runs the same validation as `Save`, so a hand-edited file holding a secret value or a bad kind is an error (naming the entry, never the value). `List` stops at the first unreadable entry with an error naming it.

## Open item for the review: ids that look like YAML scalars

The brief fixes the front matter as `name: <id>`. `brain.EncodeConfig` (which the brief names) quotes an id that YAML would read as a number, boolean or null: `123`, `true`, `null`, `no`, `1e3`, `0x1f`, `007`, `2026-11-11` are written `name: '123'` (see `TestLnEnvIdsThatLookLikeYAMLScalars -v`). It is valid YAML, reads back as the same string through `DecodeConfig`, and the example `stage` is unaffected. I did not change it, because the brief names `EncodeConfig` and an id rule beyond the regex is not in it. If the app reader matches the front matter line byte for byte, either the id regex needs to exclude these ids or the reader should parse the front matter as YAML.

## NOT VERIFIED on a live screen

Nothing here has a screen. NOT VERIFIED: that the app reader (C-ln-env-note-read) parses these bytes; the example literal is the pin, held in the test, but the reader is another repo.
