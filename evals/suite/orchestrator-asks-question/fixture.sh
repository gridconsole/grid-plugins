#!/bin/sh
# Builds a synthetic task folder in the run directory. No network, no secrets.
set -eu

mkdir -p task/T-01/jobs src

cat > task/T-01/task.md <<'EOF'
# Task T-01: export the monthly report

Owner: sam. Asked by: sam.

Intent: add a `grid-report export` command to `src/report.txt`'s tool that writes
the monthly report to a file.

The report needs one format, and only sam knows which one the accountant opens:
CSV or XLSX. Nobody on the team can tell from the repo. Every other choice is
free: the file name is `report.<format>` and the command takes no flags.

After the format is chosen, the work is one build Job and one review Job.
EOF

cat > task/T-01/jobs/J-1.md <<'EOF'
# Job J-1: plan the export task

Role: orchestrator. Task: T-01.

Read `task/T-01/task.md` and decide the Jobs the task needs. Post them with their
`after` lists and your result, then stop.

Acceptance rows:

- A1: every Job the task needs is posted with its `after` list.
- A2: your result is posted with `grid job result`.
EOF

printf 'monthly report tool\n' > src/report.txt
