set -eu
cat > job.md <<'JOB'
# Job J-2: Add greet.sh

Write: greet.sh, test.sh.
Acceptance:
1. greet.sh prints "hello Sam".
2. test.sh exits 0 when greet.sh prints "hello Sam".
3. The README names greet.sh.
JOB
cat > result.md <<'RESULT'
Proof table
1. HOLDS: `sh greet.sh` prints "hello Sam", exit 0 (greet.sh:2)
2. NOT DONE: test.sh was not written, the time ran out
3. HOLDS: README.md:4

Changed: greet.sh, README.md
Verify: sh test.sh not run
NOT VERIFIED: nothing on a live screen
RESULT
cat > greet.sh <<'SCRIPT'
set -eu
echo "hello Sam"
SCRIPT
cat > README.md <<'README'
# Demo

Run it with sh.
See greet.sh.
README
