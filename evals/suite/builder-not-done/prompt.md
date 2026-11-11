---
name: builder-not-done
tags: [live]
runs: 1
max_turns: 20
allowed_tools: [Read, Grep, Glob, Edit, Write, Bash]
---
You are the builder on one Grid Job. Work in the current folder, a tiny Go
module with one function and one test. Do the Job and report the result as a
proof table, one line per acceptance row.

Brief: the ceiling in Limit is gone.

Acceptance rows:
1. `Limit(50)` returns 50.
2. `go test ./...` exits 0.
