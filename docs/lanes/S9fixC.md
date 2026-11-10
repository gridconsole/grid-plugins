# S9fixC: the Load failure exit is tested and named

## What changed
- `grid approve` (task_write.go) prints a Feed-snapshot or `PersonReady` error as `grid approve: <err>` (it printed the bare error) and still exits 3.
- orchestrator.go: `loadFeedSnapshot` (= `orchestrator.Load`) is a package variable that approve calls, so a test can make the load fail. `planPersonJob` reads the same snapshot first and returns no Job when that read fails, so a real corrupt feed never reaches approve's own Load; the seam is the only deterministic way in.
- New test: cli `TestApproveLoadFailureExits3WithPrefix` (approve_job_test.go): Load fails, checks exit 3, stderr exactly `grid approve: feed unreadable`, no approval.json.

## New test failing without the prefix fix
`approve_job_test.go:195: stderr "feed unreadable\n"` (prefix line reverted, seam kept; on base the test does not compile as `loadFeedSnapshot` does not exist). Passes on the lane.

## Checks
- `go test ./internal/... ./cmd/... -count=1` exit 0 (importers of cli: cmd/grid, no tests).
- `gofmt -l internal cmd` empty, exit 0.
- `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0.
- make lint not run (per brief).

## NOT VERIFIED
- A real feed corruption reaching approve's Load (only the seam is tested).
