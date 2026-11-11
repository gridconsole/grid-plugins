set -eu
mkdir -p scripts ui ops
cat > scripts/build.sh <<'SCRIPT'
set -eu
out=dist
mkdir -p "$out"
cp ui/status.html "$out/"
SCRIPT
cat > scripts/lint.sh <<'SCRIPT'
set -eu
grep -c '<h1>' ui/status.html
SCRIPT
cat > scripts/pack.sh <<'SCRIPT'
set -eu
tar -cf pack.tar dist
SCRIPT
cat > ui/status.html <<'PAGE'
<h1>Status</h1>
<p>State: <span id="state"></span></p>
PAGE
cat > ops/status.json <<'OP'
{"state":"ok","last_run":"2026-11-10T09:00:00Z"}
OP
