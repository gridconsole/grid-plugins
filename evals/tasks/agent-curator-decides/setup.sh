# A registered project with one task and two waiting proposals, both written by grid brain propose --task.
# The eval workspace lies under the temp folder, which Grid treats as no project, so it becomes a git
# repository first and is then registered.
set -eu
: "${GRID_HOME:?}"
mkdir -p .claude/agents
git init -q .
grid project add . --name notes-app > /dev/null
grid task new "Add CSV export of notes" --project notes-app --json > /dev/null
grid brain propose --task T-01 --project notes-app --name csv-semicolons --topic decisions \
	--description "Read before changing the CSV export of notes." \
	--summary "The CSV export separates columns with semicolons." \
	--source T-01 \
	--body "The CSV export separates columns with semicolons because the target spreadsheet reads only that form. Changing the separator breaks the spreadsheet import." > /dev/null
grid brain propose --task T-01 --project notes-app --name ran-go-test --topic conventions \
	--description "Read when looking for what happened in the session." \
	--summary "The session ran go test and it printed ok." \
	--source T-01 \
	--body "Today I ran go test in the export package and it printed ok." > /dev/null
