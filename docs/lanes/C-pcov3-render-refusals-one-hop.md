# C-pcov3-render-refusals-one-hop

Tests only: one new file, `internal/job/render_refusals_test.go`. No non-test file changed.

## Proof table

| # | Row / rule | Result |
|---|---|---|
| 1 | Coverage before and after | HOLDS. Before (untouched clone, linux): `total: (statements) 90.1%`. After: `total: (statements) 92.8%`. Command: `go test -count=1 -coverprofile=/tmp/C-pcov3-render-refusals-one-hop-job.out ./internal/job/ && go tool cover -func=... \| tail -1`, exit 0. |
| 2 | Input refusals | HOLDS. `TestRenderRefusesNilResolverAndBadBudget` (render_refusals_test.go): nil resolver `a source resolver is required` (render.go:66); budgets -1 and 2001 `brain budget must be between 1 and 2000 tokens` (:69); 1 and 2000 accepted; budget 0 renders as 2000 (a ~1,100-token note is kept with 0 and with 2000, byte-equal briefs, and cut with 1000; asserted through Inputs.BrainTokens and NotRendered); a budget of 1 with one cut note -> `brain budget too small for the elision banner` (:247). `TestRenderRefusesBadBrainSummary`: Summary "", blank, 301 chars and 301 multi-byte runes -> `brain:project/s: summary must have 1 to 300 characters` (:139); 300 chars and 300 runes accepted; PointerOnly with no summary accepted; a recalled entry with an empty summary refused (:189-191). |
| 3 | Resolver errors | HOLDS. `TestRenderPassesResolverErrorsThrough`: one subtest each for intent owner (:119), item (:123), goal (:127), relation (:303), continues (:311), read (:319), a Decisions brain id (:136) and a recalled entry (:190); each checked with `errors.Is` on a sentinel. |
| 4 | One hop | HOLDS. `TestRenderWalksExactlyOneBrainHop`: A as Decisions, as Keep and as a Job `brain:` link. A depends-on B, A decided-by D, B decided-by C. A, B and D render; C is not resolved or rendered. A `uses` link to U and a `depends-on` link to `repo:file.go` pull nothing. The relation names are at render.go:181 (`depends-on`, `decided-by`, target prefix `brain:`). A recalled entry R that links B with depends-on starts no hop (B is absent from the brief and the inputs). Mutation check: adding `\|\| l.Rel == "uses"` at :181 made the test fail (`brain:project/u was pulled in`, 3 subtests); reverted, render.go is unchanged. |
| 5 | Job fields | HOLDS. `TestRenderJobFieldLines`: set -> `Checkpoint: step-2 (abc123)` (:345), `(traces: T1; state: done)` (:358), `Verify: go test ./...` + `Minimum tests: 3` (:363). Unset -> no `Checkpoint:`, no `; state:`, and `Verify: tests internal/x#J-2` (:365 form) is printed. |
| 6 | Replay | HOLDS. `TestReplayResolveRefusesAndAliases`: `brain:project/zzz: not in render inputs` and `note.md#L2-L4: not in render inputs` (:549); the exact key resolves; the key saved as `task:note.md:2-3` resolves from `note.md#L2-L3` (:542-546). |
| 7a | `go test -count=1 ./internal/job/` | HOLDS, exit 0, `ok ... internal/job`. |
| 7b | `GOOS=windows go vet ./internal/job/`, `GOOS=darwin go vet ./internal/job/` | HOLDS, both exit 0. |
| 7c | `GOOS=windows go test -c -o /dev/null ./internal/job/` | HOLDS, exit 0 (compiled; a Windows run is needed to execute it). |
| 7d | `make lint` | HOLDS, exit 0 (`go vet ./...`). |
| R1 | Packages that import internal/job | HOLDS. `go list` gave cli, hook, job (+ frontmatter, jobcmd, jobtest, lint), jobgraph, lease, orchestrator, ports. `go test -count=1` on all of them: exit 0, all ok. |
| R2 | New tests fail on base | Not applicable as stated, because this lane changes no code and the tests describe today's code. The mutation check in row 4 shows the one-hop test fails when the code differs. |
| R3 | No non-test file, no existing test edited, no guard/gate/cap | HOLDS. `git status` showed only the new test file; nothing is blocked on an agent. |
| R4 | Metadata error (render.go:264) | NOT DONE. `Metadata` marshals a Job of plain strings, ints, slices and a map, and `Dependency.MarshalJSON` cannot fail, so no input reaches that error. Left uncovered. |

## What changed

New file `internal/job/render_refusals_test.go` (own fake resolver `rrResolver`, no existing helper edited). Where the code differs from the brief's wording, the code won: a budget of 1000 with an oversized note is not an error (the note is cut and the banner fits); the :247 error needs a cut note and a budget below the banner.

## NOT VERIFIED on a live screen

Nothing here is UI. The Windows build is compiled only; run `GOOS=windows go test ./internal/job/` on a Windows host to execute it.
