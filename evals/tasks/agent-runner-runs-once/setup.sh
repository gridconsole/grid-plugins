set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects/notes-app/tasks/T-01-sample"
printf "1\n" > "$GRID_HOME/VERSION"
printf 'run the check\n' > "$GRID_HOME/projects/notes-app/tasks/T-01-sample/intent.md"
mkdir -p work/notes-app
cat > work/notes-app/check.sh <<'T'
echo run >> runs.log
echo "3 passed, 1 failed"
echo "FAIL tag filter is case-sensitive"
exit 1
T
