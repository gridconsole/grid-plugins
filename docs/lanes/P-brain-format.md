# Lane P-brain-format

The notes Grid writes now open and edit cleanly in Obsidian. A filed (accepted)
note has flat front matter, names its author in a flat `source:`, keeps its links
in a `## Links` section, and every rewrite Grid makes of it changes only the keys
that changed. Proved on commit `b02fa888` (the merge of `origin/main` into the
lane); the commit that adds this file sits on top of it.

## Base check

`internal/brain/patch.go` did not exist on the base (`ls` exit 2 before it was
created). The five brief lines are in the base at `Validate` note.go:122,
`Link.Validate` note.go:194 and `markSuperseded` store.go:583; the line numbers
had drifted from the brief, the code had not.

## Proof table

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | (V13) A rewrite of a note with unknown keys, comments and CRLF changes only the edited key (byte diff) | HOLDS | `TestPbFmRenameKeepsWhatAPersonWrote` and `TestPbFmSupersedeChangesOnlyItsKey` (`internal/brain/pb_format_test.go`) compare the whole file against the original with one line changed; `go test -count=1 ./internal/brain/` exit 0. Both fail on base (see below). |
| 2 | (V13) F5 links (`[[x\|y]]`, `[[x#heading]]`) survive a Grid rename | HOLDS | Same rename test: `[[a\|the first]]`, `[[a#why]]`, `[[decisions/a#why\|alias]]`, `![[a]]` become the `b` forms, every other byte of the carrier note equal. Exit 0. |
| 3 | (V13) F2 lists (tags, aliases, cssclasses) stay lists | HOLDS | `pbFmLists` asserts `[]any` for the three keys after the rename (renamed note and carrier) and after supersede. Exit 0. |
| 4 | (V13, 4.5) A note Grid writes has a flat `source:` naming its task and Job, no `origin:` key, no nested map | HOLDS | `TestPbFmFiledNoteIsFlat`: no `origin` key, `source` names `T-01/J-01` (also when the proposal's source did not), every front-matter value is a scalar, links are a `## Links` section that reads back. Also `TestFormatOriginWriter/session` (assertion changed, see below). Exit 0. |
| 5 | (V13, 4.5) A person note with no front matter (F1) is never given one by a Grid read, rename or check (bytes unchanged) | HOLDS | `TestPbFmRenameLeavesANoteWithoutFrontMatter`: `Show` leaves it byte-equal; a rename of the note it links to rewrites the link and nothing else, no front matter added. Fails on base (the link is not rewritten). Exit 0. |
| S1 | `go test -count=1 ./internal/brain/ ./internal/braincmd/ ./contract/formats/ ./internal/cli/` | HOLDS | exit 0 on `b02fa888`, all four `ok`. |
| S2 | `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| S3 | `make cross` | HOLDS | exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| S4 | Tests of every package that imports a changed package | HOLDS | `go list` of all imports was run; direct importers of `internal/brain` are braincmd, cli, hook, job, job/jobcmd, job/jobtest, people, projectenv, routine, task, workflow; `go test -count=1 ./...` before the merge: every package `ok` except `internal/job` (TestBriefGolden) and `internal/job/jobcmd` (TestSupersededNoteIsNeverRecalled), both fixed (below) and rerun `ok`. After the merge, brain, braincmd, formats, cli, job/lint rerun `ok`. |
| S5 | Walk tests `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` | HOLDS | after the merge: `walked 41 commands, 41 table entries`, both PASS, exit 0. |
| S6 | `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0 after the merge. |
| S7 | `git fetch origin && git merge origin/main`, walk tests rerun | HOLDS | merged clean (`f4a471f3`), S5 and S6 rerun after it. |
| S8 | There is no app brief | HOLDS | nothing to run; RealPlugin rigs belong to the landing. |

### New tests fail on base, pass on the lane

Run on a worktree of the base commit `63d58a47` with `pb_format_test.go` added:
`go test -count=1 ./internal/brain/ -run TestPbFm`, exit 1. The failing lines:

- `TestPbFmRenameKeepsWhatAPersonWrote`: `decisions/a is not a valid entry: front matter: yaml: construct errors: line 2: field tags not found in type brain.Note; ... field aliases ... cssclasses ... rating`
- `TestPbFmSupersedeChangesOnlyItsKey`: `accepted note "brain:project/old" not found`
- `TestPbFmRenameLeavesANoteWithoutFrontMatter`: `rename changed more than the link in a note without front matter: "Just my own text.\r\n\r\nSee [[a|the first]].\r\n", want "...[[b|the first]]..."`
- `TestPbFmFiledNoteIsFlat`: `a: front matter has an origin key`, `origin is map[string]interface {}, Properties needs a flat value`, `links are not a Links section`.

On the lane the same four pass.

### Existing tests changed

Only where they pin `origin:` in front matter, all in a listed file:

- `contract/formats/formats_test.go` `TestFormatOriginWriter/session`: the assertion `accepted.Origin == n.Origin` and the schema check of the accepted origin now assert the opposite and the flat source: no origin in the saved file, `source: job:T-01/J-01` kept.
- `contract/formats/formats_test.go` `TestFormatOriginLegacyRead`: `n.Validate() == nil` fails and `Show("brain:home/legacy")` errors were the pin that a note with no origin is not knowledge. Under 4.5 a note with no origin is the filed form, so both now assert it validates and reads. The import quarantine those lines sat next to is still asserted in the same test (an external proposal is stamped `import` and `accept` exits 1) and in `TestFormatOriginWriter/import`.
- No test was deleted.

Goldens forced by the code (the note's bytes on disk changed, so the hash of them did):
`contract/fixtures/fmt-job-render/valid/full.json` and `elided.json` (recaptured with `-update-format-fixtures`), `testdata/golden/job-inputs.golden` (edited by hand to the hash the test printed; `UPDATE_GOLDEN=1` was denied). `contract/fixtures/fmt-job-render/valid/written-by-main.json` still carries the old hash; it is only schema-checked, not byte-compared.

## Rules

| Rule | Result | Proof |
|------|--------|-------|
| Edit only the Touches files | HOLDS | `git diff --stat origin/main...lane-P-brain-format`: markdown.go, config.go, note.go, store.go, ke_rename.go, new patch.go, `formats_test.go`, the three goldens above, and the new test file. `ke_write.go` needed no change. |
| New tests in new files, helpers `pbFm`-prefixed, fail on base | HOLDS | `internal/brain/pb_format_test.go`, see above. |
| No existing test edited to pass it except `origin:` pins | HOLDS | see "Existing tests changed". |
| Small one-sentence commits, no attribution lines | HOLDS | `git log origin/main..lane-P-brain-format`; no trailer on any commit. |
| No personal paths, hosts or emails | HOLDS | none in the diff. |
| No guard, gate, cap, hold or blocking hook on agents; remove nothing | HOLDS | the code only changes how bytes are written; nothing blocks a writer. |
| No push, no claude, no curl, no background server | HOLDS | none run. |
| Owner question, first form | HOLDS | no Grid header on a person note, an agent note's `source` names the first Grid writer, no `source:` added on adoption, no write into the owner's note on save. |
| Not this lane: origin read back from change records and listing | HOLDS | untouched; P-brain-adopt. |
| A check the brief forbids or does not ask | HOLDS | app brief: not run, not asked. Nothing the brief forbids was run. |

## What changed

- `internal/brain/patch.go` (new): the line-level front-matter patch. A key's span is its line and the indented lines under it; a trailing blank or column-0 comment line belongs to no key. A changed key is replaced in place (compared by value, not spelling), a new key goes right after the key that precedes it in the new bytes, a Grid-owned key that is gone is dropped, any other key stays, and the file's line ending is kept. `EncodeOver` is that patch for a filed note, `patchConfig` for a config entry. Also the `## Links` writer and reader and the body-link rewriter (`[[x]]`, `[[x|y]]`, `[[x#h]]`, `![[x]]`, `[t](x.md)`, `\|`, `<...>` and percent-encoded targets, code fences masked).
- `markdown.go`: `Encode` writes a filed note flat; `Parse` reads the `## Links` section into `Links` and skips unknown keys of a filed note.
- `note.go`: `Validate` split. `Validate()` is what a person's note must satisfy (origin absent or valid, no link summary needed); `ValidateGenerated()` is the full strict check.
- `store.go`: `decide` files `filedOf(n)` (the origin cleared, the source completed with the Job); the retry compare is by content. `markSuperseded` writes through `EncodeOver`.
- `ke_rename.go`: `RenameWith` writes the renamed note through `EncodeOver`, repoints `superseded_by`, and rewrites body links in every other note and person file under `knowledge/`; a note it cannot rewrite is kept and listed.
- `config.go`: `editConfig` puts the editor's bytes over the existing entry with `patchConfig`.

## Open items

- Proposals and accepted proposals that carry an origin keep the nested `origin:` and front-matter `links:` (the import quarantine, the log and the author tests read them). The origin leaves the front matter only for a filed note, until P-brain-adopt reads it from the change records.
- A link is written `[[brain:<level>/<id>]]`, which Obsidian cannot resolve; `TestKeRnStore` pins that substring and is not in this lane's list.
- Callers outside this lane's Touches still re-encode a whole note and so lose a person's unknown keys: `supersede_verb.go:110`, `task_decide.go:231/261/266`, `routine/store.go:334/357`, `projectenv/projectenv.go:198`, `job/source.go:201`, `braincmd/ke_write.go:116`.
- A rename rewrites a person file's links without a version or an audit line (a person file has no versions).
- `editConfig` is authoritative: a key absent from the bytes an editor returns is dropped.
- A legacy front-matter `links:` is moved into `## Links` on the first rewrite.
- A filed note with no origin reads as knowledge: the origin stamp was never a defence against a forged file, but a hand-placed note without one used to be refused by `Validate` and is not now. Owner decision if that matters.
- A new filed note writes `superseded_by: ''` and `reason: ''` as the base did; a rewrite never adds either to a note that lacks it.

## NOT VERIFIED on a live screen

Nothing here was opened in Obsidian. The claims are that the front matter has only scalar and list values, that `tags`, `aliases` and `cssclasses` stay lists, and that a person's comment and line endings survive; each is a byte assertion in a test, none was seen in Properties, in reading view or with a wikilink clicked. Unchecked: how Obsidian shows `source:` and an empty `superseded_by:`, whether `[[brain:project/x]]` is shown as an unresolved link, and whether the `## Links` list reads as intended.
