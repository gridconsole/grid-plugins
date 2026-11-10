# A one-page site and no browser.
set -eu
mkdir -p site
cat > site/index.html <<'X'
<!doctype html>
<html><head><meta charset="utf-8"><title>Notes</title></head>
<body>
<h1>Notes</h1>
<p>Your notes are listed below.</p>
<button id="wipe">Delete all</button>
</body></html>
X
