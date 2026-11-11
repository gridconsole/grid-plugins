#!/bin/sh
# Builds the synthetic task folder for the replan-dead-job case in the run dir.
# No network, no secrets: every name in it is made up.
set -eu

dir="${1:-.}/task-replan"
mkdir -p "$dir/feed"

cat > "$dir/intent.md" <<'EOF'
# Intent

Sam asks for a CSV export of the orders list, documented in docs/export.md.
EOF

cat > "$dir/plan.md" <<'EOF'
# Plan

Status: accepted by alex. Revised after J-2 was cancelled.

## Steps
- [ ] 1 Add the CSV writer in internal/export/csv.go
  jobs: [J-2]
- [ ] 2 Add the export command to cmd/orders
  jobs: [J-3]
- [x] 3 Document the export format in docs/export.md (dropped: Sam no longer wants a document, the command help is enough)
  jobs: [J-4]

## Verify
go test ./internal/export/ ./cmd/orders/

## Where it stands
J-2 was cancelled. J-3 and J-4 wait on it. J-5 re-plans.
EOF

cat > "$dir/feed/J-2.md" <<'EOF'
# J-2: Add the CSV writer in internal/export/csv.go

role: builder
state: cancelled
exit_code: 137
after: []

Cause: the builder was cancelled by alex after the run hit the memory limit of its machine
while it read the whole orders table into memory. Nothing was merged. The writer has to
stream the rows instead of loading them.
EOF

cat > "$dir/feed/J-3.md" <<'EOF'
# J-3: Add the export command to cmd/orders

role: builder
state: waiting
after: [J-2]
plan step: 2 (open)

Calls the CSV writer from J-2.
EOF

cat > "$dir/feed/J-4.md" <<'EOF'
# J-4: Document the export format in docs/export.md

role: builder
state: waiting
after: [J-2]
plan step: 3 (dropped from the plan)

Lists the CSV columns the writer from J-2 produces.
EOF

cat > "$dir/feed/J-5.md" <<'EOF'
# J-5: Re-plan after J-2 was cancelled

role: orchestrator
skills: [plan-jobs]
state: ready
after: []

J-2 was cancelled and the Jobs after it may no longer run.
EOF
