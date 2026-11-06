---
type: regex
pattern: '(?=[\s\S]*<time id="clock">09:41</time>)(?=[\s\S]*Last run finished at <time>10:20</time>)(?=[\s\S]*min-width: 640px)(?=[\s\S]*overflow: hidden)'
target:
  source: file
  path: site/index.html
---
