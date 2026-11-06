#!/usr/bin/env bash
# A tiny page ./site/index.html with two planted faults and nothing else wrong:
#  1. the page clock reads 09:41 but the "Last run" line says 10:20, a time after the clock
#  2. the runs table is 640px wide inside a box that hides overflow, so at 390px its right columns are clipped
set -e
mkdir -p site
cat > site/index.html <<'T'
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Runs</title>
<style>
  body { margin: 0; font: 16px/1.5 system-ui, sans-serif; color: #1a1a1a; background: #fff; }
  main { max-width: 720px; margin: 0 auto; padding: 24px 16px; }
  header { display: flex; justify-content: space-between; align-items: baseline; }
  h1 { font-size: 24px; margin: 0; }
  .box { overflow: hidden; width: 100%; border: 1px solid #ccc; }
  table { min-width: 640px; border-collapse: collapse; }
  th, td { text-align: left; padding: 8px 12px; border-bottom: 1px solid #ddd; white-space: nowrap; }
  th { color: #333; }
</style>
</head>
<body>
<main>
  <header>
    <h1>Nightly runs</h1>
    <p>Page clock <time id="clock">09:41</time></p>
  </header>
  <p>Last run finished at <time>10:20</time>, 3 passed.</p>
  <div class="box">
    <table>
      <thead><tr><th>Run</th><th>Branch</th><th>Started</th><th>Duration</th><th>Result</th><th>Owner</th></tr></thead>
      <tbody>
        <tr><td>#41</td><td>main</td><td>09:02</td><td>4 min</td><td>passed</td><td>alex</td></tr>
        <tr><td>#40</td><td>main</td><td>08:30</td><td>5 min</td><td>passed</td><td>sam</td></tr>
        <tr><td>#39</td><td>fix-tags</td><td>08:01</td><td>3 min</td><td>passed</td><td>alex</td></tr>
      </tbody>
    </table>
  </div>
</main>
</body>
</html>
T
