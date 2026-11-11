#!/bin/sh
# Builds the synthetic task folder for the plan-step-anchor case in the run dir.
# No network, no secrets: every name in it is made up.
set -eu

dir="${1:-.}/task-export"
mkdir -p "$dir"

cat > "$dir/intent.md" <<'EOF'
# Intent

Sam asks for a CSV export of the orders list, documented in docs/export.md.
EOF

cat > "$dir/plan.md" <<'EOF'
# Plan

Status: accepted by alex.

## Steps
- [ ] 1 Add the CSV writer in internal/export/csv.go
  jobs: []
- [ ] 2 Add the export command to cmd/orders
  jobs: []
- [ ] 3 Document the export format in docs/export.md
  jobs: []

## Verify
go test ./internal/export/ ./cmd/orders/

## Where it stands
Accepted. No Jobs are posted yet.
EOF
