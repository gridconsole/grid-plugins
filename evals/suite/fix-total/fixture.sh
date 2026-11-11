set -eu
cat > total.sh <<'SCRIPT'
set -eu
sum=1
for value do sum=$((sum + value)); done
printf '%s\n' "$sum"
SCRIPT
cat > verify.sh <<'SCRIPT'
set -eu
test "$(sh total.sh 2 3)" = 5
test "$(sh total.sh)" = 0
test "$(sh total.sh -2 2)" = 0
SCRIPT
