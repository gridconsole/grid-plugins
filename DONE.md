# L43 completion report

Built task-scoped take, adoption, release and status under the existing lease command. Added `TaskStore.AdoptTask`, `TaskStore.ReleaseTask` and `session.ProcessStart`; reused the existing Feed writer, lease counter, tenure-end log, locking and contract validation. Added handovers, granted epoch output, same-holder recovery after failed handover, and CLI serialization through the first post. Merge behavior and its command handler are unchanged apart from dispatching the new scope.

Implementation and command examples: `docs/lanes/L43.md`.

## Validation

These commands passed:

```sh
make test
make lint
GOOS=windows go build ./...
GOOS=darwin go build ./...
go test -race ./internal/lease ./internal/session ./internal/cli
go test -v ./internal/cli -run '^TestTaskLeaseReachabilityWithFeedTaskFixture$'
```

`make test` used the default temporary directory and reported no new matching temporary entries. Run the full and race suites separately: an initial simultaneous run collided in the existing development-port test; the separate full run passed. macOS and Windows were cross-built, not runtime-tested.

The named reachability test passes with two explicit prerequisites: a stable task ULID fixture and a CLI Feed Job post. It verifies that taking the first task lease makes Orchestrator dispatch reachable and that the printed epoch authorizes J-1 assignment. The exact requested fresh-task, plan-only sequence is still blocked by existing integration gaps described below; it is not claimed as passing.

## Spec decisions

1. No default pid. Take requires `--pid` plus `--pid-start`, or `--session`. A short-lived parent shell is not implicitly made holder. This follows the lane default and the process-identity requirement in reference `docs/feed-contract.md:100,212`.
2. No local forced takeover of a live holder. Live, unknown and remote holders are refused. A person's 15-minute takeover requires server-observed offline time and remains outside this CLI (`docs/feed-contract.md:116,214,218`).
3. Both lease reading paths remain: `lease.ReadJob` and `TaskStore.Read`. Unifying them is L34 review should-fix 6 and was explicitly excluded from this lane (`internal/lease/job.go:28-32`, `internal/lease/task.go:84`).
4. Remote status prints `remote`; remote holders are never probed or adopted. This retains the boundary requested for L34 review should-fix 5 (`docs/feed-contract.md:116,214`).
5. Handwritten start times are RFC3339 and stored with `contract.FormatTime`. `proc:` and raw `lstart` strings are never persisted in a task lease (`contract/schemas/lease.json:94-98`). Registered legacy sessions are converted only after verifying their process identity.
6. Adoption omits `old_fence`: the unchanged handover schema permits `from` and `adopted` but forbids additional body fields (`contract/schemas/handover.json:30-63`). The old tenure fence is recorded in `lease/takeovers.jsonl`.
7. Take refuses unsupported process-identity hosts. Windows runtime release/status support requires the existing lock package to support Windows; cross-compilation alone does not provide that support.

## Intended differences from the old plugin

The document references below name the read-only reference specifications; they are not new local copies.

| Difference | Old behavior and governing reference |
| --- | --- |
| A locked lease file grants authority and a monotonic fence; handover announces the grant. | Old `feed/lib.mjs:238-285,335,447-451` derives authority and epoch from ordered handovers. Reference `docs/feed-contract.md:94-124` requires lease-file authority, a fence counter and tenure-end snapshots. |
| Task ownership names an explicit long-lived process or registered session, with pid and actual process start time. | Old `feed/lock.mjs:24-38,49` uses process liveness, host and freshness of the loop lock. Reference `docs/feed-contract.md:100,202-218` requires pid/start identity and treats failed checks as unknown. |
| Only proven same-host death permits automatic adoption, and its first post says `adopted: session-gone`. | Old `feed/lock.mjs:43-66` reclaims locks deemed stale. Reference `docs/feed-contract.md:116-124,214,218` reserves remote/live takeover for server authority and forbids using age alone. |
| Release checks holder, generation and fence and records the tenure's end before clearing the holder. | Old `feed/lock.mjs:72-73` removes the loop lock when its pid and started stamp match. Reference `docs/feed-contract.md:100-110,118-124` requires a released null holder and a takeover-log snapshot. |
| Shell users receive a granted fence and reusable `gen.fence` epoch; held takes exit 1 and identify holder/since. | Reference `docs/feed-contract.md:305-310` establishes lease output wording. This lane applies it to task scope and adds the explicit epoch needed by the existing Orchestrator CLI. |

## What remains incomplete and why

- The exact `task new -> plan write -> lease take task -> job next` acceptance sequence cannot yet produce a ready Job. `task new` writes a display ID without the stable Feed ULID (`internal/task/create.go:76-110`), while the shared resolver requires that ULID (`internal/cli/feed.go:190`, `internal/feed/identity.go:54`). Task creation was outside the requested files; no extra task/frontmatter writer was introduced.
- Independently, `plan write` does not publish its Jobs to Feed (`internal/cli/plan.go:100-118`), and Orchestrator readiness reads Feed Jobs (`internal/orchestrator/ready.go:29-58`). The test publishes its Job with the existing `grid feed post` command. The protected Orchestrator and Job implementations were not changed to bridge this gap.
- Existing-lease status and release cannot run on Windows because `internal/fslock/fslock_windows.go:7-12` returns `ErrUnsupported`. That package is explicitly protected. The Windows build passes and task take emits the requested Unix-host refusal, but runtime status/release support remains a separate prerequisite.
- The four requested spec gaps, unsupported `old_fence` field, server takeover and dual lease readers are documented above. No schemas, hooks, Orchestrator, Job, contract, resolver, project or locking implementation were edited.
