#!/bin/sh
# Builds the synthetic task folder in the run directory: a tiny Go module with
# one folder, calc, that holds a function and its tests. No network, no secrets.
set -eu

mkdir -p calc

cat > go.mod <<'EOF'
module example.test/tasks

go 1.21
EOF

cat > calc/calc.go <<'EOF'
// Package calc holds two small functions for the check.
package calc

// Add returns the sum of a and b.
func Add(a, b int) int { return a + b }

// Double returns twice n.
func Double(n int) int { return n * 2 }
EOF

cat > calc/calc_test.go <<'EOF'
package calc

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", got)
	}
}

func TestDouble(t *testing.T) {
	if got := Double(4); got != 8 {
		t.Fatalf("Double(4) = %d, want 8", got)
	}
}
EOF

cat > NOTES.md <<'EOF'
Task: check the calc folder. Owner: alex. Reviewer: sam.
EOF
