# M-js-if-brain-partial-onto-main

Merged main 99c11b7a and origin/lane-C-if-brain-partial-code (187b21d1) into the lane. Merge commit with the carried lane: 2614f2fd.
Conflict: contract/README.md hunk B only (main has no jd-hn/jd-ic, so no hunk A). Resolved: all table rows kept
(including `index_rebuild_failed`), then the three-line sentence "The first six constants ... `index_rebuild_failed` in
`internal/braincmd/changes.go`". N = six: the error_codes.go const block holds CodeNoSuchProject, CodeNoSuchTask, CodeNoSuchNote,
CodeInvalidArgument, CodeInvalidPolicy, CodeWorkflowFileChanged (internal/cli/error_codes.go:18-32).

| Row | Result |
| --- | --- |
| 1 ancestors | HOLDS: both `git merge-base --is-ancestor` exit 0; the final `git merge origin/main` prints "Already up to date." |
| 2 no markers | HOLDS: `git grep` of the three marker patterns in contract/README.md prints nothing |
| 3 tests | HOLDS: `go test -count=1 ./internal/braincmd/ ./internal/brain/` ok; `go test -count=1 -v -run TestHfFs ./internal/cli/` ok, `--- PASS: TestHfFsCodeTable` |
| 4 lint | HOLDS: `make lint` exit 0 (go vet ./...) |
| 5 cross | HOLDS: `make cross` exit 0 |
| 6 only README resolved | HOLDS: only contract/README.md conflicted. `git diff-tree --cc --name-only HEAD` also names changes.go and supersede.go (auto-merged, both sides touched them), but `git diff-tree --cc HEAD -- internal/braincmd/changes.go` shows no hunk. The word "six" equals the count 6. |
| Importers' tests | HOLDS: go test ok for cli, hook, job/..., people, routine, task, workflow (importers of braincmd/brain, from go list) |
| No guards, no attribution lines, no test edits | HOLDS: no Go or test file edited in this lane |

Not verified on a live screen: nothing (docs only).
Put C-if-brain-partial-code on landing-excludes.txt once this lane is accepted.
