# Workflows

## Rate change
applies when: the title has rates
only for project: helios
approvers: sam
review rounds: 2

| # | step | kind | who | runs | on reject, back to |
|---|---|---|---|---|---|
| 1 | Write the change | run | grid-builder | /grid:worker |  |
| 2 | Check it against the old rates | check | grid-runner |  | Write the change |
| 3 | Approve the new rates | approve | sam |  | Write the change |
| 4 | Roll out on the first of the month | schedule | the scheduler |  |  |
