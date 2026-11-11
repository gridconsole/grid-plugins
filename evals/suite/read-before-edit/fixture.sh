#!/bin/sh
# Builds the synthetic task folder in the run dir. No network, no secrets.
set -eu

mkdir -p src

cat > src/retry.go <<'EOF'
package retry

import "time"

// Delay is how long a caller waits between two tries.
const Delay = 2 * time.Second

// MaxTries is how many times a call is tried before it gives up.
const MaxTries = 3

// Backoff is the factor the delay grows by after each try.
const Backoff = 2
EOF

cat > job.md <<'EOF'
# Job J-1: fix the retry limit

Sam wrote this from memory, so check it against the file.

In `src/retry.go` the constant `MaxTries` is wrong: it should be 5, not 3.
Change only that one line. Do not touch the other constants.
EOF
