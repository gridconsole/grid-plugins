# M-js-iy-signal-codes-onto-il

Conflict: merging origin/lane-C-il-guest-refusal-code into the iy lane conflicted in internal/cli/error_codes.go, two hunks.
Hunk 1 (constants): kept iy's side (CodeNoteState, CodeNoteExists, CodeNotesFull, then iy's comment and `CodeGuestRefused`); one constant left.
Hunk 2 (errorCode): kept both cases, il's `errors.Is(err, ilGuestRefused)` first, then iy's `errors.As(err, &refused)`.
`git merge origin/main`: "Already up to date." Main does not hold jd-ic yet, so steps 4 to 6 had nothing to resolve; README already says "The first ten constants" and the const block holds 10.
C-il-guest-refusal-code was already on /srv/gridwork/tmp/hand/landing-excludes.txt (line 1); no edit needed.

## Proof

| Row | Result |
|---|---|
| 1 ancestors | HOLDS: `git merge-base --is-ancestor` exit 0 for origin/main, origin/lane-C-il-guest-refusal-code, origin/lane-C-iy-signal-refusal-codes; `git merge origin/main` printed "Already up to date." |
| 2 one constant, no marker, count | HOLDS: `git grep -c 'CodeGuestRefused = ' -- internal/cli/error_codes.go` prints `internal/cli/error_codes.go:1`; marker grep prints nothing; contract/README.md:91 "The first ten constants", const block string constants counted 10 |
| 3 lane rows | HOLDS: `go test -count=1 -run 'TestIl\|TestIySig\|TestHfFs\|TestGuestProjectParity' ./internal/cli/` ok, exit 0; `go test -count=1 ./internal/signal/` ok |
| 4 package, lint | HOLDS: `go test -count=1 ./internal/cli/` ok (60s); `make lint` (go vet ./...) exit 0 |
| 5 cross | HOLDS: `make cross` exit 0 (all packages ok, windows/darwin vet and builds) |
| 6 merge files | PARTLY: `git diff-tree --cc --name-only HEAD` (07bf264d) lists error_codes.go, contract/README.md and guest_parity_test.go. The last two are the auto-merged files where both lanes changed; I hand-edited only error_codes.go. |

Repo rules: no guard/gate/cap added; no test edited or deleted; no attribution lines; no pushes. Changed-package importers: only internal/cli changed (non-test code is one file); `make cross` ran the whole module's tests, all ok.

NOT VERIFIED on a live screen: nothing UI-related.
