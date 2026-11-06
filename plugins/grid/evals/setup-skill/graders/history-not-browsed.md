---
type: regex
pattern: '"type"\s*:\s*"tool_use"[^\n]*?(?:(?:~|/Users/[^/"\\\s]+|/home/[^/"\\\s]+)/\.(?:claude|codex)/(?:projects|sessions|archived_sessions)|"name"\s*:\s*"Bash"[^\n]*?"command"\s*:\s*"(?:[^"\\]|\\.)*?(?:\$\{?(?:HOME|CODEX_HOME|CLAUDE_CONFIG_DIR)\b|\.(?:claude|codex)\b|\bfind\s+(?:/(?:\s|"|Users\b|home\b)|~)))'
match: not_contains
target: trace
---
