#!/bin/sh
# Builds the synthetic task folder in the current directory: a notes file with
# one true and one false claim, and the tiny repo the claims are about.
set -eu

mkdir -p inputs repo

cat > repo/retry.go <<'EOF'
package retry

// maxAttempts is how many times Do calls fn before it gives up.
const maxAttempts = 3

// Do calls fn until it succeeds or maxAttempts calls have failed.
func Do(fn func() error) error {
	var err error
	for i := 0; i < maxAttempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
	}
	return err
}
EOF

cat > repo/upload.go <<'EOF'
package retry

// Upload sends one file. It does not queue or flush anything.
func Upload(name string, send func(string) error) error {
	return Do(func() error { return send(name) })
}
EOF

cat > inputs/notes.md <<'EOF'
# Notes from alex

- retry.go gives up after 3 attempts.
- The upload queue is flushed every 30 seconds by FlushQueue.
EOF
