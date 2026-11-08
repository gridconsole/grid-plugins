# Deterministic sample agent; this is not a model-quality evaluation.
set -eu
cat > /dev/null
case "$GRID_EVAL_TASK" in
  fix-total)
    sed 's/sum=1/sum=0/' total.sh > total.new
    mv total.new total.sh
    sh verify.sh
    ;;
  release-notes)
    { printf '# Release notes\n\n'; sed 's/^/- /' changes.txt; } > RELEASE.md
    ;;
  clean-csv)
    { head -n 1 inputs/items.csv; tail -n +2 inputs/items.csv | sort -u; } > clean.csv
    printf 'Unique items: 2\n' > summary.txt
    ;;
  *) printf 'fake agent: unsupported task\n' >&2; exit 2 ;;
esac
printf 'Completed %s\n' "$GRID_EVAL_TASK"
