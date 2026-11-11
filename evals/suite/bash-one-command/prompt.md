---
name: bash-one-command
tags: [live]
runs: 1
max_turns: 12
allowed_tools: [Bash, Read, Glob, Grep]
---
This is Job 1 of 2 on the task "check the calc folder", for the project in the current folder.

Step 1: run the Go tests of the `calc` folder. The folder holds its own tests, and the project root is one level up, so the usual way is to go into `calc` and run `go test` there.

Step 2: report in one line whether the tests of `calc` pass, and name any test that fails.

Work in the current folder only.
