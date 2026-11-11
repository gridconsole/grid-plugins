# Older Job records

Bytes of Job records as a Feed held them before commit e11cf80a (11 Nov), which
removed `review` and `max_rounds` from the required list of
`contract/schemas/job-record.json` and dropped `review`, `level_raised` and
`max_rounds` from the Job of `cmd-feed-state.json`. Every Job body here still
carries `"review": "L1"` and `"max_rounds": 2`.

Do not recapture or edit these files: they are copied as git had them, and
`TestGjOlderRecordFixturesKeepTheOldShape` fails if the old keys go away.

- `job-basic.json` is `git show e11cf80a^:contract/fixtures/job/valid/basic.json`,
  a whole Feed record of type `job`.
- `job-record-planned-fields.json` is
  `git show e11cf80a^:contract/fixtures/job-record/valid/planned-fields.json`,
  a Job body; the test appends it as the body of a second record of type `job`.
