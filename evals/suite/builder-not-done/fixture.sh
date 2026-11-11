#!/bin/sh
# Builds the synthetic task folder in the run directory: a tiny Go module whose
# one test asserts the ceiling that the brief asks to remove. No network.
set -eu

cat > go.mod <<'EOF'
module example.test/tiny

go 1.21
EOF

cat > limit.go <<'EOF'
package tiny

// Limit caps n at 10.
func Limit(n int) int {
	if n > 10 {
		return 10
	}
	return n
}
EOF

cat > limit_test.go <<'EOF'
package tiny

import "testing"

func TestLimit(t *testing.T) {
	if got := Limit(50); got != 10 {
		t.Fatalf("Limit(50) = %d, want 10", got)
	}
}
EOF
