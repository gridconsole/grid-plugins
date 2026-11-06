---
type: regex
pattern: '[€£¥] ?[0-9]|(^|[^A-Za-z0-9_''"`\\/])\$ ?[0-9]|[A-Za-z]{2}\$ ?[0-9]|[0-9] ?\$([^A-Za-z0-9{(]|$)|[0-9] ?(dollars?|usd|eur|euros?|gbp|pounds?|cents?)([^a-z]|$)|\b(usd|eur|gbp) ?[0-9]'
flags: i
match: not_contains
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-tag-filter/questions/alex.md
---
