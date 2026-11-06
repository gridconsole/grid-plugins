#!/usr/bin/env bash
# A Grid workspace folder ./ws with a static contact page (file:// URL) and a checker brief of two Acceptance lines, the second needing a form submit.
set -e
d=ws/projects/notes-app/tasks/T-01-sample
mkdir -p "$d/subagents" "$d/artifacts" ws/projects/notes-app/site
printf 'check the contact page shows the form and that sending it works\n' > "$d/intent.md"
cat > "$d/task.md" <<'T'
---
id: T-01
title: Contact page
project: notes-app
owner: alex
source: idea
size: normal
workflow:
  - {step: build, kind: run}
  - {step: check, kind: run}
step: check
status: running
updated: 2026-03-04T10:00
summary: Check the contact page in a browser.
next: Run the check step.
---

- 4 Mar · check · brief written
T
printf -- '- 2026-03-04T09:00 · created · by alex\n' > "$d/history.md"
cat > ws/projects/notes-app/site/index.html <<'T'
<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Contact us</title></head>
<body>
  <h1>Contact us</h1>
  <form id="contact" action="thanks.html" method="get">
    <label>Name <input name="name" required></label>
    <button type="submit">Send</button>
  </form>
</body>
</html>
T
cat > ws/projects/notes-app/site/thanks.html <<'T'
<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Thanks</title></head>
<body><p>Thanks, we got your message.</p></body>
</html>
T
printf 'tasks/\n' > ws/projects/notes-app/.gitignore
git -C ws/projects/notes-app init -q
git -C ws/projects/notes-app add -A
git -C ws/projects/notes-app -c user.name=alex -c user.email=alex@example.com commit -q -m "Add the contact page"
url="file://$PWD/ws/projects/notes-app/site/index.html"
cat > "$d/subagents/01-check-contact.md" <<T
---
id: 01-check-contact
role: checker
status: briefed
verify: self
follow_up: none
---

## Intent
"check the contact page shows the form and that sending it works"

## Job
1. Open $url in a browser and check each Acceptance line.

## Context
- The page: $url
- The task folder: ws/projects/notes-app/tasks/T-01-sample

## Out of scope
Every change to the page or the site.

## Open
Nothing.

## Acceptance
A1. The page has the heading "Contact us" and a form with a Name field and a Send button.
A2. Sending the form with a name shows a page that says "Thanks".

## Outcome

## Verdict

## Follow-up
T
