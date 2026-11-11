# L-brain-note-rules-tests

Behaviour tests for the note and link rules a curated brain note must meet.
One new file, `internal/brain/note_rules_test.go`. `note.go` is unchanged.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Validate: one field changed per case, each refusal by its own message; valid note passes; 61 lines without contents fail, with `## Contents` list pass; 301 fail | HOLDS | `TestRulesNoteValidateRefusals` (note_rules_test.go:35) asserts the exact message for names, level, status, type, description (empty, 201, two lines), summary, source, metadata (reason, updated, superseded_by, origin id empty/blank/two lines), origin kind, link, NUL, 61 lines, 301 lines. `TestRulesNoteValidateAccepts` (:90) covers the valid note, 60 lines without contents, 61 and 300 with contents, 200-character description (also multibyte). `go test -count=1 -run TestRules ./internal/brain/` exit 0 |
| 2. summary and hasContents | HOLDS | `TestRulesSummary` (:148): empty, blank, 300 ok, 301, 3 lines ok, 4 lines, `\r`, NUL. `TestRulesHasContents` (:120): `- ` and `* ` item pass, indented item, other heading first fails (`##` and `###`), no item, no heading, list before heading, `### Contents`, `-A`. Same command, exit 0 |
| 3. Link targets | HOLDS | `TestRulesLinkTargets` (:181): the four named targets pass; `brain:team/a`, `brain:home/a/b`, `url:x`, `brain:home/a#bad anchor` fail with the target message, plus 11 more malformed targets. Same command, exit 0 |
| 4. Link rel and why | HOLDS | `TestRulesLinkRelation` (:202): `cites` fails `unknown link relation "cites"`, every entry of `relations` passes. `TestRulesLinkWhy` (:223): "see also", "Related", "deploy-notes" and the target itself on `brain:home/deploy-notes`, 7 and 161 characters, newline each fail with the why message; 20-character why that says what the reader gains passes, 8 and 160 pass. Same command, exit 0 |
| 5. hasSecret | HOLDS | `TestRulesHasSecret` (:270): private key header (pattern 0), `https://u:p@host` (1), `api_key = abc` (2), `ghp_` + 20 characters (3) each match their own `secretPatterns` entry and are caught by `hasSecret`; "we keep a token budget per run" is not. Same command, exit 0 |
| 6. `go test -count=1 ./internal/brain/` 0 fail | HOLDS | `ok github.com/gridconsole/grid-plugins/internal/brain 2.357s`, exit 0 |
| New tests fail on base | NOT APPLICABLE as written | The lane adds tests for rules that already hold, so they pass on base too. To show they bite I broke `note.go` in two throwaway edits (reverted with `git checkout`; `git status` shows only the new test file). Edit 1: why length limits 8/160 loosened to 7/161 and the related/target-name rules removed: `--- FAIL: TestRulesLinkWhy/Related`, `/related_padded_to_length`, `/target_name`, `/target_name_any_case`, `/target_name_on_an_anchored_link`, `/7_characters`, `/161_characters`. Edit 2: `hasContents` heading stop removed and the `ghp_` minimum lowered to 19: `--- FAIL: TestRulesHasContents/another_heading_first`, `/another_heading_level_first`, `--- FAIL: TestRulesHasSecret` |
| No existing test edited or deleted | HOLDS | `git status`: only `internal/brain/note_rules_test.go` and this report are new |
| Bug found | none | No assertion left out |
| Tests of the changed package and its importers | HOLDS | `go list` importers of `internal/brain`: braincmd, cli, hook, job, job/jobcmd, job/jobtest, people, routine, task, workflow. `go test -count=1` on them all: every package `ok`, exit 0 (jobtest has no test files) |
| Lint | HOLDS | `gofmt -l internal/brain` empty; `go vet ./internal/brain/` exit 0 |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go vet ./internal/brain/` exit 0; `GOOS=darwin CGO_ENABLED=0 go vet ./internal/brain/` exit 0 |
| No guard, gate, cap or hold on agents | HOLDS | Test file only |
| No personal paths, hosts or emails | HOLDS | Test file uses only `example`-style values |

## What changed

`internal/brain/note_rules_test.go` (new): 8 test functions, 106 passing subtests. Helpers `rulesLines` and `rulesLink` and the `msg*` constants live in that file; the existing `sample()` from `note_test.go` is reused.

Two behaviours the tests pin down that the brief did not name: a `Why` is trimmed before the checks, so "  see also  " is refused; and `Origin.ID` blank is refused with the metadata message.

## NOT VERIFIED on a live screen

Nothing here touches a screen.

DONE
