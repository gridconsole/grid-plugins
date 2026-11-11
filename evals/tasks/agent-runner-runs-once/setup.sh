set -eu
mkdir -p work/notes-app
cat > work/notes-app/check.sh <<'T'
echo run >> runs.log
echo "3 passed, 1 failed"
echo "FAIL tag filter is case-sensitive"
exit 1
T
