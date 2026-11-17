# Lane P-loop-schemas

Draft. The proof table is written at the end.

## New tests fail on base (63d58a47)

`go test -count=1 -run 'TestPlSc' ./internal/contract/ ./internal/cli/` on the base tree with only the two new test files added, exit 1:

```
--- FAIL: TestPlScNewTypes (0.01s)
    --- FAIL: TestPlScNewTypes/approval (0.00s)
        pl_sc_loop_schemas_test.go:87: approval is not a registered post type: {Name: Version:0 Class: Roles:[]} (ok false)
    (the same line for blocked, cancel, exit, integrated, landed, started, task-move, waited, waiting, wake)
--- FAIL: TestPlScTaskMoveNeedsTo (0.00s)
    pl_sc_loop_schemas_test.go:117: task-move with from and to: unknown schema "task-move"
--- FAIL: TestPlScAssignFields (0.00s)
    pl_sc_loop_schemas_test.go:162: assign with the launcher fields: $.body: oneOf matched 0 branches: $.body: oneOf matched 0 branches: $.body.assign: unknown property account
--- FAIL: TestPlScSubmitFields (0.00s)
    pl_sc_loop_schemas_test.go:182: submit with repo, base_sha and commits: $.body: oneOf matched 0 branches: $.body: unknown property base_sha
--- FAIL: TestPlScJobRecordFields (0.00s)
    pl_sc_loop_schemas_test.go:207: job-record with home and landed edges: $.after[0]: anyOf matched 0 branches: $.after[0].kind: value outside enum
--- FAIL: TestPlScEnvelopeByLegacy (0.00s)
    pl_sc_loop_schemas_test.go:221: post with by and legacy: unknown schema "landed"
FAIL	github.com/gridconsole/grid-plugins/internal/contract
--- FAIL: TestPlScFirstJobHome (0.02s)
    pl_sc_first_job_home_test.go:53: J-1 home = <nil>, want the writer's machine "01ARZ3NDEKTSV4RRFFQ69G5FAW"
FAIL	github.com/gridconsole/grid-plugins/internal/cli
```
