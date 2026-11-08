# L45 delivery status

The schema, fixtures, compatibility tags, and fixture-based Feed round trip are
implemented and committed. The requested direct `job.LoadPlan` -> `json.Marshal`
-> contract validation -> Feed round trip remains blocked by additional existing
serialization mismatches described below. This lane is not fully complete.

## Built

- Added the four optional properties after `id` in `job-record.json`, keeping
  two-space formatting, the required list, and the close-reason rule unchanged.
  All keywords are supported by the existing house validator.
- Added `planned-fields.json` Job-record and Feed fixtures, plus invalid
  `visible: huge`, empty `review_reason`, and string `irreversible` fixtures.
  Existing fixture tests discover them without changes to `internal/contract`.
- Added tests for the three exact error paths and reasons, kind syntax and
  length boundaries, review-reason length boundaries, both visibility values,
  and both boolean values.
- Added `omitempty` to `Status` and `FollowUp`, with tests showing empty fields
  are omitted and nonempty legacy defaults survive JSON decoding.
- Added a CLI test that posts the valid planned-fields fixture, reads the whole
  body back unchanged, and confirms the Job and counted verdict in Feed state.
  The fold intentionally projects selected fields rather than retaining full
  bodies (`internal/contract/fold.go:125`). The four fields survive storage and
  `feed read`; they are not exposed in folded state.
- Documented the optional fields and unchanged version in
  `docs/job-contract.md:24` and recorded lane status in `docs/lanes/L45.md`.

## Validation

Run from the checkout root:

```sh
make test
make lint
GOOS=windows go build ./...
GOOS=darwin go build ./...
go test ./internal/contract ./internal/job ./internal/feed -run 'Schema|Fixture|RoundTrip|Job'
go test ./internal/cli -run TestFeedJobPlannedFieldsRoundTrip -v
```

All commands passed on this Linux environment. `make test` used the default
TMPDIR and reported no temporary-file leaks. Its first run failed in the
unchanged `internal/env` test `TestBadConfigAndFailedStart` because reserved dev
ports had a listener; a second full run passed. Native macOS and Windows runtime
tests were not available; both cross-platform builds passed.

## Compatibility audit

Searched `internal/` and `cmd/` for `.Status`, `.FollowUp`, and `follow_up`.
The Job-specific consumers are `Job.Defaults`, brief metadata rendering,
frontmatter decoding, and brief lint. Other status readers use separate types,
including `contract.Job`, and do not read these compatibility tags.

`Job.Defaults` supplies nonempty `briefed` and `none` values
(`internal/job/job.go:155`). Brief metadata still writes them
(`internal/job/source.go:408`), and brief lint still requires nonempty values
(`internal/job/lint/lint.go:66`). Lint's cancelled/rejected close-reason check
continues to read nonempty status (`internal/job/lint/lint.go:179`). No caller
was found that requires empty compatibility strings to be serialized. The
existing Job, plan, Feed, Orchestrator, lint, and golden tests passed unchanged.
No golden file was regenerated; the brief golden already contains the nonempty
legacy defaults, not empty strings.

## Spec decisions and intended differences

1. `kind` uses `^[a-z][a-z-]*$`, with 1 through 40 characters, rather than an
   enum. New lowercase hyphenated kinds need no schema revision. This carries
   forward the step kind from `docs/plan.md:38` and `docs/job-contract.md:16`.
2. `review_reason` accepts 1 through 500 characters. It is intended as a sentence;
   the schema enforces type and length, not punctuation or sentence grammar
   (`docs/job-contract.md:24`).
3. Empty `Status` and `FollowUp` values are omitted from Job JSON. Nonempty legacy
   brief values remain unchanged. The old plugin's brief format requires those
   keys (read-only reference `old-plugin-map.md:81`); the Feed Job contract is
   distinct from that brief shape (reference `docs/job-contract.md:7`, planned
   and run fields). No `follow_up` schema property was added.
4. Contract version stays `1`: these are optional additions, so existing valid
   Job records remain valid (`docs/job-contract.md:24`).

The intended change from the previous Go contract is acceptance of the inherited
kind and optional floor inputs already emitted after L38, described in
`docs/job-contract.md:16`, `docs/job-contract.md:19`, and `docs/lanes/L38.md:57`.
This lane introduces no further old-plugin plan-format or execution changes.

## Remaining blockers

A temporary validation probe loaded `testdata/job/task`, selector `J-1`, and
marshalled that Job after the two authorized tag changes. It failed as follows;
removing each failing value exposed the next one:

| Value in the loaded Job | Validation failure |
| --- | --- |
| `follow_up: "none"` | `$: unknown property follow_up` |
| `posts_back: null` | `$.posts_back: expected array` |
| `status: "briefed"` | `$.status: value outside enum` |
| `verify.min_tests: 0` with `kind: file-exists` | `$.verify: oneOf matched 0 branches` |

Clearing the compatibility fields, initializing `posts_back` to an empty array,
and removing `min_tests` from that verifier made the probe validate. These
transformations have not been added to production or disguised as a direct
round trip. `LoadPlan` calls `Defaults` (`internal/job/source.go:387`), and
`Verify.MinTests` lacks `omitempty` (`internal/job/job.go:70`). Fixing those
behaviors exceeds the explicit two-struct-tags-only restriction and would need
review of brief compatibility and existing goldens. The scope question is
pending: allow explicit normalization before posting, or expand the production
fix to support the originally requested direct round trip.

Also, `ReviewReason` already has `omitempty`: setting its Go value to `""`
omits the optional property and is valid. To test the schema's empty-value
rejection, a test must supply explicit `"review_reason": ""` JSON, as the new
invalid fixture does, rather than expect an omitted optional field to fail.

The cited `reviews/L38.md` is absent in this checkout. The current source,
`docs/lanes/L38.md`, and the read-only reference specs supplied the evidence.
No forbidden packages, other schemas, existing fixtures, or version constants
were edited.
