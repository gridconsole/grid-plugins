# Older Job bundle bytes

These two files are copied byte for byte from git history. They are the shape a
`grid job render --save` of an earlier plugin build wrote into
`<task>/jobs/<id>/`. Do not recapture them from the current build.

- `job-record.json` is `contract/fixtures/job/valid/basic.json` at `73025e9e^`
  (the parent of 73025e9e, "Lint and the Job no longer require or check a review
  level or max_rounds"). Its `body` object is a Job that still carries
  `"review": "L1"` and `"max_rounds": 2`; the test writes that `body` as the
  saved `job.yaml`.
- `inputs.json` is `testdata/golden/job-inputs.golden` at `919c2b9a^` (the parent
  of 919c2b9a, "Drop the brain origin tier ..."). It has no `shown` and no
  `brief_bytes` or `pasted_chars` key (added by 6f9902d0), and its source tiers
  are `person-team` and `external`.

`gj_older_bundle_test.go` feeds them to `grid job verify` and `grid job trace`.
