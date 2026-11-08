# Workflows

## Social post
applies when: project northwind, or label social
approvers: sam, robin
repeat: Mondays at 09:00
review rounds: 2

| # | step | kind | who | on reject, back to |
|---|---|---|---|---|
| 1 | Write the posts together | make-together | you and grid-designer |  |
| 2 | Check the copy | check | grid-runner | Write the posts together |
| 3 | Robin approves the posts | approve | robin | Write the posts together |
| 4 | Post on schedule | schedule | the scheduler |  |

## Monthly content calendar
applies when: label calendar, or the title has content calendar
approvers: sam, robin, relay page
lead time: a month ahead
batch: One task holds the whole month; the customer approves once, by the 25th
review rounds: 2

| # | step | kind | who | on reject, back to |
|---|---|---|---|---|
| 1 | Prepare the month together | make-together | you and grid-designer |  |
| 2 | Robin approves the month | approve | robin | Prepare the month together |
| 3 | Customer approves by the 25th | approve | relay page | Prepare the month together |
| 4 | Post each item on its day | schedule | the scheduler |  |

## Design together
applies when: the task changes a screen and starts with a sketch
approvers: sam
review rounds: 3

| # | step | kind | who | runs | on reject, back to |
|---|---|---|---|---|---|
| 1 | Sketch it together | make-together | you and grid-designer |  |  |
| 2 | Build it | run | grid-builder | make build |  |
| 3 | Review the screens | approve | sam |  | Build it |
| 4 | Merge into main | run | grid-builder |  |  |

## Web page
applies when: the task writes a page under a site folder
approvers: sam, alex
review rounds: 2

| # | step | kind | who | on reject, back to |
|---|---|---|---|---|
| 1 | Draft the page together | make-together | you and grid-designer |  |
| 2 | Check the links | check | grid-runner | Draft the page together |
| 3 | Approve the page | approve | sam | Draft the page together |

## Release
applies when: the task ends in a version tag
approvers: sam, alex
review rounds: 3

| # | step | kind | who | runs | sizes | on reject, back to |
|---|---|---|---|---|---|---|
| 1 | Reproduce the problem | run | grid-builder | make test | all |  |
| 2 | Fix it and test | run | grid-builder | /grid:worker | all |  |
| 3 | Preview the result | approve | sam |  | L | Fix it and test |
| 4 | Approve the version | approve | sam |  | all | Fix it and test |
| 5 | Release | schedule | the scheduler |  | all |  |

## Sign-off
applies when: the task ends in a file a person signs off
approvers: sam
review rounds: 2

| # | step | kind | who | runs | on reject, back to |
|---|---|---|---|---|---|
| 1 | Ask what is needed | ask | grid-planner asks, you answer |  |  |
| 2 | Write it | run | grid-builder | /grid:writer |  |
| 3 | Check it | check | grid-runner |  | Write it |
| 4 | Sign off | approve | sam |  | Write it |

## Accept, approve, ship
applies when: nothing else matches
approvers: sam, alex, robin
review rounds: 2
research: first

| # | step | kind | who | runs | sizes | on reject, back to |
|---|---|---|---|---|---|---|
| 1 | Accept the plan | ask | grid-planner asks, you answer |  | N L |  |
| 2 | Do the work | run | grid-builder | /grid:worker | all |  |
| 3 | Preview the result | approve | sam |  | L | Do the work |
| 4 | Approve the result | approve | sam |  | all | Do the work |
| 5 | Ship | schedule | the scheduler |  | all |  |
