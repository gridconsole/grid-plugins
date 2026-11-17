# P-land-kind: a land Job kind on the feed (R-land-job-type L1)

Repo: plugin. Clone <lanes>/P-land-kind on lane-P-land-kind from main. Model: sonnet, high. About 50 calls.
Review: Opus
Skills: grid:run-job.
Why: owner, landing batches 17 Nov (<briefs>/landing-batches-2026-11-17.md ask 5): landing as a first-class Grid Job type, one per accepted change. Design: <hand>/R-land-job-type.md sections 3 and 6, lane L1-land-kind. Keep internal/land (deploy-after-ship) as it is.
Acceptance rows:
1. kind: land with role runner passes `grid job lint`; it needs target, branch and sha and refuses each missing one with its own code: tests.
2. A land Job's record round-trips through grid feed post / grid feed state: test.
3. Existing lint tests stay green. go test ./internal/job/... ./internal/contract/... and make lint: 0 fail (tails). Walk tests before handoff: go test ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList' and go test ./internal/job/lint/ (result lines). Merge main into the lane before the proof.
Write docs/lanes/P-land-kind.md with the proof table.
Never: push, publish the plugin, touch <gmf> or <evalprog>, write <root-home>, pkill -f, add a guard, cap, delay or limit on Jobs. No attribution lines; committed files carry no private paths, hosts or emails.
