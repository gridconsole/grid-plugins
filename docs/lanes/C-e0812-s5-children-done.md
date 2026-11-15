# C-e0812-s5-children-done: pin the children-done edge for an open target

End review 0812Z S5 (`internal/jobgraph/graph.go`, `childrenDone`): the edge is met when the target is open and its children are accepted.

## Decision

| Question | Answer |
|---|---|
| Require the target itself done? | No. Behaviour kept. |
| Why | The only caller is `Ready` (graph.go, `case kind == ChildrenDone`). `TestReadyChildrenAndGates` (graph_test.go:207) releases a children-done edge on a parked split parent J-14 once J-14.1 and J-14.2 are accepted, and `docs/job-contract.md` ("A Job that is still open, such as a parked parent of a split, needs only its children") documents it. Requiring the target accepted would turn that test red and deadlock every split parent that waits for its own children (the parent is parked until they finish). Changing an existing test is not allowed. |
| What keeps deliver off a submitted build | The Judge closes the build before it accepts the review, so the review child is never accepted while the build is open. Now written in the `childrenDone` comment and pinned by tests. |
| Behaviour change | None. The only edit in graph.go is a comment. The new tests pass on base and on the lane. |

## Proof table

| Row | Result |
|---|---|
| New test, open target with accepted children meets the edge (parked, running, submitted) | HOLDS: `go test -count=1 -v -run TestS5 ./internal/jobgraph/` exit 0, `TestS5OpenTargetWithAcceptedChildrenMeetsEdge` PASS (3 subtests). Passes on base too: behaviour unchanged, so it cannot fail on base. |
| New test, open build with an open review waits | HOLDS: `TestS5OpenTargetWaitsOnOpenReview` PASS. |
| New test, closed build with accepted review is ready | HOLDS: `TestS5ClosedBuildWithAcceptedReviewMeetsEdge` PASS. |
| Comment names the close-before-review order | HOLDS: graph.go above `childrenDone`. |
| `go test ./internal/jobgraph/...` and importers (`go list`: internal/cli, internal/jobgraph, internal/orchestrator) | HOLDS: `go test -count=1 ./internal/jobgraph/... ./internal/orchestrator/... ./internal/cli/...` exit 0, all ok. |
| `make lint` | HOLDS: exit 0 (`go vet ./...`). |
| Windows and macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Walk tests | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. |
| `git fetch origin && git merge origin/main` | HOLDS: already up to date. |
| Guest walk tables (app repo only) | not run, not asked: this is the plugin repo. |
| Whole-module test, cross-repo set | not run, not asked: the orchestrator runs them at landing. |
| Mismatch proof, RealPlugin tests | not run, not asked. |

## What changed

- `internal/jobgraph/s5_children_done_open_target_test.go`: three tests that pin the edge for an open target.
- `internal/jobgraph/graph.go`: a comment on `childrenDone` naming the close-before-review order it relies on.

## NOT VERIFIED on a live screen

Nothing here is UI. The Judge's close-before-review order lives in the app (judge.go) and is not read or tested in this repo.
