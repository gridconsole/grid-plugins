# M-ho-feed-types-scope-onto-main

Merge lane: brings `lane-C-gs-feed-types-scope-check` (50f05e55) onto plugin main.

## What conflicted and how it was resolved

`git merge origin/lane-C-gs-feed-types-scope-check` conflicted in two test files; no production file conflicted.

- `internal/cli/gg_unknown_task_walk_test.go`: main's `ggTaskKnownOpen` holds five entries (`job lint`, `job render`, `job show`, `job verify`, `signal act`). The lane's side was the old longer map (plan, job and feed tick entries) minus `feed types`. Kept main's five entries, dropped `feed types`, took the lane's comment wording ("...stderr stays empty; signal act exits 0."). None of the entries main removed came back.
- `internal/cli/gg_unknown_project_walk_test.go`: kept main's `ggProjectKnownOpen`, dropped its `feed types` entry and the blank line above it. The brain entries main removed did not come back.
- `git fetch origin && git merge origin/main` afterwards: "Already up to date".

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 main and carried lane in branch | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0; `git merge-base --is-ancestor origin/lane-C-gs-feed-types-scope-check HEAD` exit 0 |
| 2 no `feed types`, main's five entries kept | HOLDS | `git grep -n '"feed types":' -- <both files>` prints nothing; `git diff origin/main` on the project walk shows only the removed `feed types` line and the blank line above it; on the task walk only the removed `feed types` line and the two comment lines |
| 3 three walk runs | HOLDS | `go test ./internal/cli/ -run 'TestGgUnknownTaskWalk\|TestGgUnknownProjectWalk\|TestGsTypes' -count=1 -v` exit 0: `--- PASS: TestGgUnknownProjectWalk`, `--- PASS: TestGgUnknownTaskWalk` ("known open: 5 of 81"), `--- PASS: TestGsTypes`, `ok ... internal/cli 0.492s`. Run as one combined command, not three. |
| 4 package, lint, cross | HOLDS | `go test ./internal/cli/ -count=1` exit 0 (91.7s); `make lint` (`go vet ./...`) exit 0; `make cross` darwin/arm64 vet and test compile ok, windows/amd64: see the final report line |
| Importers of changed package | HOLDS | `go list` importers of internal/cli: cmd/grid (no tests), lib. `go test ./cmd/grid/ ./lib/ -count=1` exit 0 |
| New tests fail on base | n/a | The lane adds no new test in this merge; it only resolves conflicts. TestGsTypes comes from the carried lane. |
| No guard, gate, cap or hold added | HOLDS | Only two test maps changed, each by one removed entry |
| No attribution lines, no personal paths | HOLDS | One merge commit with a one-sentence message |

## NOT VERIFIED on a live screen

Nothing in this lane touches a UI.
