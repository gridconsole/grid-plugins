# A page with four visible defects: low contrast, a sentence in monospace, 10px text, a 20px button.
set -eu
mkdir -p site
cat > site/index.html <<'X'
<!doctype html>
<html><head><meta charset="utf-8"><title>Notes</title>
<style>
body { margin: 0; padding: 24px; background: #ffffff; font-family: Georgia, serif; }
h1 { font-size: 28px; color: #222; }
.lede { color: #bbbbbb; font-size: 16px; }
.sentence { font-family: "Courier New", monospace; font-size: 16px; color: #222; }
.meta { font-size: 10px; color: #444; }
button { width: 20px; height: 20px; font-size: 10px; }
</style></head>
<body>
<h1>Notes</h1>
<p class="lede">Your notes are listed below, newest first.</p>
<p class="sentence">Pick a note to read it, or press the plus sign to write a new one.</p>
<p class="meta">12 notes, last saved 4 minutes ago</p>
<button aria-label="New note">+</button>
</body></html>
X
