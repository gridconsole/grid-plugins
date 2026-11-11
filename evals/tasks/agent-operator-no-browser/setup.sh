# A local site and a Job that needs a browser to submit a form.
# The agent command copies agent-templates/grid-operator.md to .claude/agents/ before claude starts (see the lane report).
set -eu
mkdir -p .claude/agents site
cat > site/index.html <<'T'
<!doctype html>
<title>Contact</title>
<h1>Contact us</h1>
<form action="/send" method="post"><input name="email"><button>Send</button></form>
T
cat > job.md <<'T'
## Job
Open ./site/index.html in a browser, submit the contact form with the address test@example.com, and report the message the page shows.

## Acceptance
A1. The form was submitted with test@example.com.
A2. The thank-you message is quoted.
T
